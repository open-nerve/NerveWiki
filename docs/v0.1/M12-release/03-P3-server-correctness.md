# M12/P3 服务端的正确性与历史：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M12/P3 服务端的正确性与历史（三个 Step：S1 小的正确性；S2 第 302 条（先写 spec）；S3 导入导出的任务） |
| 日期 | 2026-11-16（初稿；同日第一次、第二次、第三次修订；同日照 S2、S1 的实施与审查修订）；2026-11-17 照 S3 的实施与审查修订 |
| 状态 | 进行中（设计已审查，S1 已合并 `f3e5a6de`、S2 已合并 `46b6940f`、S3 已合并 `（待填：S3 的合并号）`；设计的[审查记录](reviews/P3-design-review.md)（三份设计审查、三次修订与三轮修订核对）；S1 的[审查记录](reviews/P3-S1-small-correctness-review.md)、S2 的[审查记录](reviews/P3-S2-nested-trash-roots-review.md)、S3 的[审查记录](reviews/P3-S3-transfer-jobs-review.md)；实施与审查之后与代码不同之处照实改在各节（3.21、3.22、3.23 列出），结果在 10.1、10.2、10.3） |
| 上级文档 | [M12 总设计](00-M12-design.md) 4.5（H1、24.1 写成已知限制）、4.6、第 3 节第 4、5 条、5.1、第 7 节 P3 一行与"次序与并行""分份""移交的落实""总体设计的修订"第 8、10 条、9.2、9.6、第 10 节、第 11 节决定 304、313–316、328、335、336、357、359、附录 A 的 P3/S1 一行；[M12 设计审查](reviews/M12-design-review.md)的 A-M1–M3、A-M8、A-N1–N5、A-I1–I3、A-M4、C-I2；[移交](handoffs/)：M8-history 第 5、6 条，M7-transfer 第 5 条的三项，M5-polish 第 10、25、30 条，M5-performance 第 6 条，M4-performance 第 22 条之后的补记 `:490`；[v0.1 总体设计](../v0.1-design.md) 5.8 第 6 步、13.1 第 2、28、30 条、13.4 第 1、4、8、11 条；[M11/P1](../M11-upgrade/01-P1-freeze-start-rollback.md)第 3 节（服务端的写法）、[M12/P1](01-P1-foundation.md)（骨架） |
| 代码位置的写法 | 同 00 号文档：模块写作 `page/domain/revert.go:309`（`server/internal/modules/` 之下）；`platform/…`、`bootstrap/…`、`shared/…`、`archtest/…` 在 `server/internal/` 之下；迁移的测试写作 `server/migrations/schema_test.go`；前端写作 `W/pages/notebook/transfer-report.tsx:100`（`W/` 是 `web/apps/web/src/`）；e2e 写作 `e2e/stories/transfer/tr4-malicious.spec.ts:105`；契约写作 `api/modules/transfer.yaml:1079`；其余写仓库根起的路径。基点是 `main` 的 `86565dd6`（P1 完成），它与 `194ca350` 只差文档（00 号文档、P1 设计与一份审查记录），代码相同：代码的行号都在 `194ca350` 上读过，在 `86565dd6` 上不变；00 号文档与总体设计的行号是 `86565dd6` 的 |
| 工作笔记 | 本文引的行号都在 `194ca350` 上读过（附录 A）。第一次修订照三份审查改，审查者的原型与探针在 `m12/p3-review-{a,b,c}/`，修订时另跑的在 `m12/p3-rev1/`；第二次修订照修订核对改，另跑的（3.14.3 的四个测试的用时、00 号文档脚本的试跑）在 `m12/p3-rev2/`；第三次修订照修订核对第 2 轮改 3.7 的理由与 (b) 格，依据是审查 B 的探针 `m12/p3-check2-b/`。设计不跑重的东西；在 scratchpad 的临时 worktree（`194ca350`，不带竞态）里跑了几组小实验，另在一个用完即删的 `postgres:18.7-trixie` 容器里核对了快照冻结的时刻，原样留在 `m12/p3-design/`：第 302 条的四种形状与两个反例在 domain 上的结果（改前、审查 A 的原型 `split-tops.diff`、"`topOf` 只看起点"的变异）；后台撤销的 `needed` 在嵌套的回收站根下真的出错（模型与核对的替身都红，3.10.6）；GBK 解码器对 0x5C 尾字节、Shift-JIS、GB18030 四字节、0x80 与 0xFF 的行为，GBK 名字恰好是合法 UTF-8 的比例（3.12）；REPEATABLE READ 的快照在第一条读等锁之前冻结（3.8.2）。没有跑整个程序的测试，没有量时间 |

---

## 0. 分三个 Step

照 00 号文档第 7 节，P3 分三个 Step，各自一个分支（`m12-p3-s<m>`）、一次审查与修复核对、一次合并。每个提交单独为绿（02-workflow 第 3.1 节）：build、vet、整个模块的 lint、`make test-go-short`；改到契约的提交带上 `make gen` 的生成物、网页的类型、假服务端的字段与两种语言的文字（13.2 第 4、11 条），改到网页的还过 tsc、knip、vitest。

- **S1 小的正确性**（`m12-p3-s1`）：
  - 标题键对切罗基文幂等，三处折叠一起改，`searchFold` 的映射删掉（25.1，3.2）；
  - 标题 id 带上自动链接与原始 HTML 的文字（10.2，3.3）；
  - 标签列表不列 `/`（10.1，3.4）；
  - 只由看不见的字符组成的名字算空（30.2，3.5）；
  - MCP `log` 的 `path`：`"/"` 是整本，`""` 答字段错误（30.4、30.5，3.6）；
  - MCP 调用超时的文字（3.7）；
  - 日志与变更集的详情各在一个快照里读（H5，3.8）；
  - 事件流按凭证至多 4 条、按账户至多 64 条（3.9）；
  - **单独为绿**：服务端与契约的说明为主；网页只有生成的类型，不加文字、不加测试（网页对 429 照 `Retry-After` 等，`W/services/api.ts:61`、`:70` 对任何状态码都取它）。新的整个程序的测试两个，点名进第七份（3.19）。
- **S2 第 302 条**（`m12-p3-s2`，**先写 spec**，本文 3.10 就是它）：
  - 撤销计划的 `readReverted`/`topOf`：被这个变更集删了、现在本身是回收站根的节点是它自己的顶；五处构造计划的地方都传 `Nodes`；
  - 后台撤销的 `needed` 也看段内后面的放回（实验里它真的出错，3.10.6）；
  - 两个性质测试扩开；domain 的一张确定的表；app 的一格确定的后台撤销；
  - 整个程序的 nested 一格翻过来（两本笔记本），改名，第七、第八份的模式一并改；
  - **单独为绿**：只改 page 的 domain 与 app、组合根的一个测试与 `Makefile` 的两行。nested 一格在改 domain 的同一个提交里翻过来（6.2）。
- **S3 导入导出的任务**（`m12-p3-s3`）：
  - 导入条目名：先读 Info-ZIP 的 Unicode 路径（0x7075），再按 GBK 回退，整包的判断，报告另列（T3，3.12）；
  - 按人的队列上限，新的码 `transfer.too_many_jobs`（T2，3.13）；
  - 启动时收拾上一个进程取到而没开始的任务（T4，3.14）；
  - **单独为绿**：服务端、契约与网页（新码与报告的两种新原因的文字）一起；`README.md:192`、`:193` 两句随之。新的整个程序的测试两个，点名进第七份；T4 并进已有的 `TestServeFailsTheJobsLeftRunning`，已有的几处种子随 T4 带上 River 的行（3.14.3）。

**次序**（00 号文档第 7 节"次序与并行"）：S1 与 S3 在 P1 合并之后就能开，彼此不重叠（S1 改 page、shared、linking、events、platform；S3 只改 transfer）。S2 也改 page：在 S1 合并之后开，或合并时 `git merge main`。同时实施的不多于两个。

**与 P2 的重叠**（审查 C 的 M5；P2 的初稿 `p2-draft.md` 第 0.1 节与 4.1–4.6）：S1 不改网页的源文件（只有生成的类型）。S3 改的网页文件 P2 几乎每个 Step 都改：

| S3 改的 | P2 也改它的 Step（P2 初稿的项） |
|---|---|
| `W/app/problem-messages.ts` | S2（提到） |
| `W/i18n/messages/en.ts`、`zh-CN.ts` | 每个 Step |
| `W/pages/notebook/transfer-report.tsx` | S1（19.3）、S3（第 14 行） |
| `W/pages/notebook/trash-row.tsx` | S3（26.12）、S6 |
| `W/pages/notebook/revert-dialog.tsx`（只在它的测试 `revert-background.test.tsx` 里加格） | S1（23.14、24.3/49）、S2（23.11）、S6（23.10、24.2） |
| 导出、导入的对话框（只加测试的格与 `transfer.importCut` 的文字） | S2（19.1）、S5（20.1） |
| `useForm` 的横幅（S3 的新格断言横幅的文字） | S1（横幅的焦点） |

所以：S3 的网页部分在 P2/S1 合并之后写（横幅的新格照 P2/S1 的写法）；S3 与 P2 的 S2–S6 谁后合并，谁先 `git merge main`、解决冲突、重跑 `make test-web`，再推送、等分支的 CI。S3 的网页改动小（一个码、两种原因、一个按钮的条件、一句文字），冲突由它照 P2 的新写法改。00 号文档第 7 节"P3 的三个 Step 与 P2 除翻译文件外不重叠"随之改（2.1 第 14 条）。

## 1. 基线

基点是 `main` 的 `86565dd6`（P1 完成：两个 Step 与三条旁支都已合并）。它在 `194ca350`（来源守卫的旁支合并）之后只改了文档，下面的代码行号都是在 `194ca350` 上读的。

**第 302 条（M8 的撤销计划）**：

- `readReverted`（`page/domain/revert.go:275-304`）按条目算出每个被删节点的顶：`topOf`（`:309-321`）沿 `Before` 的父节点往上走，父节点也被这个变更集删了就继续。它不看现在的状态，所以一个现在本身是回收站根的节点，只要它原来的父节点也被同一个变更集删了，就被并进父节点那一顶。
- 并进去之后先失败的是 `rootHolds`（`:337-353`）：顶那一项回收站的 `TrashPages` 只数同一时刻删下的，这个变更集在它下面删的却多。`RevertGroups`（`:196-206`）于是不读这一组，`leftDeleted`（`:494-533`）给顶答 `changed`（`groupIsDeleted` 之前就失败），给里面那个回收站根答 `changed`（`:506` 的 `n.TrashRoot`）。
- `readReverted` 有五处构造计划用：`RevertGroups`（`:197`，传入 `nil` 版本）、`scheduled`（`:389`）、`CheckRevertPart`（`page/domain/revert_part.go:26`）、`CheckRevertSteps`（`:51`）、`RevertActionFacts`（`page/domain/revert_facts.go:25`）；另外 `RevertWrites`（`revert.go:213`）只取它的 `writes`。其中读顶的（`c.top`、`c.tops`、`c.members`）只有 `left`/`leftDeleted`/`groupIsDeleted`/`rootHolds`/`movesBack`（`:683`）与调度的 `newRevertSim`、`schedule`（`page/domain/revert_schedule.go:86`、`:328`）：`CheckRevertSteps` 只用条目的 `Before` 与 `Groups`，`RevertActionFacts` 只用 `ids`（3.10.3）。
- 一次恢复只放回一个回收站项：`RestoreNodes` 要同一个 `deleted_at`、只认一个根（`page/adapter/postgres/queries/nodes.sql:258-261`），`bringBack` 只带一个时刻（`page/app/unit_restore.go:115-118`），调度的 `restore()` 取那个顶自己的组与时刻（`revert_schedule.go:431-436`）。调度要父节点活着才放回（`canRestore`，`:262-282`），所以外层的顶自然在里层之前。
- 后台撤销把每个放回拆成逐个成员的步（`revertSteps`，`page/app/jobs.go:413-520`）。在一段"让开"（`asideSpans`，`:556-576`）里的放回只先放回 `needed` 个成员，其余排到段尾之后（`:503-509`）。`needed`（`:586-602`）只看段内后面的放置（`RevertPlace`），注释写明的前提是"a restore under a member of another group is a nested trash item, which L refuses"。
- 批的核对 `CheckRevertSteps` 的模拟在一个组的根那一步就把整个组放回（`revert_part.go:79-84` 调 `p.restore`），所以它看不出"段内一个嵌套的根放在了一个还没放回的成员下"（3.10.6 的实验）。
- 树的读：`TrashChildrenOfAll`（`page/adapter/postgres/queries/trash.sql:64-90`）只认同一个 `deleted_at` 的子节点；app 的替身 `fakeStore.group`（`page/app/fakes_test.go:811-839`）也按时刻。domain 测试的 `world.state()`（`page/domain/revert_test.go:104-142`）读组时用 `world.subtree`（`:84-102`），它对已删的根取**所有**已删的后代、不看时刻：嵌套的根会被读进外层的组，与真库不同（3.10.5）。
- app 的替身时钟每读一次进一微秒（`tickingClock`，`page/app/fakes_test.go:28-33`），每个单元的删除时刻各不相同，同真库。
- 现有的两个性质测试：`TestTheRollbacksStepsReplayToTheBaseline`（`page/app/rollback_test.go:560-621`，2,000 次、`-short` 400 次，`whole` 1–4）与 `TestTheJobsStepsReplayToStar`（`page/app/revert_job_property_test.go:64-127`，同样的次数；生成器 `random`（`:476-586`）、`folder`（`:601-644`），装载 `load`（`:699-740`）把每个回收站的节点都记在 `p.at` 一个时刻）。审查 A 量过：前者不带竞态 0.30 秒、竞态下 1.85 秒，后者 2.53 秒、13.99 秒（`m12/review-a/evidence.txt`）。
- 整个程序：`TestRevertingARollbackConflictsUnderANewFolderHoldingBaselineAndNewPages`（`bootstrap/upgrade_rollback_revert_test.go:30-118`，文件 118 行）的 nested 一格（`:87-114`）在同一个变更集上先同步撤销、再后台撤销（`:99-110`），都答 409；它在第八份（`Makefile:321` 的 `BOOTSTRAP_SEVENTH` 与 `:365` 的 `BOOTSTRAP_EIGHTH` 都点名）。00 号文档引的 `Makefile:263`、`:296`、`:328`、`:342` 是 `5e8af5b5` 的行号，`194ca350` 上是 `:288`、`:321`、`:365`、`:379`（2.1 第 1 条）。

**S1 的各项**：

- **标题键**：`shared.TitleKey`（`shared/title_key.go:18-28`）是 NFC → `cases.Fold()` → NFC，ASCII 走快路；`TitleKeyContext`（`shared/title_key_context.go:15-38`）对超过 4096 字节的用 `transform.Chain(norm.NFC, cases.Fold(), norm.NFC)`（`:29`）；`IndexKeyer.Key`（`linking/domain/index.go:87-109`）自己折叠。`cases.Fold()` 把切罗基文的大写折成小写、小写折成大写，`TitleKey("Ꭰ")` 是 `"ꭰ"`、`TitleKey("ꭰ")` 是 `"Ꭰ"`（审查 A 实测）。搜索的 `searchFold`（`page/domain/search.go:238-255`）在 `TitleKey` 之后把小写换成大写（U+AB70–ABBF → U+13A0–13EF，U+13F8–13FD → U+13F0–13F5），调用在 `:272`、`:282`、`:493`。幂等的样本 `title_key_test.go:44` 没有切罗基文。纯的层只许导入 `golang.org/x/text` 的 `unicode/norm`、`transform`、`cases`（`archtest/rules_test.go:127-133`），`runes` 不在其中。
- **标题 id**：`headingIDs.Transform`（`platform/markdown/parse.go:52-79`）以 `HeadingID(PlainText(h, source))`（`:69`）为 id；`PlainText`（`:81-103`）只写 `*ast.Text` 与 `*ast.String`，`Hider` 跳过，自动链接（`*ast.AutoLink`）与原始 HTML（`*ast.RawHTML`）的文字都丢了。链接的锚点 `AnchorID`（`:168-177`）用写下的全文。`PlainText` 在非测试代码里只有 `:69` 一处调用。
- **标签**：`Tags`（`linking/adapter/postgres/queries/reads.sql:87-94`）按 `tag_key` 分组，列出 `/`；`api/modules/linking.yaml:125-133` 写着 `The tag "/" (#//) is listed, though getTag cannot name it.`
- **名字**：`CheckName`（`shared/name.go:16-27`）先 `TrimSpace`、空答 `required`，含 `Unshowable`（`:36-38`：Cc、Zl、Zp、Bidi_Control）答 `invalid_format`；它管工作区名（`workspace/domain/workspace.go:27`、`:45`）、显示名（`identity/domain/user.go:49`）与 PAT 的名字（`identity/domain/api_token.go:77`）。`CleanClientName`（`shared/client_name.go:28-47`）去掉 `Unshowable`、NFC、去首尾空白、截到 128 字符；`page/domain/client.go:50` 把空的当作没有名字。
- **`log` 的 `path`**：`AtPath`（`page/app/list_changesets.go:120-183`）对 `q.Path` 调 `domain.ParsePath("path", *q.Path, false)`（`:131-136`）；`ParsePath`（`page/domain/path.go:37-53`）去掉开头的 `/` 之后为空答 `required`/"is required"，所以 `""` 与 `"/"` 都答 422。`index` 的 `parent:""` 答 `too_short`（`linking/app/list_index.go:192-197`），`"/"` 是根（`:205-219`）。工具说明在 `page/adapter/mcp/tools.go:238-250`（`logDescription`）与 `log.go:11-18` 的参数；`api/mcp/tools.json` 是手写的契约（`log` 在 `:303-342` 与 `:938-…`），`TestTheToolListsAreTheContract`（`bootstrap/mcp_tools_test.go:156`）逐成员比对，`make gen` 不管它。
- **MCP 超时**：`internalDeadline = "the call ran out of time"`（`platform/httpserver/mcpserver/errors.go:77`），`toolError` 在 `FaultDeadline` 时用它（`:99-101`），`methodError` 也用（`:358-360`）；答复是 `internal_error: <文字>; request id <id>`（`internalAnswer`，`:290-299`）。期限是 `server.request_timeout`（`call.go:46-60`，`bootstrap/mcp.go:92`）。三处测试写着原文：`mcpserver/answer_test.go:413-415`、`prompt_bounds_test.go:498`、`bootstrap/mcp_test.go:232-237`。
- **日志的读**：`ListChangesets.page`（`list_changesets.go:189-208`）读 `ListChangesets`，再经 `logged`（`page/app/history.go:146-193`）读 `ChangesetFacts`、`FirstChanges`、`FirstIngestSources`、`DisplayNames`，各自一条语句、不在事务里（注释 `:49-51`）。`page` 由 `Execute`（`:52-94`）与 `AtPath` 调。变更集的详情：`GetChangeset.Execute`（`page/app/get_changeset.go:35-41`）先 `readableChangeset`（`history.go:59-80`：`FindChangeset`、`WorkspaceOf`、`authorize`），再 `detail`（`:84-94`：`logged` 与 `ChangesetMoves`）；`InNotebook`（`get_changeset.go:67-122`）多读 `changes` 与 `alivePaths`。`HistoryReads`（`history.go:18-24`）在 `page/module.go:318` 建，没有快照；`Deps.Snapshots` 已有（`:239-242`，没有就 panic，`:296-297`），搜索与读路径在用（`:329`、`:367`）。`WithinSnapshot`（`platform/postgres/tx.go:131-144`）是只读的 REPEATABLE READ，必须在最外层（`ErrNestedSnapshot`）。LLM Wiki 的虚拟 `log.md` 已在 `read_path.go:153-167` 的快照里（`page/virtual_files.go:104-197`）。
- **事件流**：hub（`events/app/hub.go:26-31`）只有一个扁平的集合，`open`（`:100-109`）只收 `userID`，不数；`Stream`（`events/app/stream.go:13-25`）不记凭证。`OpenStream.Execute`（`events/app/open_stream.go:30-46`）有完整的 `shared.Actor`（`shared/actor.go:15-23`：`SessionID`、`APITokenID`、`JobID` 恰有其一），只用 `UserID`。处理器在 200 之前把 `Execute` 的错误写成 problem（`events/adapter/http/handler.go:66-74`），`Retry-After` 由 `APIErrors.Write` 写（`platform/httpserver/apierrors.go:231-233`）。`rate_limited` 是契约顶层的码（`api/openapi.yaml:17`），每个操作都可以答；`shared.RateLimited`（`shared/error.go:186-189`）的文字固定。`reset` 的原因是必填的枚举，六个值（`events/domain/frame.go:15-30`，`api/modules/events.yaml:235-243`）。网页对任何 `reset` 立刻重连，不看原因（`W/events/hub.ts:198-226`、`:252-253`）；打开失败时照 `Retry-After` 等（`retryAfterOf`，`:269-273`，对任何带它的答复）。没有限频的日志帮手（`platform/httpserver/limit.go:38-47` 对限流是有意的 debug）。

**S3 的各项**：

- **条目名**：`importArchive.Entries`（`transfer/adapter/archive/archive.go:296-309`）照原样取 `f.Name`，按名字结尾的 `/` 或 `\` 判文件夹（`:304`），只读 `f.Flags&0x1`；`f.Extra` 与 0x7075 没有人读。`archive/zip` 的 `f.Name` 是原始字节，`f.Extra` 是中央目录的扩展字段；它不解析 0x7075（Go 1.27.4 的 `archive/zip/struct.go:75-79`）。`domain.Classify`（`transfer/domain/entry.go:92-178`）先判 UTF-8（`:97-101`，`name_not_utf8`，报告里的路径是 `ToValidUTF8` 之后的），再 `pathOf`（`:181-201`：把 `\` 换成 `/`、去掉 `.` 与空段、拒绝绝对路径与 `..`、每段 NFC）。纯的层不许导入 `golang.org/x/text/encoding`（同上 `archtest`）。报告：`Problem{Path, Code, To}`（`transfer/domain/report.go:136-141`），`Report.Add` 过 `MaxProblems = 1000` 只设 `Truncated`（`:213-227`）；导入的跳过在分类之后立刻进报告（`transfer/app/import.go:151-154`），写入时的 `renamed` 在单元里进（`import_write.go:350`），任务在 `finish`（`import.go:93`）结束。契约的 `TransferProblem.code` 枚举在 `api/modules/transfer.yaml:1079-1095`，网页的 `transfer-report.tsx:92-108` 是以生成的联合类型为键的 `Record`，加一个值不补就编译不过。
- **队列**：`room`（`transfer/app/start.go:203-228`）只比全局：`CountActive`（`transfer/adapter/postgres/queries/jobs.sql:16-18`）加别的笔记本正在上传的导入（`Uploads.others`，`transfer/app/uploads.go:93-103`，按笔记本记，没有人），至多 `MaxQueued`（默认 20）答 `ErrQueueFull`（503 `server_busy`，`Retry-After` 5 分钟，`transfer/domain/errors.go:49-55`）。五处调用：导出的 `admit`（`start.go:186-201`，队列锁之下）、导入的 `Check`（`start_import.go:107-115`，没有事务）与 `write`（`:266-272`，队列锁之下）、后台的 `queueBackground`（`start_background.go:182`，事务之外先判一次；`:197-200`，队列锁之下），回滚经 `queueBackground`（`rollbacks.go:31-38`，在 wiki 的事务里）。`transfer_jobs` 有 `created_by_id`（`00029_transfer_transfer_jobs.sql:22`），部分索引 `transfer_jobs_state_idx` 只收排队与运行中的行（`:61`）。同一模块的 `transfer.busy` 是 409（`errors.go:24-32`）。
- **启动**：`Rescue.AtStart`（`transfer/app/maintain.go:87-94`）只把 `running` 的行记成失败；`Run`（`:101-117`）每 5 分钟（`RescueInterval`，`transfer/adapter/river/jobs.go:47`）再失败心跳旧的、与 River 不再持有的排队行（`lost`，`:119-147`）。River 持有的判断 `Held` 经 `Inserter.Unfinished`，含 `running`（`platform/jobs/inserter.go:50-78`）。进程恰在 River 取到之后、任务开始之前停掉时，River 的行是 `running`、我们的行是 `queued`：`AtStart` 不碰它，`lost` 认为 River 还持有它，直到 River 的救援（`transfer.job_timeout` 加 1 小时）。平台的 `Job.DiscardRunning`（`platform/jobs/jobs.go:33-39`）在 River 启动之前把某一种 `running` 的行记成 discarded，排在同一个 `Job` 的 `Start` 之前（`:101-106`、`:134-148`）；transfer 的五种任务（`jobs.go:135-171`）都没设它，模块的任务次序是导出、导入、恢复、撤销、回滚、收拾、到期、清扫（`transfer/module.go:255-264`），收拾的 `Start` 是 `AtStart`（`river/jobs.go:294-305`）。`README.md:193` 写着这个窗口。

**组合根的分份**（`Makefile:272-365`，`06-environment.md` 第 4 节）：第七份是 `SEVENTH` 减 `EIGHTH`，第八份是 `EIGHTH`；第二份只跳过 `FIFTH`（`:379`）。S2 合并之后两次 `main` 的平均是 352、315.5、335、366.5、389、301.5、279.5、307 秒（00 号文档第 13 节），第七份余量最大。

## 2. 目标与范围

做的事对应 00 号文档第 7 节 P3 一行：4.6 的各项，第 3 节第 4、5 条。

不做：H1、24.1（已知限制，进 v0.2，4.5）；`batch` 单独的期限（4.6.4）；导入时选编码的选项、CP437、Shift-JIS 等的识别（4.6.6）；P4 的实测（日志的新读法由 P4/S3 照新的量）；网页的打磨（P2）。

**移交的落实**（00 号文档"移交的落实"里 P3 的部分）：

| 移交的项 | P3 的落实 |
|---|---|
| [M8-history](handoffs/M8-history.md) 第 5 条（日志的读） | S1：日志与变更集的详情各在一个快照里读（3.8） |
| 同上第 6 条（回滚的变更集的撤销） | S2：3.10；"同一形状在后台恢复、后台撤销里"照 00 号文档 4.6.1"不去造的"；后台撤销的 `needed` 在嵌套的根下出错，由 S2 改（3.10.6） |
| [M7-transfer](handoffs/M7-transfer.md) 第 5 条：按人的队列上限 | S3：3.13 |
| 同上：名称不是 UTF-8 | S3：3.12；Shift-JIS、Big5、EUC-KR 的误读与 GB18030 的四字节写进已知限制（P5/S3 的发布说明）；CP437、编码选项进 v0.2 的 `M12-transfer.md`（收尾时写） |
| 同上：River 取到之后停掉 | S3：3.14；`README.md:193` 那一句 |
| [M5-polish](handoffs/M5-polish.md) 第 10 条的前两点（`#//`、标题 id） | S1：3.4、3.3；第三点（属性链接的文字不经净化）不在附录 A 的 P3/S1 一行，照附录 A 的分类不动 |
| 同上第 25 条（标题键） | S1：3.2；"分词的版本随之提升"不要（00 号文档 4.6.3） |
| 同上第 30 条第 2、4、5 点 | S1：3.5、3.6；第 1、3 点照附录 A 是 C、D，不动 |
| [M5-performance](handoffs/M5-performance.md) 第 6 条（流数） | S1：3.9 |
| [M4-performance](handoffs/M4-performance.md) `:490`（超时的提示） | S1：3.7；硬件下限与 `batch` 的期限不在 P3 |

### 2.1 对 00 号文档的修订

设计时对着代码查到、与 00 号文档不同的，照下面改；Phase 设计合并时由脚本 `apply_p3_design.py` 一起落进 00 号文档（02-workflow 第 2 节第 3 步），每条一个带标签的替换，标签写在条目末尾。

1. **行号**（审查 A 的 M1、C 的 N4）：00 号文档的行号以 `5e8af5b5` 为准，`Makefile` 在 P1 里多了 25–37 行，照它找会找错。第 3 节第 4 条与 4.6.1 的 `Makefile:296`、`:328`，第 7 节"分份"的 `:296`、`:328`、`:263`、`:342`，"对主持者说明的核实"的 `:296`、`:328`，都在后面括注 `194ca350` 上的 `:321`、`:365`、`:288`、`:379`，不改原来的数。`bootstrap/upgrade_rollback_revert_test.go:30`、`:87`、`:99-110` 本来就对，不动。标签 `makefile-*`。
2. **4.6.1 的 `needed`**：00 号文档写"审查 A 没有造出出错的交错，前提已不成立，照改"。设计时造出来了（3.10.6），审查 A 在原型上复现（不改时 400 次里 17 次失败），并沿操作推出生产里也走得到（推出的，没跑）。批切在段尾时，第一批把嵌套的根放回在一个还在回收站里的父节点下；批的核对 `CheckRevertSteps` 整组放回，挡不住；`RestoreNodes` 的 SQL 不查父节点是否活着（推断真库也不挡，没在 PG 上跑；修订核对 A 的 N10）；只有 `needed` 挡。所以 `needed` 的改动是修正，不是防备；4.6.1 与第 10 节第 302 条一行照此改写。标签 `needed`、`risk-302`。
3. **4.6.1 的性质测试**（审查 A 的 M2）：装载也要分时刻（`load` 把所有回收站的节点记在同一个时刻，嵌套的根会被读进外层的组），domain 的 `world` 读组同样要按时刻（3.10.5）。回滚的那个测试只数 `nested`：挂在中间节点下的与两个成员以上的嵌套组在它里面每 400 次只有 0–3 次（审查 A 量过），不是"自然出现"，交给后台撤销的那个测试，"`topOf` 走过回收站根"只在那里看得出。另加 domain 的一张表与 app 的一格确定的后台撤销。标签 `property-tests-*`。
4. **4.6.2 的快照放在哪里**（审查 B 的 N4）：快照放在四个用例里，从各自第一条属于答复的读起，到 `alivePaths` 为止：`Execute` 从 `ListChangesets`，`AtPath` 从找路径的 `StateOf`，`GetChangeset.Execute` 与 `InNotebook` 从 `FindChangeset`。只包 `page` 不会嵌套，但会把 `FindChangeset`、找路径与 `alivePaths` 留在快照之外，它们的结果都在答复里。快照冻结在第一条读开始时、早于它隐含的表锁（3.8.2，设计时与审查 B 都实测过）。`HistoryReads` 加 `Snapshots`。标签 `snapshot`。
5. **4.6.4 的 MCP 超时**：码照旧是 `internal_error`，不改（P3-9）；"要不要改码，由 P3/S1 的设计看契约定"一句改为这个结论。另补一处例外：写的事务在提交时跨过期限，结局未知，不能说"什么也没有写"；`TxManager` 给读写事务的提交错误加标记，MCP 认出它时另用一句（3.7；修订核对 B 的 N14）。标签 `timeout-code`。
6. **4.6.4 的 30.2**（审查 B 的 M1）：整个名字的判断放在 `CheckName` 里，管显示名、工作区名与 PAT 的名字三处（P3-7；00 号文档只写了显示名）。默认可忽略的字符照 Unicode 的推导精确地算（3.5），`۝` 这类单独有字形的格式字符照旧通过；U+2800 不算看不见；标签块不从客户端名里去掉，只由它们组成的名字算空（P3-7；00 号文档把它留给 S1 的实施）。标签 `blank`、`tag-block`。
7. **4.6.5 的 429**：沿用顶层的 `rate_limited`，`Retry-After` 30 秒，文字是这一条自己的；契约只加 `reset` 的原因 `superseded` 与说明；网页不改代码、不加测试：它对任何状态码都照 `Retry-After` 等（`W/services/api.ts:61`、`:70`；审查 B 的过度设计）。被顶掉的一条记 `debug`，不做限频的 `info`（P3-11）。标签 `stream-429`、`stream-log`。
8. **4.6.6 的 T3**：解码在 archive 适配器里做（纯的层不许导入 `x/text/encoding`，`archtest`），domain 只给解读过的名字记报告；文件夹按解读之后的名字判（GBK 的尾字节可以是 0x5C，3.12）；报告的两种新原因在任务结束时加，排在别的问题之后（P3-12）。标签 `t3-report`。
9. **4.6.6 的 T2**（审查 C 的 I2）：状态码是 409，不是推荐的 429（P3-13，理由在那里）。码的次序照今天的检查：导出是 `transfer.too_many_jobs` → 503 → `transfer.busy`；导入、后台的恢复与撤销是 `transfer.busy`（撤销另有冲突、`page.locked`）→ `transfer.too_many_jobs` → 503（3.16；13.1 第 4 条随 S3 改，第 8 节）。回滚算进开始它的人的计数，从不被拒；计数与全局的同一条语句、同一把队列的锁（3.13）。4.6.6、5.1 的表与决定 336 的括注随之。标签 `t2-*`、`decision-336`。
10. **4.6.6 的 T4**（审查 C 的 I3、Q2；主持者定）：00 号文档列了两种做法（`AtStart` 自己先读再丢，或按 discarded 的错误认）。本文取第三种：五种任务各设 `DiscardRunning`（平台已把它排在收拾的 `Start` 之前），`AtStart` 之后立刻对每一种跑一次 `lost`（P3-14）。它让以 SQL 只种排队行、不种 River 行的已有测试变红，种子随之带上一行 River 持有的任务（3.14.3）。测试并进已有的 `TestServeFailsTheJobsLeftRunning`，不另起应用（审查 C 的过度设计）。标签 `t4`；第 7 节 P3 表的 S3 一行（`t4-p3-row`）与第 10 节风险表的"T2、T4 改了任务的入口与启动"一行（`t4-risk`）随之改（修订核对 C 的 M6）。
11. **4.6.3 的比对与 `reindex`**（审查 B 的 M2）："删掉 `searchFold` 的映射前后，同一组查询的搜索结果相同"落成函数的比对（3.2）。"发布说明写一句：跑一次 `nervewiki reindex`"补上实情：`reindex` 重算标题键，同一父页下已有只差切罗基文大小写的兄弟页时它报撞键、整本不重建，要先改名其中一页（第 9 节）。标签 `title-key-*`。
12. **9.6 的变异**：P3 的三行照 7.3 改写，加上本文新找到的：S1 的"`Blank` 不减 Prepended_Concatenation_Mark""`Find` 在快照之外""`alivePaths` 在快照之后""顶掉的流算到它关掉为止""提交的错误不带标记""只读的快照的提交错误也带标记""MCP 不看标记"；S2 的"某一处构造计划不传 `Nodes`"（三处非等价，两处等价）；S3 的"按解读之前的名字判文件夹""解读过的名字的记录排在别的问题之前""回滚也被按人的上限拒绝""先判全局、再判按人""收拾的任务排在五种任务之前"。"合法的 UTF-8 名字也被解读"照第 13 条改。标签 `mutations-p3`。
13. **4.6.6 的 T3 的范围**（P3-16；审查 C 的 Q1，主持者定）：4.6.6 第 2 步的条件"不是合法的 UTF-8"只用来判"这个包是不是 GBK 的"，末条与 9.2 的"解读只改原本不是合法 UTF-8 的名字"改为"没有候选（没有标志、不是合法 UTF-8 的名字）的包，除了有效的 0x7075 一个字节也不改；有标志的名字不改；GBK 的包里，没有标志、解得开的非 ASCII 名字都按 GBK 读"；0x7075 对每个没有标志的名字都读（第 1 步本来没有限定）。它对"有 GBK 候选、又有没标志的真 UTF-8 名字"的包是相对今天的回归，4.6.6 的已知限制与 4.13 照实写。9.6 的变异"合法的 UTF-8 名字也被解读"改为"没有候选的包里合法的 UTF-8 名字也按 GBK 读"。决定 316 的原文（0x7075 先、没有 U+FFFD、一个解不开整包不回退、读 `Flags`）不变，P3-16 落成新的决定。标签 `t3-scope*`、`t3-limits`。
14. **第 7 节与 P2 的重叠**（审查 C 的 M5）：P3 的表下"P3 的三个 Step 与 P2 除翻译文件外不重叠"改为照第 0 节写的实情：S3 的网页改动与 P2 的几乎每个 Step 都碰同一些文件，S3 的网页部分在 P2/S1 合并之后写，后合并的先 `git merge main`、重跑 `make test-web`。标签 `p2-overlap*`。
15. **5.1 的表与 4.13**（审查 B 的 N6、M2）：标题键一行的 `page.name_taken` 是 `page.title_taken`；开始任务一行照第 9 条写 409；4.13 的已知限制加 P3-16 的回归、切罗基文的旧键与 `reindex` 的撞键。标签 `api-table-*`、`limits-*`。

### 2.2 需要负责人确认

没有。P3-13（T2 答 409 而不是 00 号文档推荐的 429）、P3-7（判断放进 `CheckName`，多管工作区名与 PAT 的名字）与 P3-16（GBK 的包里恰好是合法 UTF-8 的名字也按 GBK 读）偏离了 00 号文档的写法，都是主持者权限之内的修正，列在 2.3，负责人可以改判。审查 C 的两问由主持者在第一次修订说明里定了：保留 P3-16（Q1），T4 取 P3-14 并改种子（Q2）。

### 2.3 本文的取舍（主持者的决定，负责人可以改判）

正式编号 361–377，主持者在设计审查之后给的；括号里是临时号，审查记录用它。

- **361（P3-1）`readReverted` 只有一个签名，收 `Nodes`**：`readReverted(items, versions, nodes)`，五处构造计划都传 `s.Nodes`，`RevertWrites` 传 `nil`（只取 `writes`）。不像审查 A 的原型那样另加 `readRevertedNow`：两个入口时，以后新加的一处容易用错那一个。`CheckRevertSteps` 与 `RevertActionFacts` 不读顶，传不传结果一样，照传是为了一致；它们的"不传"变异是等价的，证据是调用链上的代码事实与一个会被抓到的对照（7.3 的 S2-7、S2-8；审查 A 的 M3）。
- **362（P3-2）`needed` 也看段内后面的放回**：放回的 `To.ParentID` 是这一组的成员时，至少放回到它；另加一格确定的 app 测试（3.10.6 的形状）。不改 `CheckRevertSteps` 的模拟：00 号文档定了"调度不改"，而批的核对整组放回是为了后面的成员的前提（`revert_part.go:42-47` 的注释）；挡这一种的是 `needed` 与测试。
- **363（P3-3）domain 的 `world` 按时刻读组**：`world.state()` 读组时只收与根同一时刻删下的，同 `TrashChildrenOfAll`（替身在真东西会失败处失败，05-lessons 第 2 节）。
- **364（P3-4）回滚的性质测试撤销回滚的变更集，请求与后台两条路轮流，只数 `nested`**：种子是偶数走请求的撤销（`f.revert`），奇数走后台（`PlanRevert` 与随机切的批）；对照 agent 留下的状态（树与正文）。挂在中间节点下的与两个成员以上的嵌套组在这里太少（审查 A 量过每 400 次 0–3 次），交给后台撤销的性质测试数（审查 A 的 M2 的甲）。S2 实施另数请求与后台两条路，各有下限（3.22 第 8 条）。
- **365（P3-5）标题键的切罗基文一步用标准库**：折叠之后以 `strings.Map` 把小写换成大写，三处共用 `shared` 里的一个函数；不用 `x/text/runes`（纯的层不许，`archtest`）。`searchFold` 整个删掉，三处调用改调 `shared.TitleKey`；不升分词与 `Extractor` 的版本（00 号文档 4.6.3）。S1 实施不用 `strings.Map`（它把不是 UTF-8 的字节写成 U+FFFD），`TitleKey` 与 `TitleKeyContext` 末尾经不导出的 `cherokeeCapitals` 逐 rune 原地换；`searchFold` 的调用另有 `grep_keys.go` 的一处（3.21 第 1、2 条）。
- **366（P3-6）标题 id 带上自动链接的标签与原始 HTML 的源文本**：`PlainText` 对 `*ast.AutoLink` 写它的 `Label`，对 `*ast.RawHTML` 写它的每一段源文本。这样标题的 id 与 `AnchorID` 对同一段写法的结果相同（两边都用 `slug`）。不升提取规则的版本（13.3 第 6 条；00 号文档 4.6.3）。
- **367（P3-7）名字为空的判断放在 `CheckName` 与 `CleanClientName` 里**：
  - `CheckName` 管显示名、工作区名与 PAT 的名字。一个名字看不见，在哪一处都同样冒充空白，移交的原意是"两处一条规则"；三处都只管新的输入，库里已有的不动。
  - "看不见"是空白与默认可忽略的字符，照 Unicode 的 DerivedCoreProperties 精确地推导（审查 B 的 M1）：`White_Space`，或属于 `Other_Default_Ignorable_Code_Point` ∪ Cf ∪ `Variation_Selector`、又不在 U+FFF9–FFFB、U+13430–13440 与 `Prepended_Concatenation_Mark` 之内。这几项 Go 的 `unicode` 都有；标签块 U+E0000–E0FFF 已在 Go 的 `Other_Default_Ignorable_Code_Point` 里。Unicode 把那三项减去，是因为它们"应当看得见"：`۝`（U+06DD）、`؀`（U+0600）单独就有字形，只写它的名字照旧通过。
  - 先判空、再判长度与 `Unshowable`：只由 Bidi_Control 组成的名字（如单独的 U+202E，它属于 Cf）今天答 `invalid_format`，改后答 `required`（审查 B 的 N13）。
  - U+2800（盲文空白）是图形符号（So），不算看不见。移交把它列在"留下的"里；照"没有图形字符的名称算空"，它是图形字符。
  - 标签块 U+E0000–U+E007F 不从客户端名里去掉：它们是苏格兰、威尔士这类旗帜的 emoji 序列的一部分，去掉会把旗帜变成黑旗；只由它们组成的名字照上一条算空。00 号文档把这一条留给 S1 的实施，本文先定，实施时可以照实改。
- **368（P3-8）`log` 的 `path`**：`"/"` 当作省略（整本）；`""` 答 `validation_failed`，字段 `path`、码 `too_short`、文字 `must not be empty: omit it, or give "/", for the whole notebook`（同 `index` 的 `""` 用 `too_short`）。
- **369（P3-9）MCP 超时的文字**：`the call passed the server's time limit and changed nothing; retry it, splitting a large batch into smaller ones`。码照旧 `internal_error`：改码会改 MCP 的码表与每个工具的码的列表（`TestTheMCPToolsErrorsCarryTheirCodes` 等），agent 已有的处理随之失效；文字已经说清该怎么做。写的事务在提交时跨过期限的，结局未知：`TxManager` 给读写事务的提交错误加标记，MCP 认出它时另用一句，请 agent 先读笔记本（3.7；修订核对 B 的 N14）。S1 第一次修复：提交在调用的期限之前失败的（默认更常见），`FaultUnknown` 一支也认标记、答另一句（决定 483，3.21 第 11 条）。
- **370（P3-10）快照放在四个用例里，从各自第一条属于答复的读起**（2.1 第 4 条；审查 B 的 N3）：`Execute` 从 `ListChangesets`，`AtPath` 从 `Find` 的 `StateOf`，`GetChangeset.Execute` 与 `InNotebook` 从 `FindChangeset`，都到 `alivePaths` 为止。日志的 `Execute` 的授权（`WorkspaceOf`、`authorize`）与带 `page_id` 时的 `NodesNow`（`list_changesets.go:76`）照旧在快照之外：它们不进答复。详情的 `Execute` 要先找到变更集才知道它的笔记本，所以授权跟着进快照（先例 `read_path.go:153-167`）。
- **371（P3-11）事件流**：
  - 凭证是 `Actor` 的 `SessionID`、`APITokenID` 或 `JobID` 之一，连同它的种类；
  - 条数在打开、顶掉与关闭时维护：顶掉的那一条立刻不算（它正在结束）；别的原因重置的流算到它的处理器结束为止：客户端在读时立刻结束，不读时（`overflow` 正是这种）至多一个心跳（`events.heartbeat_interval`，默认 20 秒，至多 50 秒；处理器在写超时之下写出已送到的事件与 `reset` 帧，审查 B 的 M5）。只在账户正好 64 条、这个凭证不到 4 条、它的客户端放弃之后重连时，才会多得到一次 429；不为它改成在重置时减数；
  - 关掉的流从 `held` 里去掉；凭证的切片空了就删掉这一项，账户的数到 0 就删掉这一项，两张表不随进程的寿命涨（审查 B 的 N10）；S1 修复：顶掉时减被顶掉那一条自己的账户，与关闭共用 `uncount`，减到 0 也删（决定 484，3.21 第 12 条）；
  - 账户满了答 429 `rate_limited`，`Retry-After` 30 秒（网页最长的退避），文字 `This account has 64 event streams open: close one, or retry later.`；不加新码：`rate_limited` 是顶层的码，网页照 `Retry-After` 等，不显示文字；
  - 顶掉记一行 `debug`，不做限频的 `info`：限频的日志在服务端没有先例，限流本身也只记 `debug`（`limit.go:38-47`）。
- **372（P3-12）条目名的解读在 archive 适配器里，报告在任务结束时加**：
  - 规则（3.12.1）写在 `transfer/adapter/archive/names.go`：它要 `Flags`、`Extra`、CRC 与 GBK 解码器，都是适配器的东西；domain 不许导入 `x/text/encoding`。
  - 有 UTF-8 标志的名字一个字节也不改；不合法的照旧跳过，也不算进整包的判断。
  - 0x7075 对每个没有标志的名字都读，ASCII 的也读（把写不出的字换成 `?` 的工具靠它留下真名）；CRC 对不上不取。
  - GBK 的读法只在"GBK 的包"里用（有候选、每个候选都解得开），也用于这个包里恰好是合法 UTF-8 的名字（P3-16）。没有候选的包除了 0x7075 一个字节也不改（3.12.3 的差分）。
  - 报告的两种新原因 `name_unicode_path`、`name_gbk`，每个解读过的条目一条；在任务结束（`finish`）时加，排在这次导入找到的别的问题之后：一个几千个条目的 GBK 包的记录不会把真正跳过的条目挤出 1,000 条的上限。进程中途停掉的导入，报告里没有这些记录（它的失败是 `interrupted`）。
- **373（P3-13）按人的上限答 409 `transfer.too_many_jobs`，不带 `Retry-After`**：
  - 00 号文档推荐 429。这里的 429 是平台的限流：`rate_limited` 带 `Retry-After`，按时间窗口，客户端照它自动等了再试（网页的事件流就这样）。按人的上限不随时间解除，要等自己的任务做完或取消一个；同一模块的"你在这本笔记本里已有一个导出"是 409 `transfer.busy`（`errors.go:24-25`），按人的上限是同一类。409 不带 `Retry-After`，不会引着客户端几分钟后白试一次。
  - 上限是常量 `MaxPersonJobs = 5`，不做配置项。
  - 先判按人、再判全局：两样都满时说"你自己的"，取消一个自己的同时也空出全局的名额。
  - 与同模块别的检查的先后照今天的代码（审查 C 的 I2）：导出先 `room`、后 `Exporting`，所以是 `transfer.too_many_jobs` → `server_busy` → `transfer.busy`；导入的 `Check`、后台的恢复与撤销先 `claim`（`transfer.busy`）、后 `room`，所以是 `transfer.busy` → `transfer.too_many_jobs` → `server_busy`（撤销在 `page.locked` 之后）。与 13.1 第 4 条一致，那一条随 S3 加上新码（第 8 节）。
  - 回滚的任务算进开始它的人的计数（它占着队列），但从不被按人的上限拒绝（00 号文档 4.6.6，修订核对第 2 轮 A 的 N14）。
  - 正在上传的导入按人记（`Uploads` 记下开始它的人）。
- **374（P3-14）T4：五种任务各设 `DiscardRunning`，`AtStart` 之后对每一种跑一次 `lost`**：
  - 平台已经把每个 `Job` 的丢弃排在它的 `Start` 之前，五种任务在模块的任务表里排在收拾之前（`transfer/module.go:255-264`），所以收拾的 `AtStart` 运行时，上一个进程留下的 `running` 的 River 行都已丢掉；`lost` 这时就认得出没有 River 持有的排队行，与它每 5 分钟做的一样。
  - 不另写读 River 表的 SQL，也不按丢弃的错误文字去认。
  - 依赖任务表的次序，由一个模块的测试与整个程序的测试钉住（3.14）。
  - 它碰到以 SQL 只种排队行、不种 River 行的已有测试：今天的 `AtStart` 不看排队行，它们照旧排队；改了之后在启动时被记成失败。这些种子改为带一行 River 持有的任务（`ScheduledAt` 推后一小时，照 `transfer/rescue_test.go:47` 的 `later`），更像真的：真的排队行总有它的 River 行。改哪些、各自的反向对照在 3.14.3（审查 C 的 I3；主持者定，Q2）。不取 00 号文档的两种窄做法：(a) 要在平台的丢弃之外自己读 River 的表，(b) 依赖丢弃的错误文字。
- **375（P3-15）新的整个程序的测试都点名进第七份**（只加进 `SEVENTH`），302 那一格改名之后仍在第八份（两个模式都改）；名字避开第二、第五份的模式（3.19）。T4 不加新的整个程序的测试，并进第四份的 `TestServeFailsTheJobsLeftRunning`（审查 C 的过度设计）。
- **376（P3-16）GBK 的包里恰好是合法 UTF-8 的名字也按 GBK 读**：
  - 00 号文档 4.6.6 写"解读只改原本不是合法 UTF-8 的名字"。设计时算了：GB2312 一级汉字约 18% 的单字、两个字的名字约 3% 的 GBK 字节恰好是合法的 UTF-8，常用的 `目录`、`图片` 都是（3.12.1）。照 00 号文档的写法，一个中文 Windows 的包里 `图片/a.png` 进 `ͼƬ`、`图片/截图.png` 进 `图片`，同一个文件夹分成两个，报告却说解读过了。
  - 所以候选只用来判"这个包是不是 GBK 的"；是的话，没有标志、没有有效 0x7075、含非 ASCII 字节、按 GBK 解得开的名字都按 GBK 读。
  - **这是一种回归**（审查 C 的 Q1）：一个包同时有没有标志的真 UTF-8 名字与解得开的 GBK 候选时，今天真 UTF-8 名字导入得对、候选被跳过；P3-16 之后真 UTF-8 名字会被误读成 GBK（`ü.md` 读成 `眉.md`）。发布说明的已知限制照这样写成"相对今天的回归"，不只是"会被误读"。
  - 为什么仍取它（主持者定）：中文 Windows 的包里 `图片`、`目录` 这类常用文件夹被拆成两个，是主要读者都会碰到的错；混合的包少：7-Zip、WinRAR、Python、Java、Go 写非 ASCII 名字都带标志，macOS 的包与本程序的导出没有候选（审查 C 核对）；误读的名字在报告里逐个列成 `name_gbk`，看得见。
  - 它偏离的是 00 号文档 4.6.6 第 2 步的条件"不是合法的 UTF-8"与末条（9.2 同）"解读只改原本不是合法 UTF-8 的名字"；决定 316 的原文（先 0x7075、没有 U+FFFD、一个解不开整包不回退、读 `Flags`）不变。它也偏离 05-lessons 第 5 节"宁可不做（只漏、不写错）"：那一条说的是手写的非标准读法追着罕见的形状跑；这里不是追形状，是一条按包定的边界，写错的情形（混合的包）罕见、看得见（报告），只漏的那一边（拆开的文件夹）常见、看不见（报告说解读过了）。
  - 全部是恰好合法的名字、没有一个候选的包（只有 `图片/a.png`）认不出，照旧导入成 `ͼƬ`，与今天相同（已知限制）。
- **377（P3-17）回收站一行在后台开始答 `transfer.too_many_jobs` 时留着"在后台恢复"的按钮**：同 `server_busy`，等自己的一个任务结束再按（`offeredOf`，4.2）。撤销对话框与导出、导入的对话框照旧只显示文字（它们本来就留着确认按钮）。

## 3. 服务端

### 3.1 S1 的文件

| 文件 | 改动 |
|---|---|
| `shared/title_key.go`、`shared/title_key_context.go`；`shared/title_key_test.go`、`title_key_context_test.go` | 切罗基文一步（3.2） |
| `linking/domain/index.go`；`index_test.go` | `IndexKeyer.Key` 同一步，随机差分的字母表加切罗基文（3.2）；新测试 `TestAnIndexKeyIsCherokeesCapitals`（3.21 第 4 条） |
| `page/domain/search.go`；新的 `page/domain/search_fold_internal_test.go` | 删掉 `searchFold`；新旧比对（3.2）；S1 实施另改 `grep_keys.go` 的 `keyReader.fold`、几个测试与生成 `grep_tables.go` 的模板（3.21 第 1 条） |
| `page/app/` 的建页测试 | 只差切罗基文大小写的两页被拒（3.2） |
| `platform/markdown/parse.go`；`parse_test.go`、`links_test.go`、`render_test.go`、`obsidian/view_test.go` | `PlainText` 带上自动链接与原始 HTML（3.3）；维基链接的一格在 `obsidian/view_test.go`（3.21 第 3 条） |
| `linking/adapter/postgres/queries/reads.sql`、生成的 `gen/reads.sql.go`；`reads_test.go`；`api/modules/linking.yaml` | 标签列表不列 `/`（3.4） |
| `shared/name.go`、`shared/client_name.go`；`name_test.go`、`client_name_test.go`；`identity/domain/user_test.go` | `Blank` 与两处的判断（3.5） |
| `page/app/list_changesets.go`；`page/adapter/mcp/log.go`、`tools.go`；`api/mcp/tools.json`；`page/app/path_history_test.go`；`bootstrap/mcp_reads_test.go`、`mcp_history_test.go` | `log` 的 `path`（3.6） |
| `linking/app/backlinks_test.go` | S1 修复：替身的 `paths.Check` 照真的拒尾随的 `/`，`"/"` 答 `required`（3.6、3.21 第 13 条） |
| `platform/postgres/tx.go`、`tx_test.go`；`platform/httpserver/mcpserver/errors.go`；`answer_test.go`、`prompt_bounds_test.go`；`bootstrap/mcp_test.go` | 超时的文字；提交错误的标记与它的文字（3.7）；S1 修复：`FaultUnknown` 一支也认标记（决定 483，3.21 第 11 条） |
| `page/app/history.go`、`list_changesets.go`、`get_changeset.go`；`page/module.go`；`page/app/history_test.go`、`path_history_test.go` 与它们的替身；新的 `bootstrap/history_snapshot_test.go` | 快照（3.8）；S1 实施另改 `history_fakes_test.go`、`ingest_history_test.go` 的调用表与 `bootstrap/interleavings_virtual_test.go` 的计数（3.21 第 5、6 条） |
| `events/app/hub.go`、`stream.go`、`open_stream.go`；`events/domain/frame.go`、`errors.go`；`events/app/hub_test.go`、`events/adapter/http/handler_test.go`；新的 `bootstrap/events_streams_test.go`；`api/modules/events.yaml` | 流数（3.9）；S1 实施：hub 的测试在新的 `hub_bounds_test.go`（`hub_test.go` 不动），两张表的长度经 `export_test.go`（3.21 第 8 条） |
| `web/packages/api-client/src/schema.gen.ts`、`api/dist/openapi.yaml`（`make gen`） | 生成物（第 4 节） |
| `W/events/hub.ts`、`W/services/event.service.ts` | S1 修复：只改注释，503 与 429 都带 `Retry-After`（决定 485，3.21 第 14 条） |
| `Makefile` | 第七份点名两个新测试，计数的注释与 `make help` 的第七份随之（3.19） |
| `README.md` 事件流一节（`:198`、`:207`、`:210`）与 MCP 一节（`:223`） | 第 8 节第 7–10 条 |

### 3.2 S1：标题键（25.1）

**规则**：折叠之后把切罗基文的小写换成大写。Unicode 的 CaseFolding 把切罗基文折到大写（小写是后来编进去的），`cases.Fold()` 却把大写折成小写、小写折成大写，所以 `TitleKey` 不幂等。换成大写之后，任何一种写法的键都是大写，再算一次不变。

```go
// shared/title_key.go

// cherokeeCapital is CherokeeCapital's body: Unicode's case folding takes
// Cherokee to its capitals, cases.Fold turns its capitals small and its
// small letters capital, so a key would not be its own key (M12 design
// 4.6.3, 25.1).
func cherokeeCapital(r rune) rune {
	switch {
	case 0xAB70 <= r && r <= 0xABBF: // ꭰ–ꮿ: Ꭰ–Ꮿ
		return r - 0xAB70 + 0x13A0
	case 0x13F8 <= r && r <= 0x13FD: // ᏸ–ᏽ: Ᏸ–Ᏽ
		return r - 0x13F8 + 0x13F0
	}
	return r
}

// CherokeeCapital is r, a small Cherokee letter, as its capital; any other
// rune as it is: the step of a title key after its fold, for the keyers
// that fold by themselves (linking's IndexKeyer). A letter and its capital
// are three bytes each in UTF-8, so a keyer may swap them in place.
func CherokeeCapital(r rune) rune

func TitleKey(s string) string // 非 ASCII 的一支：strings.Map(CherokeeCapital, norm.NFC.String(cases.Fold().String(norm.NFC.String(s))))
```

- `TitleKeyContext`（`title_key_context.go:15-38`）：短的一支调 `TitleKey`；长的一支读完之后对结果 `strings.Map(CherokeeCapital, …)`（逐 rune，不在 4096 字节的块上做，免得切开一个字母）。
- `IndexKeyer.Key`（`linking/domain/index.go:87-109`）：`norm.NFC.Append` 之后、截到 `MaxIndexKey` 之前，逐 rune 以 `shared.CherokeeCapital` 在 `k.key` 上原地换（两段的字母在 UTF-8 里都是 3 字节）。不经字符串：`TestAnIndexKeyerTakesEachKeyWithItsBuffers` 要 `AllocsPerRun ≤ 4`（审查 B 的 N12）。
- 标准库的 `strings.Map`，不用 `x/text/runes`：纯的层只许 `norm`、`transform`、`cases`（`archtest/rules_test.go:127-133`；P3-5）。S1 实施不用 `strings.Map`：它把不是 UTF-8 的字节写成 U+FFFD，`TestTitleKeysKeepASCIIAndUnicodeBoundaries` 要它们原样留着；`TitleKey` 的非 ASCII 一支与 `TitleKeyContext` 的长的一支末尾经不导出的 `cherokeeCapitals`，逐 rune 解码、遇到小写时在一份拷贝里原地换 3 个字节，没有小写时不分配（3.21 第 2 条）。上面代码块与 `TitleKeyContext` 一条里的 `strings.Map(CherokeeCapital, …)` 读作这一步。
- `page/domain/search.go`：`searchFold`（`:238-255`）删掉，`:272`、`:282`、`:493` 改调 `shared.TitleKey`，注释照实改（S1 实施：`grep_keys.go:399` 的 `keyReader.fold` 也调它，一并改，3.21 第 1 条）。不升分词与 `Extractor` 的版本：改了之后键与原来 `searchFold` 的结果逐字节相同，分出的词不变（00 号文档 4.6.3）。

**测试**：

- `TestTitleKeyIsStable`（`title_key_test.go:43`）的样本加 `"Ꭰ"`、`"ꭰ"`、`"Ᏸ"`、`"ᏸ"`、`"Ꭰꭱᏸx"`（大小写混着、带 ASCII）；`TestTitleKeysMatchAcrossCaseAndNormalization`（`:9`）加一对 `"ᏣᎳᎩ"`/`"ꮳꮃꭹ"`。
- `title_key_context_test.go`：一个超过 4096 字节、全是切罗基文小写的标题，`TitleKeyContext` 等于 `TitleKey`，也等于它的大写写法的键。
- linking：`IndexKeyer.Key("ꭰ") == Key("Ꭰ")`，且 `Key(Key(x)) == Key(x)`；随机差分的字母表（`linking/domain/index_test.go:75`）加两段切罗基文的大小写，让原地换的那一段也在差分里（S1 实施：前两条写成新测试 `TestAnIndexKeyIsCherokeesCapitals`，3.21 第 4 条）。
- page/app：同一父页下先建 `Ꭰ`，再建 `ꭰ`，第二个答 `page.title_taken`（建页的现有测试，替身照 `NameKey` 判重）。
- **新旧比对**（新的 `page/domain/search_fold_internal_test.go`，`TestSearchFoldIsTheTitleKeyNow`）：测试里留一份改之前的 `searchFold`（改之前的 `TitleKey` 的式子加那段映射，原样抄进测试，注释写明出处 `194ca350`），在一组语料上比它与 `shared.TitleKey`：两段切罗基文的每个码位单独一次、成对一次；三个混写的词；`search_test.go` 里已有的查询与页名（希腊文的 Σ、土耳其文的 İ、德文的 ß、NFD 的组合字符）。逐个相等。这就是 00 号文档"删掉前后同一组查询的搜索结果相同"：搜索只经这个函数比较键与词。
- 不加迁移。`nervewiki reindex` 先以 `Rekey` 重算每个节点的标题键（`linking/app/reindex.go:106`、`page/link_targets.go:252-274`、`page/domain/rekey.go:24` 用 `shared.TitleKey`），升级之后跑一次就改对旧键；同一父页下已有只差切罗基文大小写的兄弟页时它报撞键、整本不重建（`reindex.go:106-109`）。发布说明照第 9 节写（审查 B 的 M2）。`checkPages` 只数同一父节点下重复的 `name_key`，不与 `TitleKey(name)` 比。

### 3.3 S1：标题 id（10.2）

```go
// platform/markdown/parse.go

// PlainText is the text of n's descendants, from source, as written, but
// for what a Hider hides: a heading's, for its id. An autolink is its
// label and raw HTML its source, as an anchor written to the heading has
// them (AnchorID), so that the two meet (M12 design 4.6.3, 10.2).
func PlainText(n ast.Node, source []byte) string
```

`ast.Walk` 里加两支：`*ast.AutoLink` 写 `c.Label(source)`；`*ast.RawHTML` 写 `c.Segments` 的每一段的 `Value(source)`。`Hider` 一支照旧在前。

**测试**：`TestHeadingsGetPrefixedUniqueIDs`（`parse_test.go:94`）加两格：`# See https://a.example now` 的 id 是 `nw-see-httpsaexample-now`，`# A <span>b</span> c` 的是 `nw-a-spanbspan-c`；`TestHeadingIDIsAHeadingsID`（`links_test.go:165`）对这两段写法断言 `AnchorID(text)` 等于标题的 id；`TestALinkToAnAnchorAloneLeadsToItsHeading`（`render_test.go:127`）加一格 `[[#See https://a.example now]]` 渲染出的 `href` 指到那个标题的 id。S1 实施：`platform/markdown` 没有维基链接，`render_test.go` 那一格用 Markdown 链接 `[a](<#See https://a.example now>)`，维基链接的一格在 `obsidian/view_test.go`；这两格守锚点一侧，没改时也绿（3.21 第 3 条）。审查之后四个测试另加 `# Go www.example.com now` → `nw-go-wwwexamplecom-now`：linkify 的 `www.` 链接的 `URL` 带 `http://`，分得出写的是 `Label`（S1 审查 A 的 M1）。期望的 id 写死，不由被测的 `slug` 算（05-lessons 第 1 节）。

### 3.4 S1：标签列表不列 `/`（10.1）

```sql
-- name: Tags :many
-- A notebook's tags by key (M6/P5 design 5): each as most of its pages write it, the first by bytes of those as
-- many, and how many pages have it. The tag "/" (#//) is left out: getTag cannot name it (M12 design 4.6.4, 10.1).
SELECT mode() WITHIN GROUP (ORDER BY t.tag COLLATE "C")::text AS tag, count(*) AS pages
FROM page_tags t
WHERE t.notebook_id = sqlc.arg(notebook_id) AND t.tag_key <> '/'
GROUP BY t.tag_key
ORDER BY t.tag_key;
```

- `TagPages` 不动：它对 `/` 本来就走不到（审查 A 核对）。补全取同一个读，随之不列。
- `api/modules/linking.yaml:125-133` 的末句改为 `The tag "/" (#//) is not listed: getTag cannot name it.`，`make gen`。
- **测试**：`TestTheTagsOfANotebook`（`linking/adapter/postgres/reads_test.go:417`）的种子（`tagged(...)` 直接造的 `Facts`，`reads_test.go:406-411`，不解析 Markdown）加一页 `tagged("/", "X")`，期望 `[{a/b 1} {Project 4} {X 3}]`：`X` 从 2 变 3，证明那一页确实被读到（审查 B 的 N7）。先在没改的树上看到 `/` 被列出。

### 3.5 S1：只由看不见的字符组成的名字（30.2）

```go
// shared/name.go

// Blank reports whether s shows nothing: every rune of it white space or
// a default ignorable code point (M12 design 4.6.4, 30.2). Go's unicode
// does not hold the derived Default_Ignorable_Code_Point; it is derived
// here as DerivedCoreProperties does: Other_Default_Ignorable_Code_Point,
// Cf and Variation_Selector, but White_Space, U+FFF9–FFFB, U+13430–13440
// and Prepended_Concatenation_Mark, format characters that should be
// visible (۝ alone shows). A name of only these passes for empty: shown,
// it reads as no name at all.
func Blank(s string) bool

func CheckName(field, s string, maxLen int) (string, *FieldError) // TrimSpace 之后：s == "" || Blank(s) 答 required，在 too_long 与 invalid_format 之前（只由 Bidi_Control 组成的从 invalid_format 变成 required）
```

- `CleanClientName`（`client_name.go:28-47`）：最后 `Blank(s)` 时交回 `""`，`page/domain/client.go:50` 照旧把它当作没有名字。
- `Unshowable`、标题的检查、库的 CHECK、迁移都不动（00 号文档 4.6.4）。`TestChecksRejectCounterexamples` 不改，照过。
- 三处调 `CheckName` 的：显示名、工作区名、PAT 的名字都得到这条规则（P3-7）。只管新的输入。
- **测试**：
  - `TestCheckName`（`shared/name_test.go:10`）加格：只由 U+200B、U+2060、U+3164 组成的答 `required`；前后带空格的同样；只由 U+E0041 U+E0042（标签字符）组成的答 `required`；`I ❤️ Go`（U+FE0F）、`👨‍👩‍👧`（ZWJ）、`می‌خواهم`（ZWNJ）、苏格兰旗（U+1F3F4 加标签字符）照旧通过，原样交回；U+2800 单独一个照旧通过（P3-7，写在注释里）；`۝`（U+06DD）、`؀`（U+0600）单独一个照旧通过（审查 B 的 M1）；单独的 U+202E 答 `required`，改之前是 `invalid_format`（审查 B 的 N13）；一个 300 个 U+200B 的名字答 `required` 而不是 `too_long`。
  - `Blank` 自己的一格逐码位比对：对 U+0000–U+10FFFF 每个码位，`Blank(string(r))` 等于照 DerivedCoreProperties 的式子由 `unicode` 的表算出的（`White_Space` 或 DI）。式子写在测试里、不调 `Blank`；样本码位的期望另写死几个（U+200B、U+3164、U+FE0F、U+E0041、U+06DD、U+FFF9、U+13430、U+2800），免得两处同错（审查 B 的 `blank-probe.out` 是它的原型）。
  - `TestCleanClientName`（`client_name_test.go:22`）同样几格，空的交回 `""`；`TestCleanClientNameIsIdempotent` 照过。
  - `identity/domain` 的显示名测试加一格（只由 U+200B 组成的 `display_name` 答 `required`）：这是第 3 节第 5 条点名的那一处。
  - 05-lessons 第 8 节：测试文件里的 `\u` 转义由 Python 以 `chr(92)` 写，lint 之前扫一遍非 ASCII 的字符；需要原样字符的样本（emoji、波斯文）写成 Go 的 `\U` 转义，不直接写字符。

### 3.6 S1：`log` 的 `path`（30.4、30.5）

`AtPath`（`list_changesets.go:131-136`）：

```go
	var path domain.Path
	switch {
	case q.Path == nil || *q.Path == "/":
		// The whole notebook, as list's and index's root (M12 design 4.6.4, 30.4).
	case *q.Path == "":
		return PathLog{}, shared.Invalid(shared.FieldError{Field: "path", Code: shared.FieldTooShort,
			Message: `must not be empty: omit it, or give "/", for the whole notebook`})
	default:
		if path, err = domain.ParsePath("path", *q.Path, false); err != nil {
			return PathLog{}, err
		}
	}
```

`"/"` 之后走的是今天 `q.Path == nil` 的那一支（实施时以一个变量记"有没有路径"，不改 `ParsePath`）。

- 说明：`log.go:13` 的 `path` 参数改为 `Only the changesets that changed the page or attachment at this path, from the notebook's root; "/" or omitted for the whole notebook. "" is refused.`；`logDescription`（`tools.go:245-247`）的 `path keeps …` 一句加 `"/" or no path keeps them all`。`api/mcp/tools.json` 两处 `log` 的 `description`（`:306`、`:940`）与 `path`（`:316`、`:954`）手改成同样的字（`TestTheToolListsAreTheContract` 逐成员比对，审查 B 的 N8）。
- `index` 不动（00 号文档 4.6.4）。
- **测试**：`TestListChangesetsAtPathAnswersItsCodesInOrder`（`page/app/path_history_test.go:405`）加 `""` 答 `validation_failed path too_short`；`TestListChangesetsAtPathListsAsExecute`（`:333`）加 `"/"` 与省略的答复逐字相同；`bootstrap/mcp_reads_test.go` 的 `historyCodeCalls`（`:557`）加 `{nb, "log", {"path": ""}, "validation_failed"}`；`TestTheMCPHistoryIsRESTs` 的 `log` 子测试（`bootstrap/mcp_history_test.go:269`）加 `path: "/"` 与 REST 的整本日志相同。`index` 的 `parent:"/"` 已有测试（`linking/app/list_index_test.go:325` 的 "the root" 一格），照过；它抓 00 号文档 9.6 的"`index` 的 `parent:\"/\"` 被拒"（7.3 的 S1-27）。S1 实施时 linking 的替身 `paths.Check` 收 `"/"`，那一格抓不到 S1-27；修复让替身照真的拒尾随的 `/`（`"/"` 答 `required`），之后 "the root" 等三格抓到（3.21 第 13 条）。

### 3.7 S1：MCP 调用超时的文字

```go
// platform/httpserver/mcpserver/errors.go

	// internalDeadline: a call past server.request_timeout. Its transaction
	// rolled back whole; an agent should retry, a batch split smaller (M12
	// design 4.6.4; M4-performance hand-off, after item 22).
	internalDeadline = "the call passed the server's time limit and changed nothing; retry it, splitting a large batch into smaller ones"
```

- 码照旧 `internal_error`（P3-9），`; request id <id>` 照旧接在后面，`warn` 一行照旧（`MCP call deadline exceeded`）。
- `methodError`（`:358-360`）与 `toolError` 一样经下面的 `deadlineText` 选文字：那里是 prompt 与方法级的错误，"changed nothing"同样成立（它们不写，走不到带标记的一支）。
- **测试**：三处写着原文的测试（`answer_test.go:413-415`、`prompt_bounds_test.go:498`、`bootstrap/mcp_test.go:232-237`）改成整句比对新文字，不比片段（9.1 的约定）；它们抓"超时的文字照旧"（7.3 的 S1-28）。
- **提交时跨过期限**（审查 B 的"未核实的怀疑"；修订核对 B 的 N14 照 pgx 的代码判它走得到，现在定）：
  - **路径**：写的事务的 `COMMIT` 在 `TxManager.end` 里以 `WithoutCancel` 跑，限 `database.commit_timeout`（`platform/postgres/tx.go:197-203`）。`COMMIT` 拿不到回答、跨过这个时限时，连接默认的 `DeadlineContextWatcherHandler`（pgx v5.11.0 `pgconn/config.go:472-474`）让网络读超时，错误包成 `errTimeout{ctx.Err()}`，即 `DeadlineExceeded`（`pgconn/errors.go:154-168`）；`TxManager` 再包成 `commit transaction: %w`（`tx.go:176-177`）。提交开始得晚、请求的期限也已过时，`ProblemOf` 归为 `FaultDeadline`（`platform/httpserver/apierrors.go:278`），今天答"ran out of time"，改了文字之后会答"changed nothing"；提交的时限从提交开始算，默认 2 秒短于请求的 15 秒，更常见的是提交先超时、调用的期限还没到，`ProblemOf` 归为 `FaultUnknown`，答 "the server failed"（S1 审查 B 的 M1 以 overlay 探针核实了两种；下面"文字"一条，3.21 第 11 条）。可 `TxManager` 的注释写明这时结局未知（`tx.go:91-92`）。理由（修订核对第 2 轮 B 的 M6 改正了第二次修订的说法）：这个 handler 自己只设读写的期限，但读超时之后 `MultiResultReader.receiveMessage` 调 `asyncClose`（pgx v5.11.0 `pgconn/pgconn.go:1613-1620`），`asyncClose` 向服务端发 `CancelRequest`、再发 `Terminate`（`pgconn.go:770-790`）。提交前段的停顿（延迟触发器、等锁）会被取消，事务回滚，什么也没写；过了提交点的停顿（例如等同步复制：本地已经提交，取消只得一个 WARNING），或者取消送不到服务端（网络断了）时，提交会落下。调用方分不出是哪一种，所以结局未知。审查 B 复现了前一半（`m12/p3-check2-b/commit-probe.log`、`commit-probe2.log`，两次相同）：照下面 (b) 的造法，`WithinTx` 在 101–107 ms 交回 `commit transaction: timeout: context deadline exceeded`，包着 `DeadlineExceeded`；等满 5 秒，`notes` 里仍是 0 行。后一半（提交落下）没有造过。
  - **标记**（`platform/postgres/tx.go`）：读写事务的 `COMMIT` 失败时，`run` 交回 `*commitError`，同 `beginError` 的写法（`tx.go:181-195`）：`Error()` 照旧是 `commit transaction: <原因>`（日志的字不变），`Unwrap` 交出原因，方法 `TxCommitFailed() bool` 标记它。只读的快照（`WithinSnapshot`，`opts.AccessMode == pgx.ReadOnly`）的提交失败不标记、照旧 `fmt.Errorf`：它什么也没写。`shared` 不加对应的函数：模块里没有人要认它，只有 MCP 的文字用（平台不导入 `shared`，按结构认，同 `apierrors.go:20` 的写法）。
  - **文字**（`mcpserver/errors.go`）：`toolError` 与 `methodError` 的 `FaultDeadline` 一支经同一个函数选文字；码、`; request id <id>`、`warn` 一行都照旧。REST 的 500 不带文字，不变。S1 第一次修复：`FaultUnknown` 一支也认标记（`committing`、`failedText`），带标记的答 `the server failed as its changes were being saved, so they may or may not have been; read the notebook to see whether they were before retrying it`，码照旧、日志照旧 error（决定 483，3.21 第 11 条）。

```go
	// internalDeadlineSaving: a call past server.request_timeout whose
	// transaction's COMMIT got no answer within database.commit_timeout:
	// its outcome is unknown (TxManager), its changes saved or not.
	internalDeadlineSaving = "the call passed the server's time limit as its changes were being saved, so they may or may not have been; read the notebook to see whether they were before retrying it"

// deadlineText is a deadline's message: a COMMIT's that got no answer
// may have saved its changes; any other deadline changed nothing.
func deadlineText(err error) string {
	var c interface{ TxCommitFailed() bool }
	if errors.As(err, &c) && c.TxCommitFailed() {
		return internalDeadlineSaving
	}
	return internalDeadline
}
```

  - **测试**：
    - `platform/postgres/tx_test.go` 新的 `TestACommitThatFailsIsMarked`（真库，每格一个 `newNotes`：各是一个新的库）三格。(a) 读写事务的 `fn` 里由池的另一条连接结束本事务的后端（同 `TestWithinTxReportsAFailedRollback` 的 `pg_terminate_backend`），`COMMIT` 失败：带标记、以 `commit transaction: ` 开头、`Unwrap` 交出原因。(b) 读写事务的 `COMMIT` 跨过 `commitTimeout`：`notes` 上一个 `DEFERRABLE INITIALLY DEFERRED` 的约束触发器在提交时 `pg_sleep(2)`，这一格的 `TxManager` 的 `commitTimeout` 是 100 ms。只断言两件事：错误带标记，`errors.Is(err, context.DeadlineExceeded)`。这一造法下取消让它回滚，测试不断言结局。用时约 0.1 秒，加收尾。审查 B 在没改的代码上跑过 (a)、(b)、(c) 的造法（修订核对第 2 轮，`m12/p3-check2-b/`）：(a) 没有标记而红，(b) 同；(c) 照绿，只守 S1-33。实施时照旧先在没改的代码上看 (a)、(b) 红。(c) `WithinSnapshot` 里同 (a) 结束后端：`COMMIT` 失败、不带标记。
    - `mcpserver` 的 `answer_test.go` 加一格（同 `:413-415` 的超时一格）：工具交回一个带 `TxCommitFailed` 的、包着 `context.DeadlineExceeded` 的错误，ctx 已过期：整句比对 `internalDeadlineSaving` 加 `; request id`；不带标记的照旧是 `internalDeadline` 那一格。`methodError` 不另加格：prompt 与方法不写，走不到这一支，它与 `toolError` 共用 `deadlineText`。S1 第一次修复另加两格整句比对：`answer_test.go` 的 "a fault at a commit"（活着的 ctx、带标记），`prompt_bounds_test.go` 的 `unknown_commit`（3.21 第 11 条）。
    - 7.3 的 S1-32–S1-34 是它们的变异。

### 3.8 S1：日志与变更集的详情各在一个快照里（H5）

#### 3.8.1 改动

```go
// page/app/history.go

// HistoryReads are the reads of the history (M8/P3 design 3.4). A log's
// page and a changeset's detail are read in one snapshot (Snapshots), as
// of the moment of their first read: a revert committed meanwhile is in
// all of them or none (M12 design 4.6.2, H5).
type HistoryReads struct {
	Notebooks Notebooks
	History   History
	Names     Names
	Auth      shared.Authorizer
	Paths     PathStore
	Snapshots shared.Snapshots
}
```

- `page/module.go:318`：`HistoryReads{…, Snapshots: d.Snapshots}`。`d.Snapshots` 已是必有的（`:296-297`）。
- `ListChangesets.Execute`（`list_changesets.go:52-94`）：授权、`NodesNow` 照旧在快照之外；`return l.page(…)` 改为在 `l.r.Snapshots.WithinSnapshot` 里调 `page`。快照的第一条语句是 `ListChangesets`。
- `ListChangesets.AtPath`（`:120-183`）：`decideNotebook` 在外；从 `NewNodePaths(…).Find`（`:158`）到 `alivePaths`（`:178`）在一个快照里。第一条读是 `Find` 的 `StateOf`；不带路径时是 `page` 的 `ListChangesets`。
- `page`（`:189-208`）本身不开快照（它在两个用例的快照里，`WithinSnapshot` 不许嵌套）。
- `GetChangeset.Execute`（`get_changeset.go:35-41`）：`readableChangeset` 与 `detail` 一起在一个快照里；授权跟着进去（先例 `read_path.go:153-167`）。第一条语句是 `FindChangeset`。
- `GetChangeset.InNotebook`（`:67-122`）：`decideNotebook` 在外；从 `FindChangeset`（`:88`）到 `alivePaths`（`:115`）在一个快照里。
- 注释 `list_changesets.go:49-51`、`get_changeset.go:34`、`:65-66`（`InNotebook` 的，修订核对 B 的 N16）的"A read takes no lock and opens no transaction"改为"Its reads, from the first read of the answer on, are one read-only snapshot"（`AtPath` 的第一条读是 `StateOf`，不是历史的读，修订核对 B 的 N15）。
- MCP 的 `log`（`AtPath`）与 `changeset`（`InNotebook`）一并得到。LLM Wiki 的 `log.md` 已在快照里，不动。

#### 3.8.2 快照从哪一刻起

REPEATABLE READ 的快照冻结在事务的第一条查询开始时（PostgreSQL 文档 13.4 "Data Consistency Checks at the Application Level"：快照可能早于这条查询取得的锁）。所以第一条读即便在等别人的表锁，它与之后的读看到的都是它开始的那一刻。设计时在 `postgres:18.7-trixie` 上实测过（`m12/p3-design/evidence.txt`）：快照里的第一条读在等 `ACCESS EXCLUSIVE` 锁，其间另一个事务提交了一行，第二条读看不到它；简单协议与扩展协议（psql 的 `\bind`）相同；对照的 READ COMMITTED 看得到。审查 B 经生产的 `postgres.NewPool` 与 `TxManager.WithinSnapshot` 复核了冷的连接（Parse）与预热过的连接（缓存的语句，Bind），结果相同（`m12/p3-review-b/snapshot-probe.log`）。

由此：

- 快照要从第一条属于答复的读起（P3-10 逐个用例写出是哪一条）：在它之前、快照之外的读（今天的 `FindChangeset`、`Find`）与快照里的读可以看到不同的时刻。快照之外的只能是不进答复的读（日志的授权、带 `page_id` 时的 `NodesNow`）。
- 授权跟着进快照无害：它们也是读，看到的与历史同一时刻。
- 3.8.3 的整个程序的测试挡在 `page_revisions` 上：快照的第一条读不碰它，所以挡住的是快照里的第二条或更后的读。"快照晚了一步才开始"（S1-24、S1-29：`FindChangeset` 或 `Find` 在快照之外）与"`alivePaths` 在快照之后"（S1-30：SQL 写的 R 不碰 `nodes`）在这个测试里看不出：晚开始的快照仍在等锁之前冻结。它们由单元的测试挡（审查 B 的 M4）。

设计时曾以为"等锁的第一条语句在拿到锁之后才取快照"，由此要求"快照的第一条语句必须是历史的读"；实测否定了它，要求改为上面的第一点。审查不必再核对这一条。

#### 3.8.3 测试

- **单元**（`page/app`）：`HistoryReads` 的替身多一个 `Snapshots`（`fakeSnapshots`，`search_test.go:119-125` 已有，同在 `app_test` 包）。新的 `TestTheLogAndAChangesetReadInOneSnapshot`：替身的 `WithinSnapshot` 在 ctx 里放一个标记；替身的每个端口，`History`、`Names`、`Paths`（`RootByKey`、`ChildByKey`、`PathsOf`）、`Notebooks`（`StateOf`、`WorkspaceOf`）与 `Auth`，每次调用都记下方法名与 ctx 里的标记。`Execute`（带与不带 `page_id`）、`AtPath`（带与不带路径）、`GetChangeset.Execute`、`InNotebook` 各一格，断言：从这个用例的第一条读（P3-10 写的那一条）起，每一次端口调用都带同一个标记；在它之前的都不带；`WithinSnapshot` 恰好一次。漏记一个端口、或快照晚开始、早结束，都有一格红（S1-20–S1-24、S1-29、S1-30；审查 B 的 M4）。S1 实施：替身的 `historyReads()` 带 `Snapshots`，已有的写死的调用表加 `WithinSnapshot` 与 ` in tx`，否定断言改用 `calledAtAll`，替身的快照 `oneSnapshot` 拒绝嵌套（3.21 第 5 条）；`TestVirtualRead/no_nested` 改数这次读多出的快照（第 6 条）。
- **整个程序的交错**（新的 `bootstrap/history_snapshot_test.go`，`TestTheLogReadsOneMomentAcrossARevert`，13.4 第 4 条），照 `TestVirtualRead` 的 `rename_snapshot`（`bootstrap/interleavings_virtual_test.go:128-164`）：
  1. 每一格各种一本笔记本：建页 `P`，改名为 `Q`（变更集 C）。各用自己的 C 与 R：共用一个 C 时，前一格的 R 让后一格的"`reverted_by` 为空"不成立（审查 B 的 N1）。
  2. holder 事务 `LOCK TABLE page_revisions IN ACCESS EXCLUSIVE MODE`。
  3. 在 goroutine 里发读；`pgtest.WaitForTableLockWaits(t, tm.pool, "page_revisions", 1, interleavingWait)` 等到它停在第二条读上。
  4. 以 SQL 写一条撤销 C 的变更集 R，提交，照 `bootstrap/background_cost_test.go:203-205` 的 `revertOfAttachments`：列是 `id, notebook_id, kind='revert', reverts_id, client, created_by_id, created_at, updated_at`，恰好是全部没有默认值的必填列；`changesets` 上没有触发器，`reverts_id` 没有外键（审查 B 的 N2 核对过）。它不碰 `page_revisions`。
  5. holder 回滚，等读答复。
  6. 断言：答复里 C 没有被撤销（`reverted_by` 为空），整本的日志里也没有 R。之后再读一次：C 被 R 撤销，整本的日志里 R 在（对照：SQL 写的那一行读得到）。
  - 五格各一次：REST 的 `listChangesets`、REST 的 `getChangeset`（C）、MCP 的 `log`、MCP 的 `log {path: "Q.md"}`（`AtPath` 带路径的一支）；`changeset` 工具（`InNotebook`）。
  - 改之前的代码上这五格都红，原因分两种（审查 B 的 N5）：
    - 三个列日志的格（REST 的 `listChangesets`、MCP 的 `log` 两种）：`ChangesetFacts`（或带路径时的 `ListChangesetsOfPage`）在锁放开之后另取时刻，答复里 C 被 R 撤销；整本的日志里却没有 R（`ListChangesets` 在 R 之前读的）。这正是 H5 说的不一致。
    - 两个详情的格（REST 的 `getChangeset`、`changeset` 工具）：改之前的答复等于 R 之后一次一致的读（R 只改它自己那一行）。它们红，是因为测试钉住了快照的时刻：从 `FindChangeset` 起，早于 R。这是本文定的语义（答复是它第一条读那一刻的），不是 H5 的不一致。
  - 去掉四个用例中任一处的快照，对应的格红（S1-20–S1-23）。
  - 用时：五次读各等一次锁，约 1–2 秒（推断）。S1 实测 1.46 秒，竞态 2.39 秒（10.1）。

### 3.9 S1：每个凭证与每个账户的事件流数

#### 3.9.1 规则

- 每个凭证至多 `MaxCredentialStreams = 4` 条。凭证已有 4 条时，新开的那条顶掉它自己最老的一条（`reset`，原因 `superseded`），账户的总数不变，不看账户的上限。
- 凭证不到 4 条、要净增一条时，才看账户：已有 `MaxAccountStreams = 64` 条就答 429 `rate_limited`，`Retry-After` 30 秒。
- 凭证是 `Actor` 的 `SessionID`、`APITokenID` 或 `JobID`（恰有其一，`shared/actor.go:15-23`），连同种类（`session`、`pat`、`job`）。同一浏览器的几个标签页共用一个登录会话，网页一个浏览器只开一条（`W/events/hub.ts`），所以 4 条只在睡眠留下的半开连接、几个配置文件或照契约写的脚本时才碰到。
- 两个上限都是常量（00 号文档 4.6.5、5.2）。

#### 3.9.2 改动

```go
// events/app/hub.go

// MaxCredentialStreams is how many streams a credential holds open: the
// fifth's opening resets the credential's oldest, superseded, its
// account's count unchanged (M12 design 4.6.5). A browser opens one; a
// laptop that slept leaves a half-open one its new tab takes the place of.
const MaxCredentialStreams = 4

// MaxAccountStreams is how many streams an account holds open, over all
// its credentials: an opening that would make one more is refused,
// domain.ErrTooManyStreams. A credential at its own bound takes the place
// of its oldest and is never refused.
const MaxAccountStreams = 64

// credential is the one an actor holds: a sign-in's session, a personal
// access token or a background job, by kind and id.
type credential struct {
	kind string
	id   uuid.UUID
}

func credentialOf(a shared.Actor) credential

type Hub struct {
	logger    *slog.Logger
	mu        sync.Mutex
	streams   map[*Stream]struct{}
	held      map[credential][]*Stream // each credential's streams counted, oldest first
	accounts  map[uuid.UUID]int        // each account's streams counted
	listening bool
}

// open registers a stream of the actor (M12 design 4.6.5): a credential
// at MaxCredentialStreams has its oldest reset, superseded, no longer
// counted; else an account at MaxAccountStreams is refused,
// domain.ErrTooManyStreams; ErrNotReady when the hub is not listening.
func (h *Hub) open(a shared.Actor) (*Stream, error)
```

- `Stream`（`stream.go:13-25`）加 `credential credential`；`close`（`hub.go:111-116`）从 `held` 里去掉它（按指针，还在时）并把账户的数减一；凭证的切片空了就 `delete` 这一项，账户的数到 0 也 `delete`，两张表不随进程的寿命涨（审查 B 的 N10）。
- 顶掉的那一条：`held` 里先去掉、账户减一，再 `resetFor(domain.ResetSuperseded)`；它的处理器照常先写出已送到的事件、再写 `reset` 帧（同 `access`，`handler.go:96-100`），关掉时 `close` 发现它已不在 `held` 里，不再减。S1 修复：减的是被顶掉那一条自己的账户（`oldest.userID`，同 `close`），两处共用 `uncount`（减一、到 0 就删）：凭证跨了账户时数仍对、表不留 0 项（决定 484，3.21 第 12 条）。别的原因重置的流照旧算到它的处理器结束为止：客户端在读时立刻结束，不读时至多一个心跳（P3-11；审查 B 的 M5）。
- 锁的次序照旧：先 `h.mu`，再 `s.mu`（`ResetAll` 已是这样，`hub.go:89-95`）。
- `OpenStream.Execute`（`open_stream.go:30-46`）调 `o.hub.open(actor)`。
- `events/domain/frame.go`：`ResetSuperseded ResetReason = "superseded" // the same credential opened its fifth stream: this, its oldest, ends`。
- `events/domain/errors.go`：

```go
// AccountStreamsRetry is the Retry-After of a stream refused at its
// account's bound: the web's longest backoff.
const AccountStreamsRetry = 30 * time.Second

// ErrTooManyStreams is a stream refused at its account's bound: 429
// rate_limited (M12 design 4.6.5).
var ErrTooManyStreams = &shared.Error{Kind: shared.KindRateLimited, Code: shared.CodeRateLimited,
	Detail: "This account has 64 event streams open: close one, or retry later.", RetryDelay: AccountStreamsRetry}
```

  `Detail` 里的 64 由一个测试与 `MaxAccountStreams` 对证（常量在 app、文字在 domain）。
- **日志**：顶掉时 `debug` 一行 `event stream superseded`，带 `user_id`、`credential`（种类，不带 id 与令牌，13.1 第 10 条）与 `streams`（4）；拒绝时 `debug` 一行 `event streams of the account at their bound`，带 `user_id` 与 `streams`（64）。访问日志另有 429 的 `info`（同限流，`limit.go:38-41`）。
- 网页不改代码、不加测试：`reset` 的任何原因都立刻重连（`hub.ts:198-226`）；打开失败时照 `Retry-After` 等，`ApiError` 对任何状态码都取它（`W/services/api.ts:61`、`:70`；`hub.ts:220-224`、`:269-273`）。S1 修复只改了 `hub.ts`、`event.service.ts` 的注释（决定 485，3.21 第 14 条）。

#### 3.9.3 测试

- **hub 的单元**（`events/app/hub_test.go`，`openAs`（`:61`）改为可指定凭证；S1 实施在新文件 `hub_bounds_test.go`，`openAs` 不改，两张表的长度经 `export_test.go` 导出，3.21 第 8 条）：
  - `TestACredentialsFifthStreamSupersedesItsOldest`：同一会话开 5 条，第 1 条以 `superseded` 结束，第 2–5 条照常收事件；另一会话、另一账户的流不受影响。
  - `TestAnAccountsSixtyFifthStreamIsRefused`：一个账户 16 个凭证各 4 条，第 17 个凭证开第一条答 `ErrTooManyStreams`；关掉任一条之后能开；全部关掉之后 `held` 与 `accounts` 两张表都是空的（同包的测试读它们的长度）。
  - `TestAFullAccountsCredentialSupersedesItsOwn`：同上，账户已有 64 条时，一个已有 4 条的凭证开第 5 条，顶掉它自己最老的一条、不被拒，账户仍是 64。
  - `TestClientsThatReconnectAtOnceComeToRest`：一个账户 17 个凭证，每个客户端被 `reset` 就立刻重开；模拟按步走，至多 1,000 步。期望 17 次打开之后没有任何重置，模拟停下。按账户而不按凭证计（初稿的规则）时永远互顶，过 1,000 步就失败。另一格：同一凭证 5 个这样的客户端每一步恰好顶掉一个，账户的数一直是 4（写明它们会一直轮转，00 号文档 4.6.5）。
  - `TestASupersededStreamLeavesTheCountsAtOnce`：账户放在 63 条（15 个凭证各 4 条、一个 3 条），其中一个 4 条的凭证 A 开第 5 条、顶掉它最老的一条；被顶掉的那一条的处理器还没关（测试不调 `close`）时，一个新的凭证开第一条照常打开（账户 64）。变异"顶掉的仍算到它关掉"（S1-13）下账户已是 64，新的那条被拒。账户放在 64 时两种数法都拒，看不出区别（审查 B 的 N11）。
  - 文字里的 64 等于 `MaxAccountStreams`。
  - S1 另加：`TestTheHubForgetsWhatClosed`（关两次不多减，全关之后两张表空）、`TestTheRefusalSaysItsBound`（上一条写成一个测试）；第一次修复加 `TestEachTokenIsACredentialOfItsOwn`（每个 PAT 各是一个凭证，S1 审查 C 的 M1）与 `TestASupersededStreamLeavesItsOwnAccount`（顶掉时减被顶掉那一条的账户，被顶到 0 也删项，决定 484）。
- **处理器**（`events/adapter/http/handler_test.go`）：`TestAStreamThatCannotOpenIsAProblem`（`:505`）加一格：`Execute` 答 `ErrTooManyStreams` 时是 429、`rate_limited`、`Retry-After: 30`、200 之前，没有 `hello`。S1 实施：先看状态，答 200 时读第一帧就失败，不读到流的末尾（3.21 第 10 条）。
- **整个程序**（新的 `bootstrap/events_streams_test.go`，`TestASignInsFifthStreamSupersedesItsOldest`）：alice 以同一个访问令牌开 5 条（`openStream`，`events_stream_test.go:44`），第 1 条收到 `reset`、原因 `superseded`；之后写一页，第 2–5 条都收到 `pages`；alice 的一个 PAT 开的流照旧收到。用时约 1 秒（推断；S1 实测 1.53 秒，竞态 2.19 秒）。S1 实施用一次新的登录开这 5 条，PAT 的流开在它们之前（隔离；照上面用团队的令牌也确定地绿，3.21 第 7 条）。名字以 `TestAS` 开头，不在第二份的模式里（第二份收 `TestE…`，现有的 `TestEvery…` 在那里，00 号文档第 7 节"分份"）。

### 3.10 S2：第 302 条的 spec

S2 先写 spec（00 号文档第 7 节）。本节就是它：撤销计划是所有撤销都走的路，改动的边界在这里定清，S2 的实施照它，审查照它核对。

**S2 的文件**（审查 A 的 N6）：

| 文件 | 改动 |
|---|---|
| `page/domain/revert.go`、`revert_part.go`、`revert_facts.go` | 规则与五处传 `Nodes`（3.10.3） |
| `page/domain/revert_schedule_internal_test.go` | 五处 `readReverted(…, nil)`（`:89`、`:110-111`、`:128`、`:144-145`、`:179`）照新签名补第三个参数，否则编不过；S2 实施：四处补 `nil`，`TestARestoreWaitsForTheGroupItGoesUnder` 传 `s.Nodes`、去掉手工的顶（3.22 第 2 条） |
| `page/domain/revert_test.go` | `world.state()` 按时刻读组；`TestPlanRevertOfNestedTrashRoots`；`TestRevertGroupsAndRefs` 的一格（3.10.5） |
| `page/app/jobs.go`；`page/app/revert_job_test.go` | `needed` 与它的确定的一格（3.10.6） |
| `page/app/revert_job_property_test.go`、`rollback_test.go` | 两个性质测试（3.10.7） |
| `bootstrap/upgrade_rollback_revert_test.go`；`Makefile` | nested 一格翻过来、改名（3.10.8） |
| `bootstrap/revert_batch_test.go`、`interleavings_background_stops_test.go` | S2 实施：两格照决定 304 由 409 翻过来（3.22 第 1 条） |
| `docs/v0.1/v0.1-design.md` | 5.8 第 6 步、13.1 第 2 条（第 8 节）；S2 的文档提交另给 00-M11 第 302 条加括注、给 M8/P4 与 M9/P3 被决定 304 改了的三句加后注（3.22 第 1 条，决定 481） |

#### 3.10.1 规则

**一个被这个变更集删了、现在本身是回收站根的节点，是它自己的顶。**（决定 304，00 号文档 4.6.1。）

- `topOf` 从被删的节点起往上走，每一步先看脚下的节点：现在是回收站的根、而且已删（`NodeNow.TrashRoot && DeletedAt != nil`），就停在这里；否则照旧看 `Before` 的父节点是否也被这个变更集删了。
- 因为只在"被这个变更集删了"的节点上走（起点是被删的条目，往上只走到被删的父节点，`revert.go:314-317`），规则只管这个变更集删下的节点。
- 理由是按状态比较（M8 总设计 4.8）：撤销要把变更集删下的每个节点放回它删之前的地方。它们现在分在几个回收站项里，每一项各自"恰好是这个变更集在它下面删下的"（`rootHolds`、`groupIsDeleted`）、每个节点各自"在它删下的地方"（`leftDeleted`），那么把这几项依次放回（外层先，`canRestore` 要父节点活着），得到的就是删之前的样子。今天"恢复之后整个再删"已照常撤销（`DeletedByRevert` 与 `sameState` 一支），分成几次再删是同一回事。
- 一次恢复只放回一个回收站项（1 节），所以几项是几个放回的步，调度本来就按顶逐个放回，不用改。

#### 3.10.2 认出的形状与反例

认出的形状就是"本身是回收站根、被这个变更集删的节点"，有三个来源（00 号文档 4.6.1）：

1. 回滚第一遍对计划那一遍：新文件夹收着基线页与新页；新页在先删的一遍里删成一项，文件夹在计划那一遍删。
2. `PackFreeSubtrees`（`page/domain/rollback.go:58-69`）对超过 `whole` 的自由子树自下而上地删，下一遍才删它的根；可以多层，里面的根可以挂在外层那一组的中间节点下。
3. 不是回滚的：变更集 X 一次删了一个文件夹与它下面的页（一项）；之后有人恢复了这一项，先删那页，再删文件夹。

domain 上的结果（`m12/p3-design/domain_zz_p3design_test.go`，组照根的时刻读，3.10.5；左列是 `194ca350`，右列是审查 A 的原型 `split-tops.diff`，它与本节的规则等价）：

| # | 形状 | 改之前 | 改之后 |
|---|---|---|---|
| 1 | 回滚：Made{Inner（新，先删）, Guide（基线，移出）}，再删 Made | `changed Made`、`changed Inner` | 照常撤销：`restore Made`、`restore Inner`、`place Guide`（移回） |
| 2 | `PackFreeSubtrees`：F{G{H1, H2}}，H1、H2 先各删一项，再删 F（连同 G） | `changed F`、`changed H1`、`changed H2` | `restore F`、`restore H1`、`restore H2`（H1、H2 挂在 G 下，G 是 F 那一组的中间节点） |
| 3 | 不是回滚：X 删 Made{Inner}；恢复；删 Inner；删 Made | `changed Made`、`changed Inner` | `restore Made`、`restore Inner` |
| 4 | 嵌套的组有两个成员：F{G{H{J}}}，H（连同 J）先删一项，再删 F（连同 G） | `changed F`、`changed H`、`changed J` | `restore F`、`restore H` |
| 5 | 反例：同 3，但 Made 恢复之后、再删之前在它下面新建了一页 | `changed Made`、`changed Inner` | `changed Made`（Made 那一项多了一页，`rootHolds` 不成立） |
| 6 | 反例：同 3，但 Inner 恢复之后移到别处再删 | `changed Made`、`changed Inner` | `changed Inner`（它不在 X 删下它的地方） |

S2 的 domain 表另加三个反例与"太深"一格（照规则推出；审查 A 的探针 `m12/p3-review-a/domain_zz_reviewa_test.go` 跑过，结果与期望相同）：

| # | 形状 | 改之后的期望 |
|---|---|---|
| 7 | 同 1，但 Inner 那一项已过保留期（`TrashMargin`），Made 那一项没有 | `expired Inner`：里层先删，可以先过期；它不再并进 Made，过期照它自己的时刻判（`left`，`revert.go:472-476`） |
| 8 | 同 3，但之后有人只恢复了 Made 那一项（Inner 仍在回收站） | `changed Made`（活着），Inner 本身不冲突 |
| 9a | 同 2，但 F 那一项恢复之后，在 G 下新建一页，再删 F（G 是 F 那一项的中间成员，不是回收站根，单独恢复不了；审查 A 的 N1） | `changed F`（F 那一项多了一页） |
| 9b | 同 2，但 F 那一项恢复之后原样再删 | `restore F`、`restore H1`、`restore H2` |
| 10 | 同 4，嵌套的根 H 下最深的页使外层的放回太深 | 两条 `too_deep`（F 与 H）；最深的页不在 H 下时只有 F 一条（审查 A 的 N2；S2 的表另作一格 10b，3.22 第 3 条） |

"改之后"的冲突比"改之前"少了里层那一条（5、6），是因为里层不再被并进外层：每个节点按它自己的一项判。反例的整体结论（不撤销）不变。

#### 3.10.3 改动的每一处

```go
// page/domain/revert.go

// reverted is a changeset as a revert reads it: its items and versions by
// node; every node it changed, by id; its deletions' tops, by id, the top
// of each node it deleted and each top's deleted nodes, by id; the
// versions whose bases a revert writes back, by page; and its nodes now,
// which tell a deleted node that is a trash item's root itself.
type reverted struct {
	items    map[uuid.UUID]ChangesetItem
	versions map[uuid.UUID]ChangesetVersion
	ids      []uuid.UUID
	tops     []uuid.UUID
	top      map[uuid.UUID]uuid.UUID
	members  map[uuid.UUID][]uuid.UUID
	writes   []ChangesetVersion
	now      map[uuid.UUID]NodeNow
}

// readReverted reads the changeset's items and versions against its nodes
// now, nil where only the writes are read (RevertWrites).
func readReverted(items []ChangesetItem, versions []ChangesetVersion, now map[uuid.UUID]NodeNow) reverted

// topOf is the top of the deletion of the node id the changeset deleted:
// up its parents before the changeset while the changeset deleted them,
// to the first, from the node itself, that is a trash item's root now
// (M12 design 4.6.1, decision 304). A rollback deletes, in a pass of its
// own, a new subtree under a new folder it deletes a pass later; a subtree
// restored and deleted again in two is the same: each is a trash item of
// its own, which a revert restores alone, the outer first (canRestore).
// A tree has no cycle; a defect's would end where it closes.
func (c reverted) topOf(id uuid.UUID) uuid.UUID
```

- `RevertGroups`（`revert.go:196-206`）：`readReverted(items, nil, nodes)`；注释的"each top of the changeset's deletions, a deleted node whose parent before it the changeset did not delete"加"or that is a trash item's root itself"。
- `RevertWrites`（`:212-214`）：`readReverted(items, versions, nil)`，注释写明它只读 `writes`。
- `scheduled`（`:388-…`）、`CheckRevertPart`（`revert_part.go:25-30`）：`readReverted(s.Items, s.Versions, s.Nodes)`。这两处与 `RevertGroups` 读顶，传错了结论就错（7.3 第 S2-4–S2-6 条）。
- `CheckRevertSteps`（`revert_part.go:50-…`）、`RevertActionFacts`（`revert_facts.go:23-25`）：同样传 `s.Nodes`、`state.Nodes`（P3-1）。它们不读顶，传 `nil` 结果一样，7.3 把这两条写成等价的变异（S2-7、S2-8），证据照审查 A 的 M3 分两条：
  1. 代码上的事实：`CheckRevertSteps` 走 `treeSim`、`canPlace`、`canRestore`、`restore`、`added`、`canWrite`；`RevertActionFacts` 只在 `treeSim` 里读 `c.ids`；两条链上没有一处读 `c.top`、`c.tops`、`c.members`。
  2. 会被抓到的对照，证明那一处被调到：`CheckRevertSteps` 处改成 `readReverted(nil, …)`，任何一组的放回都让它空指针 panic，连没扩开的 `TestTheJobsStepsReplayToStar` 都在头几个试验里崩（修订核对 A 在 `86565dd6` 上实测，A 的 N7），所以它只证明这一处被调到；它在嵌套的形状上跑过，由扩开的测试的 `nested`、`nestedCut` 计数大于 0 证明（每一批都经 `CheckRevertSteps`）。`RevertActionFacts` 处去掉版本，`TestAContentOnlyRevertsFactsNeedNoTree` 红，同样只证明那一处被调用（它的格只有正文）。等价的依据是第 1 条。
- `movesBack`（`:677-686`）随之把嵌套的根当作"回来的顶"，对它查深度与环。外层的顶已覆盖它的子树，嵌套的根太深时外层一定也太深，所以这只会多列出同一个原因的一条冲突，不会多拒绝；多不多那一条，看最深的页在不在嵌套的根下（3.10.2 第 10 格）。
- 批的状态（`revertPart`，`unit_revert.go:470-515`）的条目只有这一批的节点：嵌套的根与外层在同一批时，今天的 `topOf` 把它并进外层、`leftDeleted` 答 `changed`；改之后它是自己的顶。外层在更早的一批放回时，它在这一批里本来就是自己的顶（父节点不在条目里）。所以 `CheckRevertPart` 必须传 `Nodes`。

#### 3.10.4 不改的

- `groupIsDeleted`、`leftDeleted`、`rootHolds` 与调度（`newRevertSim`、`schedule`、`canRestore`、`restore`、`fallback`）。
- `CheckRevertSteps` 的模拟（整组放回，P3-2）。
- SQL：`NodesNow`（`queries/history.sql:172-197`）对每个回收站根都算 `deleted_by_revert`，嵌套的根照常有它；`TrashGroup` 按时刻读，嵌套的根不会被读进外层。
- M11 的"一个任务一个变更集"、任务答复的 `changeset_id`、契约、码。放弃照旧不要求输入名称（00 号文档 4.6.1）。
- 回滚自己合成的计划没有删除的条目（M11/P1 设计 3.18），不走 `topOf`。
- 不去造"后台恢复与后台撤销里的同一形状"（00 号文档 4.6.1"不去造的"）。

#### 3.10.5 domain 的测试

- **替身**（P3-3）：`world.state()`（`revert_test.go:104-142`）读组时只收与根同一时刻删下的成员，同 `TrashChildrenOfAll` 与 app 的 `fakeStore.group`。改法：`world.subtree` 加一个只给读组用的变体，或在 `state()` 里照根的 `DeletedAt` 过滤（实验里用的后一种）。先在没改的树上跑 `go test ./internal/modules/page/domain/`，现有的格照过（它们没有不同时刻的嵌套）。
- **新的表** `TestPlanRevertOfNestedTrashRoots`（`revert_test.go`）：3.10.2 的十一格（1–8、9a、9b、10）；S2 另加 10b，共十二格（3.22 第 3 条）。撤销得了的格写死 `w.ops(plan)` 的期望（实验的结果）；冲突的格写死 `w.conflicts(plan)`。每一格另断言 `RevertGroups` 交回的根与时刻（嵌套的根在它自己的时刻）。
- `TestRevertGroupsAndRefs`（`:955`）加一格：一个嵌套的根，`RevertGroups` 名下两个根、各在自己的时刻；`RevertRefs` 对两组里的页各读它们在回收站里的版本。
- 现有的 `TestPlanRevertConflicts`（`:454-695`）的各格照过：它们的"恢复之后再删"都是整个再删（实验时 page 模块的短测试与 `Revert|Rollback|Schedule|Trash|Restore` 的非短测试在原型上全过，审查 A）。S2 照实跑一遍，有翻过来的格照实列出。S2：page 模块里没有一格翻过来；组合根里翻过来两格（3.22 第 1 条）。

#### 3.10.6 后台撤销的 `needed`

**它真的出错。** 实验（`m12/p3-design/app_zz_p3design_test.go`，在原型上，`needed` 不改）：

- 形状：G{K{x, y}, P1{z{zz}}}。变更集把 x、y 移到根下，单独删 z（连同 zz，一项，早一分钟），删 G，在根下新建一个 G，把 y 移进去。这是 `folder()` 的形状（`revert_job_property_test.go:601-644`）加一个先单独删的 z。
- 计划的步：`[aside y, delete G', restore G g0.0, restore K g0.1, restore z g1.0, place x, place y, restore P1 g0.2, restore zz g1.1]`。从 `aside y` 到 `place y` 是一段"让开"；G 的放回在段里，`needed` 只看段内的放置（x、y 放到 K 下），只放回 G、K，P1 排到段尾之后；z 的放回在段里，它的父节点 P1 还在回收站里。
- 测试的模型：第 4 步 "a restore under a parent gone"。
- 核对的替身上，批切在段尾（`[0,7)`、`[7,9)`）：第一批不报错，替身记下 "after a restore of z, z under P1, not alive"（之后两次放置后同样），即两批之间笔记本里有一个活着的页在一个已删的父节点下；第二批之后又好了。整个一批跑时没有问题（一批里一组的成员在它的第一步一次放回）。
- 批的核对不报：`CheckRevertSteps` 的模拟在 G 的根那一步就把整组放回（`revert_part.go:79-84`），它看到 P1 活着。`RestoreNodes` 的 SQL 不查父节点是否活着（`nodes.sql:249-262`）。由此推断真库也不挡、两批之间会留下活着的页挂在已删的父节点下，没在 PG 上跑：`applyStep` 的守卫与参与者（链接索引等）没有追（审查 A；修订核对 A 的 N10 自己更正）。
- 生产里走得到（审查 A 沿操作推出，没有跑）：X 一次删了 G（含 P1、z），新建 G′，把 y 移进去；之后有人把 G′ 改名，恢复 G 那一项，先删 z、再删 G，再把 G′ 改回 G。撤销 X 就是这个形状。
- 审查 A 在扩开的性质测试的原型上复现：`-short` 400 次里不改时失败 17 次，都是 "a restore under a parent gone"；改后 0 次。

**改法**（P3-2）：

```go
// page/app/jobs.go

// needed is how many of a group's members, in its pre-order, its restore
// within a segment of steps aside brings back there (M9/P3 design
// 3.22.2): up to the last one a later place or restore of the segment
// puts a node under; the group's root at least. A restore under a member
// of another group is of a trash item's root the changeset deleted, its
// own top (M12 design 4.6.1, decision 304), whose parent must be back
// first, as a place's. No other step needs one there: a member's write
// back follows its own restore, and only a deletion waits for a write back
// (review A-I1); a step aside's parent is alive (review A-N5, C-N1).
func needed(later []domain.RevertOp, nodes []domain.GroupNode) int
```

循环的条件 `op.Kind != domain.RevertPlace` 改为 `op.Kind != domain.RevertPlace && op.Kind != domain.RevertRestore`。改了之后实验的步是 `[…, restore G g0.0, restore K g0.1, restore P1 g0.2, restore z g1.0, place x, place y, restore zz g1.1]`，模型、批与替身的结局都对。

**确定的一格**（新，`page/app/revert_job_test.go`，`TestARestoreUnderAMemberCutShortInASegmentBringsItBack`）：照实验的形状与装载（每一项在它自己的时刻：3.10.7 的分时刻的装载与 `delApart` 在这一格的提交里加，6.2 第 3 个提交；审查 A 的 N3）；断言计划里 P1 的放回在 z 的放回之前、都在段里；批切在段尾跑完，核对的替身没有记下任何违例，笔记本等于 S*。段尾按计划的 `Joined` 找，不写死下标：改了 `needed` 之后计划多一步，原来的 `[0,7)`、`[7,9)` 会切在段中间，第二批答人为的冲突（审查 A 量过；按改过的计划是 `[0,8)`、`[8,9)`）。S2 另在 G 的 P1 之后加成员 Q，钉住"只放回到那个成员"：期望的步末尾是 `place y, restore Q, restore zz`，批是 `[0,8)`、`[8,10)`（3.22 第 11 条；S2 审查 A 的 M1）。

#### 3.10.7 两个性质测试怎样扩开

次数不变：两个都是 2,000 次、`-short` 400 次（`TestTheJobsStepsReplayToStar` 的 2,000 对 400，审查 A 的 M8）。期望由各自的模型算出，不读被测的计划（05-lessons 第 1 节）。每种新形状数"它真的出现"的次数，不数"规则触发"的次数；下限照 `TestTheJobsStepsReplayToStar` 的约定定（`:88-93`）：在别的种子基上取最少的次数的三分之二，2,000 次取 20 个基（0、100,000、…、1,900,000），400 次取 40 个基；`-short` 下每个下限至少 3，不够时调生成器里那种形状的概率，不加次数。

**`TestTheJobsStepsReplayToStar`**（`page/app/revert_job_property_test.go`）：

- **分时刻的装载**：`jobModel` 加 `at map[uuid.UUID]time.Time`（回收站项的根到它的时刻，没有的是 `p.at`）。`load`（`:699-740`）照每个节点所在那一项的时刻写 `DeletedAt`，每一项照它的时刻读组、记计数；`modelOf`（`:757-787`）照每个根的 `DeletedAt` 读回，填上 `at`。不改的话，同一时刻的嵌套的根会被读进外层的组（替身照时刻与父节点读组），这是真库里不会有的状态。
- **单独删**（来源 1、2）：`jobEdit` 加 `apart map[uuid.UUID]bool` 与 `delApart(x)`：同 `del`（`:426-458`），但这一项记为单独的；之后的 `del` 不吞并单独的项（`absorbed` 跳过它们）。这模拟一个变更集由几个单元在不同的时刻写下（回滚的几遍）。`done`（`:461-474`）按删除的次序给单独的项各一个更早的时刻（第 i 个是 `p.at` 减 n−i 秒），别的项在 `p.at`。`random`（`:476-586`）的第 5 种操作以三分之一的概率改为 `delApart`。
- 来源 3（恢复之后分两次再删）不另造：对计划与各批读到的东西，它与 `delApart` 造出的状态相同（里层一项先于外层一项、条目不变），只有时刻相对变更集的先后不同，`leftDeleted`、`RevertGroups` 与调度都不看这个先后；domain 表的第 3、5、6、8 格钉住它（审查 A 的过度设计；初稿的 `redeleted` 删掉）。
- **`needed` 的形状**：`folder`（`:601-644`）以二分之一的概率在 T0 里给 G 的某个在 K 之后（按前序）的页加一个子页 z（有时 z 下再加一页），在删 G 之前 `delApart(z)`。3.10.6 的形状由此出现。S2 另：造 z 时以二分之一的概率在 z 下另加一页 w，先单独删 w、再单独删 z（三层，z{zz} 不拆；决定 480，3.22 第 12 条）。
- **数的几种**（`jobCounts` 加四个，`t.Logf` 与下限表各加四行）：
  - `nested`：计划放回的某一组，它的根在 S* 里的父节点是另一组的成员；
  - `nestedMid`：同上，且那个成员不是另一组的根（来源 2 的"挂在中间节点下"）；
  - `nestedGroup`：同 `nested`，且这一组有两个以上的成员（"`topOf` 只看起点"只在这里看得见，3.10.2 第 4 格）；
  - `nestedCut`：嵌套的根的放回与它父节点那一组的根在同一段"让开"里，父节点在它那一组前序里的位置不在"段内的放置单独要的成员"之内（`needed` 的形状）。这样定义与改法无关：读成"被截到段后的成员"，改了 `needed` 之后永不成立；读成"被截短的那一组里的任一成员"，又把 S2-3 显不出来的也算进去（审查 A 的 N5）。
  - `nestedDeep`（S2 加，决定 480）：一组放回到另一组的成员下，而那一组本身也放回到第三组的成员下（多层的嵌套）。
- **下限**（审查 A 在原型上量过；实施时照约定再量，写进第 10 节）：

  | 计数 | 40 个基 × 400 次的最少 | 20 个基 × 2,000 次的最少 |
  |---|---|---|
  | `nested` | 25 | 172 |
  | `nestedMid` | 22 | 162 |
  | `nestedGroup` | 11 | 73 |
  | `nestedCut` | 9 | 87 |

  照三分之二取整，`-short` 下都在 3 以上。原有的 15 个下限在 60 组里都没报 idle（`m12/p3-review-a/short40.txt`、`full20.txt`）。S2 照约定量出，20 个下限全部重算（决定 479，3.22 第 10 条）；加 w 之后 `nestedCut` 的最少次数是 11、88，`nestedDeep` 是 9、72（10.2 的表）。
- 原有的 `movegone` 与注入照旧在 `post` 上做，它们与新形状叠加的组合不另数。

**`TestTheRollbacksStepsReplayToTheBaseline`**（`page/app/rollback_test.go:560-621`）：

- `p.agent(…)` 之后记下 agent 留下的状态 `tidied`：每个活着的节点的位置、名称、次序与页的正文的哈希。
- 回滚 `Done` 之后撤销回滚的变更集：以 `f.store.recorded` 里这个变更集的条目与版本建它（`f.recorded`（`revert_job_property_test.go:903-929`）加一个变更集种类的参数，回滚的是 `domain.ChangesetRollback`、没有 `RevertsID`；S2 另加 `recordedAs(kind, id, reverted)`、`recorded` 调它，3.22 第 4 条）。种子是偶数走请求的撤销（`f.revert(id, nil, 0)`，`revert_test.go:20-28`；S2 经核对的替身 `checkedWriter` 建 `RevertChangeset`，3.22 第 5 条），奇数走后台（`w.PlanRevert` 与 `f.runPass`，批照 `cutsBy` 随机切）；trial 另授 `domain.ActionRevert`（P3-4）。S2 另：撤销之前笔记本设成没有进行中的升级，同回滚结束之后的生产；agent 删子树时先读整棵再删（3.22 第 6、7 条）。
- 撤销成功，之后活着的节点恰是 `tidied` 的那些，各在它的位置、名称与次序，页的正文的哈希相同；核对的替身没有记下违例。
- 时刻：替身的时钟每读一次进一微秒（`tickingClock`），每一遍、每个单元的删除各在自己的时刻，同真库；不用另改。
- 只数 `nested`（回滚的变更集在撤销时有一个嵌套的根；S2 另数请求与后台两条路，各有下限，3.22 第 8 条），下限照上面的约定（审查 A 的 8 个基每 400 次 14–30 次；实施时量 40 个基再定）。挂在中间节点下的与两个成员以上的嵌套组在这里每 400 次只有 0–3 次，调 `parentFor`（`:415`）的深度也不多，不数：它们交给 `TestTheJobsStepsReplayToStar` 与 domain 表的第 2、4 格（审查 A 的 M2 的甲）。

**变异能不能显出来**：

- 去掉新规则：两个测试都在嵌套的形状上答 `page.revert_conflict`（`PlanRevert` 的冲突）。
- "`topOf` 只看起点"：只在 `TestTheJobsStepsReplayToStar` 的 `nestedGroup` 试验里显出来（3.10.2 第 4 格；原型每 400 次红 17–18 次），所以它的 `nestedGroup` 下限是证据。回滚的那个测试也偶尔抓到（每 400 次 0–3 次），不作数。
- `needed` 不看段内的放回：只在 `nestedCut` 的试验里，模型的第 4 步失败。
- 每一条先在没改的树上跑同一个命令，看到它绿（7.3）。

#### 3.10.8 整个程序：nested 一格翻过来

`bootstrap/upgrade_rollback_revert_test.go`：

- 测试改名为 `TestRevertingARollbackPutsBackWhatTheAgentLeft`，文件头的说明照新的结论改写（回滚的变更集照常撤销；M11 第 302 条由 M12/P3/S2 修好）。
- `flat` 一格不变。
- `nested` 拆成两格，各一本笔记本（同审查 A 的 `bootstrap_zz_m12review_test.go`，`m12/review-a/prototype/`）：`nested, request`：请求的撤销答 200、`changed`，笔记本等于 agent 留下的样子，撤销的变更集一个；`nested, background`：后台的撤销 `succeeded`、`restored` 4、`put_back` 1（审查 A 的原型的数），笔记本等于 agent 留下的样子，撤销的变更集一个。两格在同一个提交里与 domain 的改动一起翻过来（6.2）。
- 末尾的 `checkPages`、`checkLinks`、`checkBackground` 照旧。
- S2 另：两格 nested 在撤销之前核对回收站里 Inner 与 Made 各是一项、Inner 在 Made 下、删得早于 Made（3.22 第 13 条）。
- `Makefile`：`BOOTSTRAP_SEVENTH`（`:321`）与 `BOOTSTRAP_EIGHTH`（`:365`）里的旧名字都换成新名字；`:349` 一带的注释与第八份的说明（`:402`）随之。漏了的话新名字以 `TestR` 开头，落进补集的第四份（00 号文档 4.6.1）；3.19 的脚本核对。

#### 3.10.9 代价

- 撤销计划多读几组（每个嵌套的根一组），只在有嵌套的根时。`MaxRevertSteps` 照旧数放回的步。
- `race` 一份：两个性质测试在组合根之外、不带 `-short`（`Makefile:367-368`）。审查 A 经 `heavy.sh` 量了两轮：`TestTheJobsStepsReplayToStar` 改前 13.79、13.86 秒，原型 14.03、14.09 秒（多约 0.2 秒）；`TestTheRollbacksStepsReplayToTheBaseline` 1.83、1.85 秒，回滚加撤销的探针 2,000 次 3.16 秒，推断扩开之后多约 1.3 秒。S2 照 7.5 再量。
- 组合根：第八份的那个测试多一次回滚与撤销，推断多 2–4 秒。
- S2 实测（10.2）：竞态下后台撤销的性质测试多 0.34–0.40 秒（w 与 `nestedDeep` 在内），回滚的多约 1.5 秒；整个程序的那一格竞态多 0.1 秒。

### 3.11 S3 的文件

| 文件 | 改动 |
|---|---|
| 新的 `transfer/adapter/archive/names.go`、`names_test.go`、`names_fuzz_internal_test.go`；`archive.go`（`Entries`）；S3 实施另有新的 `names_internal_test.go`（差分，3.23 第 5 条） | 条目名的解读（3.12） |
| `transfer/domain/entry.go`、`report.go`；`entry_test.go` | `RawEntry.From`、`Sorted.Read`、两种新原因（3.12） |
| `transfer/app/import.go`；`import_test.go`、`import_fakes_test.go`（S3 实施：替身改在 `fakes_test.go`，`import_fakes_test.go` 不改，3.23 第 3、4 条） | 解读过的名字在 `finish` 进报告（3.12） |
| `transfer/domain/errors.go`；`transfer/app/start.go`、`start_import.go`、`start_background.go`、`uploads.go`、`ports.go`；`transfer/adapter/postgres/queries/jobs.sql`、生成的 `gen/jobs.sql.go`、`store.go`；各层的测试与替身（`app/fakes_test.go`、`adapter/http/fakes_test.go`）；S3 实施另改 `transfer/app/rollbacks.go` 的注释（回滚算进去、从不因此被拒） | 按人的上限（3.13） |
| `transfer/adapter/river/jobs.go`；`transfer/app/maintain.go`（`AtStart`、`lost`、`Run` 的注释）；`transfer/rescue_test.go`、`transfer/adapter/river/jobs_test.go`、`transfer/app/requests_test.go`（`TestTheRescue`）；S3 实施另改 `transfer/contributors_test.go`（`root.module`，3.23 第 10 条） | 启动时的收拾（3.14） |
| `bootstrap/transfer_jobs_test.go`（`seedJobs` 与 `TestServeFailsTheJobsLeftRunning`、`TestTheSweepDeletesTheArchivesNoJobKeeps` 的种子）、`permission_matrix_seeded_test.go`、`upgrade_fixtures_test.go`（`rollingBackIn`）、`transfer_test.go`（`TestTheQueuesLimitIsTheConfigured`）、`transfer_import_test.go`（`TestOneImportAtOnceThroughServe`）；写 River 行的两条 SQL 常量 `riverHoldsSQL`、`riverTakenSQL`（3.14.3）；修复第 1 轮加 `riverHeldJobSQL`（排队行与 River 行一条语句，3.23 第 13 条）；`permission_matrix_transfer_test.go` 的注释照改 | 种子带上 River 持有的行（3.14.3） |
| `api/modules/transfer.yaml`；`make gen` 的生成物 | 3.15 |
| 新的 `bootstrap/transfer_names_test.go`、`transfer_person_test.go` | 两个整个程序的测试（3.12、3.13）；T4 并进 `TestServeFailsTheJobsLeftRunning` |
| `W/app/problem-messages.ts`、`W/i18n/messages/{en,zh-CN}.ts`（含 `transfer.importCut`）、`W/pages/notebook/transfer-report.tsx`、`trash-row.tsx`、`W/test/jobs-server.ts`；测试 `transfer-page.test.tsx`、`page-menu.test.tsx`、`transfer-import.test.tsx`、`trash-background.test.tsx`、`revert-background.test.tsx`、`transfer-report` 的测试（`transfer-report.test.ts`）；S3 实施另有决定 482 的 `transfer-background.test.tsx`（3.23 第 2 条） | 第 4 节 |
| `Makefile`；`README.md:192`、`:193`（S3 实施：随 Step 的文档提交，决定 531） | 3.19；第 8 节 |

### 3.12 S3：名称不是 UTF-8 的导入条目（T3）

#### 3.12.1 规则

对每个条目的名字（中央目录里的原始字节）：

1. 有 UTF-8 的标志（`Flags&0x800 != 0`）：照原样，合法的照常导入，不合法的 `Classify` 照旧以 `name_not_utf8` 跳过。读 `Flags`，不读 `NonUTF8`：名字不合法时 `archive/zip` 不管标志就把 `NonUTF8` 设为真（Go 1.27.4 `archive/zip/reader.go:391-406`；00 号文档 4.6.6）。
2. 没有标志、有一个有效的 Unicode Path 字段：取它的名字，来源 `unicode_path`（与原名逐字节相同时照原样，不记）。有效是：字段 ID 0x7075，长度够，版本 1，其后 4 字节的 CRC-32（IEEE）等于名字原始字节的 CRC-32，余下的是非空的合法 UTF-8（APPNOTE 4.6.9：CRC 对不上说明别的工具改过名字、留下了旧的字段，要忽略）。只读中央目录的扩展字段（`f.Extra`），同 `archive/zip` 读的那一份。原名是 ASCII 的也读：把写不出的字换成 `?` 的工具正是靠这个字段留下真名。一个条目有几个 0x7075 时只看第一个，它无效就当没有（S3 实施，3.23 第 6 条）。
3. 余下的没有标志的名字里，不是合法 UTF-8 的是 GBK 的**候选**。**整包**：有候选、而且每个候选按 GBK 解出来都没有 U+FFFD，这个包才算 GBK 的包；只要有一个候选解出 U+FFFD，没有一个名字按 GBK 读（候选随后都跳过）。`golang.org/x/text` 的 GBK 解码器从不报错，非法的字节写成 U+FFFD，所以"解得开"只能这样定义（审查 A 的 M4）。第 1 条的名字不算候选，不影响整包的判断。
4. GBK 的包里，第 3 条的每个名字，含非 ASCII 字节、按 GBK 解出来没有 U+FFFD 的，都取 GBK 的读法，来源 `gbk`：候选之外，**也包括恰好是合法 UTF-8 的**（P3-16）。GBK 的双字节有一部分恰好是合法的 UTF-8：一级汉字里约 18% 的单字，两个字的名字约 3%（3.12.1 的计算）；常用的 `目录`（`c4 bf c2 bc`，UTF-8 读作 `Ŀ¼`）、`图片`（`cd bc c6 ac`，`ͼƬ`）都是。只改不合法的话，`图片/a.png` 进 `ͼƬ`、`图片/截图.png` 进 `图片`，同一个文件夹分成两个。解出 U+FFFD 的合法 UTF-8 名字照原样。
5. 其余的照原样。

所以：没有候选的包（macOS 的"归档实用工具"、Linux 的 `zip` 在 UTF-8 的环境里、M7 的导出），除了有效的 0x7075，一个字节也不改；有标志的名字不改；改了的 GBK 名字等于解码器对原始字节的输出、没有 U+FFFD（3.12.3 的差分）。GBK 的往返不是不变式：`A2E3` 解成 `€`、再编成 `80`，`A3A0` 解成 U+3000、再编成 `A1A1`，全部 21,992 个解得开的单双字节里只有这两对（审查 C 的 M2，`m12/p3-review-c/gbkround/`）。解读之后的名字照旧经 `Classify` 的每一项判断（越出目录、被忽略的名字、太深、重名……）。00 号文档 4.6.6 末条"解读只改原本不是合法 UTF-8 的名字"照此改写（2.1 第 13 条）。

**为什么在适配器里**：规则要 `Flags`、`Extra`、CRC 与 GBK 解码器；纯的层不许导入 `golang.org/x/text/encoding`（`archtest/rules_test.go:127-133`）。适配器把读出的名字与来源交给 domain，domain 只管记报告（P3-12）。

**文件夹按读出的名字判**：GBK 的第二个字节可以是 0x5C（`\`）：`乗` 是 `81 5C`、`俓` 是 `82 5C`（实验）。今天 `Entries`（`archive.go:304`）按原始名字结尾的 `\` 判文件夹，一个以这类字结尾、没有扩展名的文件会被当成文件夹；`pathOf` 也会把它当成分隔符。所以先解读、再用读出的名字判文件夹与 `Special`、再交给 `pathOf`。

**实验**（`x/text` v0.42.0，`m12/p3-design/evidence.txt`，`gbk_zz_test.go`、`gbk_zz2_test.go`）：

| 输入 | GBK 的读法 | U+FFFD |
|---|---|---|
| `测试资料.md` 的 GBK 字节 `b2 e2 ca d4 d7 ca c1 cf 2e 6d 64` | `测试资料.md` | 无（不是合法的 UTF-8） |
| `乗.md` 的 GBK 字节 `81 5c 2e 6d 64` | `乗.md` | 无；原始字节里有 `\` |
| Shift-JIS 的 `ソース.md`：`83 5c 81 5b 83 58 2e 6d 64` | `僜乕僗.md` | 无：被读成乱码（已知限制） |
| GB18030 四字节的 😀：`94 39 fc 36 2e 6d 64` | `�9�6.md` | 有：整包不回退 |
| `80 2e 6d 64` | `€.md` | 无（CP936 的欧元符号） |
| `66 ff 2e 6d 64`（TR4 的 `f\xff.md`） | `f�.md` | 有：TR4 照旧整包不回退、`name_not_utf8` |
| Shift-JIS 的 `テスト資料.md` | `僥僗僩帒椏.md` | 无：同审查 A 的实测 |
| `目录`、`图片` 的 GBK 字节 `c4 bf c2 bc`、`cd bc c6 ac` | `目录`、`图片` | 无；原始字节是合法的 UTF-8（`Ŀ¼`、`ͼƬ`），第 4 条 |

"一级汉字里约 18% 的单字恰好是合法 UTF-8"是对 GB2312 一级汉字（`B0A1`–`D7F9`，3,755 个）逐个判的；两个、三个、四个字的名字是随机拼的 20 万个的比例（3.3%、0.6%、0.1%）。原样写在 `m12/p3-design/evidence.txt`。

#### 3.12.2 改动

```go
// transfer/domain/entry.go

// NameSource is where an entry's name was read from, when its header's
// bytes are not UTF-8 (M12 design 4.6.6, T3): its Info-ZIP Unicode Path
// field, or the GBK reading of a zip that a Chinese Windows made.
type NameSource string

const (
	NameAsWritten       NameSource = ""
	NameFromUnicodePath NameSource = "unicode_path"
	NameFromGBK         NameSource = "gbk"
)

type RawEntry struct {
	Index     int
	Name      string     // as read, From says how
	From      NameSource
	Folder    bool       // by the name as read, or the entry's mode
	Special   bool
	Encrypted bool
	Method    uint16
}

type Sorted struct {
	Entries []ImportEntry
	Meta    int
	Skipped []Problem
	// Read are the entries, but those ignored, whose names were read from
	// elsewhere than their headers' bytes, each a problem of its source:
	// the report lists them after every other (M12 design 4.6.6).
	Read []Problem
}
```

```go
// transfer/domain/report.go（导入的问题，:100-133 那一组）

	// ProblemNameUnicodePath is an entry whose name, not UTF-8, was read
	// from its Info-ZIP Unicode Path field.
	ProblemNameUnicodePath ProblemCode = "name_unicode_path"
	// ProblemNameGBK is an entry whose name, not UTF-8, was read as GBK, as
	// a Chinese Windows writes names; a name in another encoding may read
	// wrong (M12 design 4.6.6).
	ProblemNameGBK ProblemCode = "name_gbk"
```

```go
// transfer/adapter/archive/names.go

// unicodePathID is the Info-ZIP Unicode Path extra field's (APPNOTE 4.6.9).
const unicodePathID = 0x7075

// unicodePath is the name the Unicode Path field in extra gives an entry
// whose header's name is name: version 1, the CRC-32 of name, then the
// name in UTF-8. False when there is none, or it is of another version,
// or its CRC is not name's (a tool renamed the entry and left the field),
// or its name is empty or not UTF-8.
func unicodePath(extra []byte, name string) (string, bool)

// readNames are the names of files as the import reads them, and their
// sources (M12 design 4.6.6, T3; M12/P3 design 3.12.1): one flagged UTF-8
// as written (Classify skips one that is not); else its Unicode Path
// field's, when valid; else, in a zip whose names neither flagged nor in
// UTF-8 all read as GBK with no U+FFFD, and are one or more, its GBK
// reading, when it has one with no U+FFFD (a GBK name may be valid UTF-8
// by chance: 目录 is Ŀ¼); else as written. A zip with no such name has
// no name read otherwise but by its Unicode Path fields.
func readNames(files []*zip.File) []readName

type readName struct {
	name string
	from domain.NameSource
}
```

- `Entries`（`archive.go:296-309`）先 `readNames(a.z.File)`，再以读出的名字判 `Folder` 与 `Special`，交回带 `From` 的 `RawEntry`。
- `Classify`（`entry.go:92-178`）：一个 `From` 不为空、没有被 `ignored` 滤掉的条目，往 `Read` 里加 `Problem{Path: r.Name, Code: <来源对应的码>}`；不论它之后被收下还是被跳过（被跳过的另有它的跳过的问题）。`Read` 按条目的次序；作为库的顶层文件夹、本身不导入的那一个条目也进（S3 实施，3.23 第 7 条）。来源到码是 `NameSource` 的不导出方法 `problem()`（`gochecknoglobals` 不许包级的表，3.23 第 8 条）。
- `importRun`（`transfer/app/import.go`）：记下 `sorted.Read`；`finish`（`:93` 调的那一个）在写结束的报告之前，把它们逐条 `r.report.Add`，排在这次导入找到的别的问题之后；不加 `Counts.Skipped`。进程中途停掉的导入没有这些记录（P3-12）。
- app 的替身 `zipArchive`（`transfer/app/fakes_test.go:486-497`）认得来源（S3 实施，3.23 第 3 条）：替身的 `archives` 加一张 `from`（名字 → 来源），`Entries` 照它填 `From`；app 的测试照旧以 zip 造导入，经 `Import.Run` 的整条路走到报告。
- 已知限制（P5/S3 的发布说明，4.13）：Shift-JIS、Big5、EUC-KR 的名字会被读成乱码的中文；GB18030 的四字节字符让整包不按 GBK 读；只有恰好是合法 UTF-8 的 GBK 名字、没有一个候选的包认不出（`ͼƬ`）；**相对今天的回归**：一个包同时有 GBK 候选与没有标志的真 UTF-8 名字时，今天真 UTF-8 名字导入得对，之后会被读成 GBK（P3-16），报告里逐个列成 `name_gbk`。

#### 3.12.3 测试

- **适配器的表**（`names_test.go`，`TestTheImportReadsNamesNotInUTF8`），每格一个 zip，由 `archive/zip` 的 Writer 写。Writer 给合法 UTF-8、又要多字节的名字自动设上标志（Go 1.27.4 `archive/zip/writer.go:313-320`；`detectUTF8` 对 0x5C、0x7E 也算要），所以凡要"没有标志"的条目都给 `FileHeader.NonUTF8: true`；要标志时给 `Flags: 0x800`。帮手 `zipEntry`（`import_fakes_test.go:28-70`）加一个 `noFlag` 字段，整个程序的 `vaultZip` 同样处理（S3 实施：表直接交 `*zip.FileHeader`，整个程序的测试另有 `unflaggedZip`，`zipEntry`、`vaultZip` 不动，3.23 第 4 条）。每一格先断言读回的 `Flags&0x800` 是这一格要的，作为对照（审查 C 的 I1，`m12/p3-review-c/flagprobe/`）：
  1. 带有效 0x7075 的 GBK 名字：取字段的名字，`unicode_path`；
  2. 0x7075 的 CRC 对不上：不取，落到 GBK；
  3. 0x7075 的版本是 2：不取；
  4. ASCII 的 `??.md` 带有效 0x7075（`测试.md`）：取字段的名字；合法 UTF-8 的名字带一个与它相同的有效 0x7075：照原样，`From` 为空；
  5. 没有标志的 GBK 名字（`测试资料.md`、`目录/文件.md`）：GBK 读法，`gbk`；
  6. 同一个包里的 `图片/`、`图片/a.png`、`图片/截图.png`、`目录/`（除了 `图片/截图.png`，原始字节都恰好是合法的 UTF-8）：都读成 GBK，`a.png` 与 `截图.png` 在同一个文件夹 `图片` 下，`目录/` 是 `目录`（P3-16）；
  7. 只有 `图片/`、`图片/a.png` 的包（没有候选）：照原样 `ͼƬ`（已知限制）；
  8. 有标志却不是合法 UTF-8 的：照原样；同一个包里第 5 格的名字照样按 GBK 读（它不算候选）；有标志的 `ü.md`（合法）在 GBK 的包里也照原样；
  9. 同一个包里多一个 `f\xff.md`（0xFF 不是 GBK 的首字节，读出 U+FFFD）：包里没有一个名字按 GBK 读，第 6 格的 `图片/a.png` 也照原样；
  10. 没有标志、合法 UTF-8 的 `ü.md`（`c3 bc`）在 GBK 的包里：按 GBK 读成 `眉.md`（P3-16 的代价，钉住）；在没有候选的包里：照原样；
  11. `乗.md`（`81 5C …`）：读成 `乗.md`，是文件；`资料/乗`（`d7 ca c1 cf 2f 81 5c`，没有扩展名，最后一个字的尾字节是 0x5C）：读成 `资料/乗`，是附件、不是文件夹，`资料` 下有它；`资料/` 的 GBK 字节：文件夹。`乗.md` 的原始字节以 `.md` 结尾，抓不到 S3-8，`资料/乗` 抓得到（审查 C 的 M1）；符号链接 `资料/俓`（`82 5C` 结尾）按读出的名字判，是特殊文件（修复第 1 轮，S3 审查 B 的 N1）；
  12. Shift-JIS 的 `テスト資料.md`：读成 `僥僗僩帒椏.md`（钉住已知限制：改了它的人要同时改发布说明）；
  13. GB18030 四字节：整包不按 GBK 读。
- **差分**（`TestReadNamesReadsAsGBKOnlyAZipOfGBKNames`，在内部测试 `names_internal_test.go` 里，随机生成的包，种子 1–3，每组 1,000 个包，`math/rand/v2` 的 PCG；3.23 第 5 条）：名字取自合法的 UTF-8（含非 ASCII）、GBK 字节（含恰好合法 UTF-8 的）、Shift-JIS 字节与任意字节，标志与 0x7075 随机；"没有标志"那一支设 `NonUTF8: true`。断言：有标志的名字逐字节不变；没有候选的包里，只有带有效 0x7075 的名字会变；ASCII 的名字只因 0x7075 而变；一个包里的 GBK 候选要么全读、要么全不读，全不读时没有一个名字是 `gbk`；读出的 GBK 名字等于解码器对原始字节的输出、没有 U+FFFD（不断言往返，审查 C 的 M2）；读出的 `unicode_path` 名字等于字段里的、字段的 CRC 等于原始名字的。数各种情形出现的次数，各有下限（05-lessons 第 1 节），其中两种单独数、各带下限："没有候选的包里，无标志、非 ASCII 的合法 UTF-8 名字"与"GBK 的包里，无标志、恰好合法 UTF-8 的名字"；S3-3、S3-4 各在差分上红一次（审查 C 的 I1）。13 个计数与下限在 10.3。
- **模糊**（`names_fuzz_internal_test.go`，`FuzzUnicodePathReadsOnlyAValidField`）：任意的 `extra` 与 `name` 从不 panic；交回真时，字段里的 CRC 等于 `name` 的、名字是合法的 UTF-8；种子由一个写字段的帮手造（有效、CRC 错、版本 2、截断、两个字段）。S3 实施：断言是"当且仅当第一个有效才取，取的是它的名字"（3.23 第 6 条）；修复第 1 轮加三个钉边界的种子：只有 1 字节的字段、尺寸比余下的多 1 字节的、截在 CRC 与 `测` 之后的（S3 审查 B 的 M1）。普通的 `go test` 跑种子，变异照文件头的命令手跑（13.4 第 11 条）。
- **domain**（`entry_test.go` 的 `TestClassify`，`:55`）：带 `From` 的条目进 `Read`，被忽略的（`__MACOSX/…`）不进，被跳过的（越出目录）进 `Read` 也进 `Skipped`。
- **app**（`import_test.go`）：一个有 1,100 个 `gbk` 条目、其中一个过大的导入，报告里先是 `too_large`，再是 `name_gbk`，`Truncated` 为真；`Counts.Skipped` 只数那个过大的。修复第 1 轮另加：失败的导入（`too_many_nodes`）照样列出读过的名字（S3 审查 B 的 N2）。
- **整个程序**（新的 `bootstrap/transfer_names_test.go`，`TestAnImportReadsNamesNotInUTF8`）：一个 zip，每个条目都没有标志（S3 实施：本文件的 `unflaggedZip` 给 `NonUTF8: true`）：`图片/截图.md`（GBK，不是合法 UTF-8）、`图片/a.md`（GBK 字节恰好是合法 UTF-8）、`测试资料.md`（GBK）、一页带 0x7075 的；导入之后四页的标题与位置对，`截图`、`a` 在同一个 `图片` 下（P3-16），报告依次是 `name_gbk` 与 `name_unicode_path`（照条目的次序），`to` 都是 `null`。约 1–2 秒（推断；实测 1.36 秒，10.3）。
- e2e：TR4 的 `f\xff.md`（`tr4-malicious.spec.ts:68`、`:105`）照旧是 `name_not_utf8`：0xFF 读不出，整包不回退。全量 `make e2e` 证明它没变。

### 3.13 S3：按人的队列上限（T2）

#### 3.13.1 规则

- 一个人排队与运行中的任务（每本笔记本的，`created_by_id`），加他正在上传的导入，到 `MaxPersonJobs = 5` 时，他新开的导出、导入、后台的恢复与撤销答 409 `transfer.too_many_jobs`（P3-13）。
- 在 `room` 里先判按人，再判全局（`server_busy`）。
- 与 `transfer.busy` 的先后照今天的检查（审查 C 的 I2；13.1 第 4 条）：导出是 `transfer.too_many_jobs` → `server_busy` → `transfer.busy`（`admit` 先 `room`、后 `Exporting`，`start.go:186-201`）；导入的 `Check` 是 `transfer.busy` → `transfer.too_many_jobs` → `server_busy`（先 `Uploads.claim`、`writing`，后 `room`，`start_import.go:86-115`）；后台恢复是 `transfer.busy` →（`page.source_duplicate`）→ `transfer.too_many_jobs` → `server_busy`（`claim` 在页模块的核对里，`start_background.go:68-80`、`:135-146`，`room` 在 `:182`、`:197-200`）；后台撤销是 … `transfer.busy` → `page.revert_conflict` → `page.changeset_trimmed` → `page.locked` → `transfer.too_many_jobs` → `server_busy`（`:108-128`）。导入的 `write`（`:266-272`）先 `room` 后 `writing`，是开任务时的再判一次，不改上面的次序。
- 升级的回滚算进开始它的人的计数，但从不被按人的上限拒绝（放弃与驳回要随时做得了，00 号文档 4.6.6）。
- 正在上传的导入算进开始它的人；后台的恢复与撤销在 `claim` 里占的位（0 字节，从不 `admit`）照旧不算。
- 计数与全局的计数是同一条语句、同一把队列的锁之下（导出、导入的 `write`、后台），导入的 `Check` 照旧没有锁、先读上传再读行（`start.go:203-211` 的注释照旧成立：一次 `Check` 与一次导出同时判过时，可以多出一个，下一次在锁下的开始被拒）。

#### 3.13.2 改动

```go
// transfer/domain/errors.go

// MaxPersonJobs is how many jobs one person has queued or running, with
// the imports they are uploading, over every notebook (M12 design 4.6.6,
// T2): a start past it is refused, ErrTooManyJobs; a rollback's never is,
// though it counts.
const MaxPersonJobs = 5

// ErrTooManyJobs is a start refused because its person has MaxPersonJobs
// jobs queued or running: 409 transfer.too_many_jobs. Their own jobs'
// ends, or a cancel, lift it, not a wait: no Retry-After.
var ErrTooManyJobs = shared.NewError(shared.KindConflict, "transfer.too_many_jobs",
	"You have 5 jobs queued or running: wait for one to end, or cancel one.")
```

```sql
-- name: CountActive :one
-- The jobs queued or running, of every notebook, and of them those person started (M12 design 4.6.6, T2), both
-- in one statement: the rows through transfer_jobs_state_idx, at most transfer.max_queued and the few more a start
-- outside the queue's lock lets by.
SELECT count(*)::integer AS active,
    (count(*) FILTER (WHERE created_by_id = sqlc.arg(person)))::integer AS mine
FROM transfer_jobs WHERE state IN ('queued', 'running') AND deleted_at IS NULL;
```

```go
// transfer/app/ports.go（:275-286 那一组）
	// CountActive counts the jobs queued or running, of every notebook, and
	// of them person's.
	CountActive(ctx context.Context, person uuid.UUID) (active, mine int, err error)

// transfer/app/uploads.go
type upload struct {
	person           uuid.UUID
	bytes, stored    int64
	admitted         bool
	committing       bool
	written          bool
}
func (u *Uploads) claim(id, person uuid.UUID, bytes int64) bool
// others counts the uploads admitted and not written, into notebooks but
// id, all and person's, and the bytes they have left to store; ... (as now)
func (u *Uploads) others(id, person uuid.UUID, queueLocked bool) (all, mine int, bytes int64)

// transfer/app/start.go
// room is 409 transfer.too_many_jobs when person (none for a rollback's,
// which no person's bound refuses) has MaxPersonJobs jobs queued or
// running with the uploads they have under way, but except's; then 503
// server_busy when ... (the rest as now).
func (d StartDeps) room(ctx context.Context, except, person uuid.UUID, queueLocked bool) error
```

- 五处调用都传开始它的人：导出的 `admit`（`start.go:190`，`userID`）、导入的 `Check`（`start_import.go:111`）与 `write`（`:269`）、`queueBackground` 的两次（`start_background.go:182`、`:200`）。`queueBackground` 对回滚（`job.Kind == domain.KindRollback`）传零值。
- `free`（`start.go:235-245`）照新的 `others` 取字节，行为不变。
- 全局的 503 `server_busy`（`ErrQueueFull`）照旧。

#### 3.13.3 测试

- **app**（`transfer/app/requests_test.go`、`start_import_test.go`、`start_background_test.go`、`rollbacks_test.go`）：四种开始各一格：这个人已有 5 个，答 `ErrTooManyJobs`，别人的照常开始；他有 4 个、全局满了，答 `ErrQueueFull`；他有 5 个、全局也满了，答 `ErrTooManyJobs`（先后）；他正在上传一个导入、另有 4 个任务，他的下一个答 `ErrTooManyJobs`（`Check` 与 `write` 两条路各一格）；回滚：他有 5 个时 `Rollbacks.Start` 照常开始，之后他的导出答 `ErrTooManyJobs`（回滚算进去）。另有"两样都在"的格钉住 3.13.1 的先后：导出在这个人有 5 个、又在这本笔记本里有一个导出时答 `ErrTooManyJobs`；导入、后台恢复与撤销在笔记本里有写入的任务、这个人又有 5 个时答 `transfer.busy`，撤销在 `page.locked` 与按人都在时答 `page.locked`（审查 C 的 I2）。S3 实施：app 的格是三个新测试 `TestAPersonsSixthExportIsRefused`、`TestAPersonsSixthImportIsRefused`、`TestAPersonsSixthBackgroundStartIsRefused`，回滚一格在已有的 `TestARollbackStartsInItsUpgradeAlone`；修复第 1 轮加后台开始在锁下再数一次的一格（"four of his, his fifth after the first count"：替身在锁外的计数之后给他加第 5 个，S3 审查 C 的 M1；3.23 第 9 条）。
- **适配器**（`transfer/adapter/postgres`）：`CountActive` 对两个人的排队、运行、已结束与已删除的行，交回的 `active` 与 `mine` 写死（每格的数互不相同，05-lessons 第 1 节）。
- **http**（`transfer/adapter/http`）：四个操作各有一格答 409 `transfer.too_many_jobs`（经 `CheckResponse`；`apitest.Main` 要每个声明的码都有人答过，`apitest/problems.go:161-194`），没有 `Retry-After`。
- **整个程序**（新的 `bootstrap/transfer_person_test.go`，`TestAPersonsSixthJobIsRefusedAndAnothersStarts`）：以 SQL 给 alice 在五本笔记本里各种一个排队的导出，每个带一行 River 持有的任务（3.14.3 的帮手，修复第 1 轮起与排队行写在同一条语句里，3.23 第 13 条；T4 之后不带的会与启动时的 `lost` 赛跑，审查 C 的 I3）；alice 在第六本开导出答 409 `transfer.too_many_jobs`、整句的 `detail`；bob 开导出答 202；取消 alice 的一个之后她的那一个答 202。约 1 秒（推断；实测 1.31 秒，10.3）。
- 网页的几格在第 4 节。

### 3.14 S3：启动时收拾上一个进程取到而没开始的任务（T4）

#### 3.14.1 改动（P3-14）

```go
// transfer/adapter/river/jobs.go：ExportJob、ImportJob、TrashRestoreJob、RevertJob、RollbackJob（:135-171）各加
	DiscardRunning: ExportKind, // 各自的种类
```

注释：这一种 River 记着运行中的行，在服务启动时都是上一个进程的（v0.1 一个实例，4.8）：平台在收拾的 `Start` 之前丢掉它们，收拾随即把仍在排队的行记成失败（M12 design 4.6.6, T4）。

```go
// transfer/app/maintain.go

// AtStart fails every running job: as the server starts, before it runs
// any, they were all interrupted (one server). One a request holds is
// left to Run, its heartbeat old by then. Then, as Run does, the queued
// jobs River no longer holds: the runner discarded, before this, each
// kind's jobs River held running, which the last process took and stopped
// before they started, so that their rows wait no more for River's rescue
// (M12 design 4.6.6, T4).
func (r *Rescue) AtStart(ctx context.Context) error

// lost fails the queued jobs of kind River no longer holds, logging each
// as msg (... as now).
func (r *Rescue) lost(ctx context.Context, kind domain.Kind, msg string) error
```

- `AtStart`（`maintain.go:87-94`）在 `InterruptJobs` 之后对 `domain.Kinds()` 各跑一次 `lost`，日志的话是 `queued job left by a stopped server`；`Run`（`:101-117`）照旧是 `queued job dropped by River`。`Run` 的注释（`:96-100`，"One whose process stopped as River took it River holds, running, until its rescue"）T4 之后过时，改为"one whose process stopped as River took it fails at the next start"（审查 C 的 N8）。
- 依赖的次序：模块的任务表里五种任务在收拾之前（`transfer/module.go:255-264`），平台把每个 `Job` 的丢弃排在它的 `Start` 之前、所有的 `Start` 在 River 启动之前（`platform/jobs/jobs.go:101-106`、`:169-186`）。收拾的注释写明这一点；3.14.2 有测试钉住。
- `lost` 的并发照旧成立（`maintain.go:119-122` 的注释：先读行、再读 River）：启动时 HTTP 已在服务（`bootstrap/app.go:94-107`），这时新开的任务的行与 River 的行在同一个事务里写下，要么不在读到的排队行里，要么被 River 持有。
- `README.md:193` 的那一句随之（第 8 节第 6 条）。

#### 3.14.2 测试

- **app**：`TestTheRescue`（`transfer/app/requests_test.go:339-380`）今天断言 `AtStart` 只做 `InterruptJobs`、不问行与 River（`:361-363`），T4 之后这一句红（审查 C 的 `rescue-probe.log`）。改为：`AtStart` 先 `InterruptJobs`，再照种类先读行、后问 River，失败每种被丢下的那一个，各记一行 `queued job left by a stopped server`；之后在替身里每种再加一个被丢下的，`Run` 照旧失败它们、记 `queued job dropped by River`（`Run` 那一半的断言照旧有东西可抓）。S3 实施：替身的 `InterruptJobs` 记进调用表，照调用的次序断言（3.23 第 10 条）；修复第 1 轮加末尾一格：River 的读失败时 `AtStart` 交回错误、没有任务被记失败（S3 审查 A 的 N4，3.23 第 14 条）。
- **River 适配器**（`transfer/adapter/river/jobs_test.go`）：五个构造函数各自的 `DiscardRunning` 是它的种类（同 `page/adapter/river/search_test.go:64`）。
- **模块**（`transfer/rescue_test.go`，真库与 River，`contributors_test.go:76-103` 的 `start` 以 `m.Jobs()` 起）：新的 `TestAQueuedJobRiverHeldRunningFailsAtTheStart`：写一个排队的导入的行与它的 River 行（`enqueue`，`:155-169`），把 River 行改成 `running`（同 `platform/jobs/jobs_test.go:305-317`），起 runner：行是失败 `interrupted`，River 行是 `discarded`。另一格：`m.Jobs()` 里五种任务都在收拾之前（数下标）。S3 实施：`contributors_test.go` 加 `root.module`（交回模块与它的 `Inserter`），`start` 改调它（3.23 第 10 条）。
- **整个程序**：不另起应用（审查 C 的过度设计），并进已有的 `TestServeFailsTheJobsLeftRunning`（`bootstrap/transfer_jobs_test.go:165-194`，第四份）。它的 `waiting` 拆成三种（3.14.3 的第 1 条）：
  - `waiting held`：排队，带一行 River 持有的任务（`scheduled`，一小时之后）：照旧排队；
  - `waiting dropped`：排队，没有 River 的行：失败 `interrupted`（启动时的 `lost`）；
  - `taken`：排队的导入，它的 River 行是 `running`（`riverTakenSQL`；上一个进程取到、没开始；`args` 是 `{"job_id": …}`，种类 `transfer.import`，队列 `transfer_import`，同 `TestASearchRunLeftRunningByAStoppedServerIsDiscardedAtTheStart`，`bootstrap/search_test.go:485-527`）：River 行是 `discarded`，任务失败 `interrupted`。
  - 期望的状态与失败的原因照此改写；文件头的注释改为"a queued one River holds stays queued; one it does not, or held running, fails"。名额空出由状态推出：`writing` 与 `Exporting` 只看排队与运行中的行，种子的用户登录不了（`seedJobs` 的密码是 `x`），不另开任务。
  - 多两行种子，用时不变（同一次起应用）。

#### 3.14.3 已有的测试的种子（审查 C 的 I3；主持者定，Q2）

`AtStart` 跑 `lost` 之后，以 SQL 只种排队行、不种 River 行的已有测试会在启动时（或与启动赛跑时）看到它被记成失败。审查 C 照 3.14.1 改了 `AtStart`、别的不改，复现了五处红（`m12/p3-review-c/t4-*.log`、`rescue-probe.log`），设计时又找到一处。都改为种子带一行 River 持有的任务：`bootstrap` 包里新的 SQL 常量 `riverHoldsSQL`（参数是种类、任务 id、队列），写 `river_job (kind, args, queue, max_attempts, scheduled_at, state)`，`args` 是 `{"job_id": …}`，`scheduled_at` 是一小时之后、状态 `scheduled`（照 `transfer/rescue_test.go:47` 的 `later`，River 不去跑它；`Inserter.Unfinished` 把 `scheduled` 算作持有，`platform/jobs/inserter.go:59-61`）。

```go
// riverHoldsSQL writes the River row of a queued transfer job ($2) of
// kind $1 on queue $3, scheduled an hour on: River holds it and does not
// run it, so the start's lost leaves the job queued (M12/P3 design
// 3.14.3). riverTakenSQL writes it running, as a stopped server left one
// it took and never started. Each seed runs them on its own executor.
const (
	riverHoldsSQL = `INSERT INTO river_job (kind, args, queue, max_attempts, scheduled_at, state)
		VALUES ($1, jsonb_build_object('job_id', $2::text), $3, 1, now() + interval '1 hour', 'scheduled')`
	riverTakenSQL = `INSERT INTO river_job (kind, args, queue, max_attempts, state, attempt, attempted_at)
		VALUES ($1, jsonb_build_object('job_id', $2::text), $3, 1, 'running', 1, now())`
)
```

是常量、不是收 `t` 与池的帮手（修订核对 C 的 N12）：种子的写法各有自己的执行者，`seedJobs` 与矩阵的种子经 `exec` 闭包，`rollingBackIn` 在 `pgx.BeginFunc` 里经 `tx.Exec`（要在同一个事务里写，它没有 `t`），第 6 条经 `tm.pool.Exec`。两条 SQL 照修订核对 C 的探针（`m12/p3-check1-c/ck1_patch.py` 的 `zzRiverHolds`、`zzRiverTaken`），在那里跑绿过；任务 id 同探针以字符串传（`id.String()`）。

| # | 测试 | 今天的种子 | T4 之后不改会怎样 | 改法 | 反向对照 |
|---|---|---|---|---|---|
| 1 | `TestServeFailsTheJobsLeftRunning`（第四份） | `waiting`：排队、没有 River 行，期望照旧排队（`:173`、`:182`） | 确定地红：`waiting failed`（`t4-serve-probe.log`） | 3.14.2 的三种 | `waiting held` 去掉它的 River 行，这一格红；S3-16 下 `waiting dropped` 与 `taken` 红 |
| 2 | `TestTheSweepDeletesTheArchivesNoJobKeeps`（第三份，`transfer_jobs_test.go:225-249`） | `importing`：排队的导入、没有 River 行，期望它 48 小时前的 zip 留着（`:237`、`:247`） | 红（设计时找到，修订核对 C 复现）：启动时它被记成失败，清扫照"导入已结束"删掉它的 zip（`m12/p3-check1-c/noholds-top.log`；带上 River 行绿，`seeded-top.log`） | `seededJob` 加 `held bool`，`seedJobs` 对它 `exec(riverHoldsSQL, …)`；`importing` 设 `held` | 去掉 `importing` 的 `held`，这一格红 |
| 3 | `TestPermissionMatrix` 的 `getTransferJob, another's` 与 `cancelTransferJob, another's, queued`（第四份） | `matrixJobs` 的"someone else's, queued, never enqueued"（`permission_matrix_transfer_test.go:36-49`、`permission_matrix_seeded_test.go:716-727`） | 读的格确定地红（`State:failed`）；取消的格随启动的先后偶发地红（409 `transfer.not_cancellable`）（`t4-matrix-probe.log`） | 种子里的排队导出各 `exec(riverHoldsSQL, …)`（种类 `transfer.export`，队列 `transfer_export`）；注释改为"queued, River holding it later" | 去掉 River 行，读的格红 |
| 4 | `TestUpgradePermissionMatrix` 的 `cancelTransferJob, another's rollback, queued`（第七份） | `rollingBackIn(..., "queued")`（`upgrade_fixtures_test.go:79-99`，"written as River never enqueued it"），在 `startApp` 之前 | 偶发地红（409 `transfer.not_cancellable`） | `rollingBackIn` 在同一个事务里对排队的回滚 `tx.Exec(ctx, riverHoldsSQL, …)`（种类 `transfer.rollback`，队列 `transfer_import`）；注释照改 | 不确定：这是写的格，各起一个应用，取消的请求与启动时的 `lost` 赛跑，去掉 River 行之后修订核对 C 两次都红，但不保证每次都红（C 的 N11）。确定的反向对照在第 3 条读的那一格；`rollingBackIn` 这一支只能看它偶发地红，第 10 节照实记跑了几次、红了几次 |
| 5 | `TestTheRescue`（`transfer/app`） | 断言 `AtStart` 不问行与 River | 确定地红 | 3.14.2 的 app 一条 | S3-16 下红 |
| 6 | `TestTheQueuesLimitIsTheConfigured`（第三份，`transfer_test.go:453-465`）、`TestOneImportAtOnceThroughServe`（第四份，`transfer_import_test.go:372-386`） | 在 `newAcmeTeam` 之后以 SQL 种排队的导出、导入，没有 River 行 | 与启动时的 `lost` 赛跑：`newAcmeTeam` 不等 `jobs started`，平常准备比任务启动慢得多，没有复现（审查 C 的"未核实的怀疑"） | 种子各 `tm.pool.Exec(ctx, riverHoldsSQL, …)`，消掉赛跑。修复第 1 轮改为排队行与 River 行一条语句（`riverHeldJobSQL`：`WITH j AS (… RETURNING id) INSERT INTO river_job … FROM j`），T2 的整个程序的测试同样：两条语句之间仍有一个往返的窗口；审查 A 量得启动时的 `lost` 总比 `newAcmeTeam` 返回早约 0.7 秒，下面的 `-count=20` 证明的是"不偶发"（S3 审查 A 的 N3，3.23 第 13 条） | 去掉 River 行，种子之后插一行收拾的任务让 `Run` 立刻跑一次（同 `bootstrap/upgrade_rollback_test.go:709`）并 `awaitJob` 它，这一格红；带着 River 行照绿 |

不改的：`holdExportWorker`、`holdImportWorker`（`interleavings_transfer_test.go:116-147`、`interleavings_import_test.go:174-…`）在团队建好、开了笔记本与页之后才写排队行，写下随即以 `FOR UPDATE` 锁住，再写 River 行；`FailQueued` 跳过被锁的行（`jobs.sql:138-147`）。剩下的窗口只是"写行到上锁"之间，而启动时那一次 `lost` 在十几次请求之前就跑完了（推断）。S3 的第 1 个提交在 `make test` 之外跑四个测试各 20 次（修订核对 C 的 N13）：`holdExportWorker` 只有 `TestDeletingANotebookAndStartingAnExport`（`interleavings_transfer_test.go:156`）用；`holdImportWorker` 的七处用者（`interleavings_wiki_test.go:211`、`:230`，`interleavings_background_stops_test.go:411`，`interleavings_background_test.go:332`，`interleavings_import_test.go:230`，`wiki_downgrade_test.go:39`，`upgrade_start_test.go:94`）里取最轻的 `TestTheDowngradeAnswersInItsOrder`（`wiki_downgrade_test.go:28`）：同一个帮手、同一个窗口，重的交错测试不各跑 20 次；再加第 6 条的两个。命令（经 `heavy.sh`）：`cd server && go test -race -count=20 -timeout 15m -run '^(TestDeletingANotebookAndStartingAnExport|TestTheDowngradeAnswersInItsOrder|TestTheQueuesLimitIsTheConfigured|TestOneImportAtOnceThroughServe)$' ./internal/bootstrap/`。用时：第二次修订在 `86565dd6` 上竞态下各跑一次（`m12/p3-rev2/time4.log`，负载 0.6，编译已缓存），3.31、0.99、0.78、0.79 秒，20 次约 2 分钟，另加编译。结果写进第 10 节。

S3 的第 1 个提交（6.3）带上这六处的改动，单独为绿。

### 3.15 契约

S1：

```yaml
# api/modules/events.yaml
    EventReset:
      description: The data of reset, the last frame.
      required: [reason]
      properties:
        reason:
          type: string
          enum: [access, notebooks_deleted, expired, unauthenticated, reconnected, overflow, superseded]
```

- `streamEvents` 的说明（`:54-61` 的原因一段）加 `superseded: the same credential (a sign-in's session or a token) opened a fifth stream, and this, its oldest, ends; connect again, as after any reset.`；`:71-72` 之后加一段：`A credential holds at most 4 streams, an account at most 64 over its credentials: a credential's fifth resets its oldest (superseded), and is never refused; an opening that would make the account's 65th is rate_limited, with Retry-After.` `:59-61` 的 "A reset for access, notebooks_deleted, reconnected or overflow comes after the events the stream held" 加上 `superseded`（它同样先写出已送到的事件，3.9.2；审查 B 的 N9）。`x-problem-codes` 不改（`rate_limited` 是顶层的码）。
- `api/modules/linking.yaml` 的 `listTags` 说明末句（3.4）。
- `api/mcp/tools.json` 两处 `log` 的 `description` 与 `path`（3.6），手改。

S3：

```yaml
# api/modules/transfer.yaml（四个开任务的操作，都在 server_busy 之前、transfer.busy 之后加；列表的次序不表示检查的次序）
      x-problem-codes: [notebook.not_found, page.not_found, transfer.busy, transfer.too_many_jobs, server_busy, storage_full]   # startExport :33
      x-problem-codes:
        [notebook.not_found, forbidden, notebook.upgrading, page.not_found, transfer.busy, transfer.too_many_jobs, server_busy,
         storage_full]                                                                                                          # startImport :105-106
      # startTrashRestore :176-188、startRevert :283-302：transfer.busy 与 server_busy 之间（startRevert 里在 server_busy 之前）加 transfer.too_many_jobs

    TransferProblem:
      properties:
        code:
          enum: [renamed, file_missing, unsafe_path, special_file, encrypted, unsupported_method, too_compressed,
                 name_not_utf8, name_unicode_path, name_gbk, invalid_content, too_large, too_deep, duplicate,
                 duplicate_source, unreadable]
```

- 四个操作的说明里讲 `server_busy` 的那一句（`:28-29`、`:88-90`、`:166-167`、`:256-257`）之前加：`One whose person has 5 jobs queued or running, with the imports they are uploading, over every notebook, is transfer.too_many_jobs: wait for one to end, or cancel one.`S3 实施并进分号列着的那一串，写成 `caller`，导入的写 "the other imports they are uploading"，意思相同（3.23 第 11 条）。
- `TransferProblem` 的说明（`:1055-1060`）加：`name_unicode_path and name_gbk are no skip: the entry's name, not UTF-8, was read from its Info-ZIP Unicode Path field, or as GBK (as a Chinese Windows writes names); they come after every other problem.`S3 实施多半句 "path being the name so read"（3.23 第 11 条）。
- `wiki.yaml` 的 `abandonUpgrade`、`rejectUpgrade` 不改：回滚不受按人的上限拒绝。
- 每处改动之后 `make gen`、`make gen-check`（05-lessons 第 8 节）。

### 3.16 码的表

| 码或值 | 状态 | 何处 | 新还是改 | Step |
|---|---|---|---|---|
| `transfer.too_many_jobs` | 409，无 `Retry-After` | `startExport`、`startImport`、`startTrashRestore`、`startRevert` | 新 | S3 |
| `rate_limited` | 429，`Retry-After: 30` | `streamEvents`（顶层的码，新的原因：账户的 64 条） | 新的起因，码不新 | S1 |
| `reset` 的原因 `superseded` | 流的最后一帧 | `streamEvents` | 新的枚举值 | S1 |
| 报告的 `name_unicode_path`、`name_gbk` | 报告里的一条，不是跳过 | 导入的任务 | 新的枚举值 | S3 |
| `validation_failed`（`path`，`too_short`） | 422 / MCP 的工具错误 | MCP 的 `log` | 新的一格（原来是 `required`） | S1 |
| `internal_error` 的文字 | MCP 调用超过请求的期限 | 每个 MCP 工具与方法 | 改文字，码不变 | S1 |
| `required`（名字） | 422 | 显示名、工作区名、PAT 的名字 | 新的一种输入答它 | S1 |

码的次序（审查 C 的 I2；初稿把导入与后台写反了）：

| 操作 | 次序 |
|---|---|
| `startExport` | … → 409 `transfer.too_many_jobs` → 503 `server_busy` → 409 `transfer.busy`（`admit` 先 `room`、后 `Exporting`，`start.go:186-201`） |
| `startImport`（`Check`，读文件之前） | … → 409 `transfer.busy` → 409 `transfer.too_many_jobs` → 503 `server_busy`（先 `Uploads.claim`、`writing`，后 `room`，`start_import.go:86-115`） |
| `startTrashRestore` | … → 409 `transfer.busy` →（`page.source_duplicate`）→ 409 `transfer.too_many_jobs` → 503 → 事务里再判定、写行 |
| `startRevert` | … → 409 `transfer.busy`（读任何东西之前）→ 409 `page.revert_conflict` → 409 `page.changeset_trimmed` → 409 `page.locked` → 409 `transfer.too_many_jobs` → 503 |

13.1 第 4 条的 M8/P4 两个开始照此加上新码，导出、导入补一句（第 8 节第 12 条）。契约的说明照这个次序写；S3 的 app 测试各有一格"两样都在"钉住（3.13.3）。

### 3.17 矩阵的行

- 权限矩阵不加行、不改格：没有新操作；四个开任务的操作的格是身份与可见性的（`permission_matrix_transfer_test.go:83-175`），按人的上限只在种子之外的情形出现。事件流整个模块豁免（13.4 第 6 条）。
- MCP 的码表：`historyCodeCalls`（`bootstrap/mcp_reads_test.go:557`）加 `log {path: ""}` 答 `validation_failed`（3.6）。
- `TestThePermissionMatrixCoversEveryOperation` 照过（没有新操作）。

### 3.18 日志

| 何处 | 级别 | 消息 | 字段 | Step |
|---|---|---|---|---|
| events 的 hub，顶掉一条 | debug | `event stream superseded` | `user_id`、`credential`（种类）、`streams` | S1 |
| events 的 hub，账户满了 | debug | `event streams of the account at their bound` | `user_id`、`streams` | S1 |
| MCP 调用超过期限 | warn（照旧） | `MCP call deadline exceeded` | 照旧 | S1 |
| transfer 的收拾，启动时 | warn | `queued job left by a stopped server` | 照 `Rescue.log`（`maintain.go:149-154`）：`job_id`、`notebook_id`、`user_id`、`client`、`failure` | S3 |
| 平台丢弃 River 的行 | warn（照旧） | `jobs left running by a stopped server discarded` | `kind`、`jobs`（`platform/jobs/jobs.go:146`） | S3 |

日志只写事实（05-lessons 第 8 节）。不记令牌与 PAT 的 id（13.1 第 10 条）。

### 3.19 分份：新的整个程序的测试

| 测试 | 文件 | Step | 名字的模式 | 进哪一份 | 推断的用时 |
|---|---|---|---|---|---|
| `TestTheLogReadsOneMomentAcrossARevert` | `bootstrap/history_snapshot_test.go` | S1 | `BOOTSTRAP_THIRD`（`The[F-Z]`），不在第二、第五份的模式里 | 加进 `SEVENTH`：第七份 | 1–2 秒（S1 实测竞态 2.39 秒） |
| `TestASignInsFifthStreamSupersedesItsOldest` | `bootstrap/events_streams_test.go` | S1 | `BOOTSTRAP_FIRST` | 加进 `SEVENTH`：第七份 | 约 1 秒（S1 实测竞态 2.19 秒） |
| `TestRevertingARollbackPutsBackWhatTheAgentLeft`（改名） | `bootstrap/upgrade_rollback_revert_test.go` | S2 | 补集（`TestR…`） | `SEVENTH` 与 `EIGHTH` 都换名字：第八份 | 比今天多 2–4 秒（S2 实测竞态多 0.1 秒，10.2） |
| `TestAnImportReadsNamesNotInUTF8` | `bootstrap/transfer_names_test.go` | S3 | `BOOTSTRAP_FIRST` | 加进 `SEVENTH`：第七份 | 1–2 秒 |
| `TestAPersonsSixthJobIsRefusedAndAnothersStarts` | `bootstrap/transfer_person_test.go` | S3 | `BOOTSTRAP_FIRST` | 加进 `SEVENTH`：第七份 | 约 1 秒 |
| `TestServeFailsTheJobsLeftRunning`（已有，种子多两行） | `bootstrap/transfer_jobs_test.go` | S3 | 补集 | 照旧第四份 | 不变（同一次起应用） |

- 加进 `SEVENTH` 的都写成 `Name$$`（同已有的点名）。第一、三份跳过 `SEVENTH`，所以它们离开原来的份。没有一个以 `E`、`Source`、`TheMCP[C-Z]`、`TheContents` 开头，不会在第二份再跑一遍（第二份只跳过 `FIFTH`，`Makefile:379`）。
- **核对**：每个 Step 在加测试的提交里跑一个脚本：`cd server && go test -race -list . ./internal/bootstrap`（只编译；要带 `-race`：组合根有 28 个文件是 `//go:build !race`，八份与 `Makefile:364` 的计数是竞态构建的列表，审查 C 的 N2），对每个顶层测试以 Python 的 `re` 照八份的 `-run`/`-skip` 判它落在哪几份（这几个模式在 RE2 与 Python 里意思相同），断言每个顶层测试恰在一份、新的在上表那一份；输出贴进 Step 的记录。`Makefile:364` 的计数注释（"554 个顶层 145/50/119/120/60/15/30/15"，竞态构建的列表；以 `grep` 数源文件是 607 个，不是这个数）照实改（S1 之后 556 个，145/50/119/120/60/15/32/15，10.1）。审查 C 照初稿的名字跑过这个判法（`m12/p3-review-c/shards.py`）；修订时照新的名字（四个新的、一个改名）在 `86565dd6` 的 `Makefile` 上再跑：新的四个都只在第七份，改名的只在第八份，全部顶层每个恰在一份（`m12/p3-rev1/shards.py`）。
- **第七份**：S2 合并之后两次 `main` 的平均 279.5 秒，落在 7763 上的一档约 315 秒；P3 加约 4–6 秒（推断：四个新测试），7763 上约 321 秒，余量约 129 秒。**第八份**：307 秒，加 2–4 秒。都在 450 秒之下，不挪份。每个 Step 合并之后两次 `main` 照 06-environment 第 4 节重看，过线先挪给邻份：从 `SEVENTH` 去掉名字之后，`TestA…` 回第一份、`TestTheL…` 回第三份（审查 C 的 N3），不提高 `-timeout`。

### 3.20 每个新检查的反向对照

13.4 第 1 条：新的检查先证明它会失败。各条的变异在 7.3，这里列检查与它的对照：

| 检查 | 反向对照 |
|---|---|
| 标题键的幂等与切罗基文的样本 | 去掉 `cherokeeCapital`：`TestTitleKeyIsStable` 红（S1-1） |
| 新旧 `searchFold` 的比对 | 同上：比对红；比对里的旧函数改成新函数（自指）时它恒绿，所以旧函数原样抄写、不调被测的代码 |
| 标签列表不列 `/` | 去掉 `<> '/'`：`TestTheTagsOfANotebook` 红 |
| 名字为空 | `Blank` 恒假：`TestCheckName`、`TestCleanClientName` 的新格红；`Blank` 逐码位的比对在"不减 Prepended_Concatenation_Mark"上红（S1-31） |
| 快照 | 四个用例各去掉一处：单元的那一格与整个程序的那一格红；快照晚开始、早结束只有单元的格红（S1-24、S1-29、S1-30）；先在没改的树上看整个程序的五格都红 |
| 提交错误的标记 | 7.3 的 S1-32–S1-34；先在没改的代码上看 `TestACommitThatFailsIsMarked` 的 (a)、(b) 红（3.7） |
| 流数 | 7.3 的 S1-12–S1-17 |
| 第 302 条 | 7.3 的 S2-1–S2-9 |
| 名字的解读 | 7.3 的 S3-1–S3-10；差分测试先在一个"所有没有标志的名字都按 GBK 读"的变异上红 |
| 按人的上限 | 7.3 的 S3-11–S3-15 |
| 启动时的收拾 | 7.3 的 S3-16–S3-18；改过的种子各有一个对照（3.14.3 的表的最后一列；第 4 条的不确定） |
| 分份的脚本 | 把一个新测试的 `$$` 去掉、或只加进 `EIGHTH`：脚本报"不在一份"或"在两份"；S1：去掉 `$$` 不会失败（没有别的顶层测试以新名字开头），会失败的是只加进 `EIGHTH` 与从 `SEVENTH` 去掉（3.21 第 9 条） |

### 3.21 S1 的实施对设计的修正

S1 的审查与修复见 [S1 的审查记录](reviews/P3-S1-small-correctness-review.md)，结果在 10.1。与上文最初的设计不同之处已改在各节里（头的状态，2.3 的 365、369、371，3.1 的文件表，3.2、3.3、3.6、3.7、3.8.3、3.9.2、3.9.3，3.19，3.20，4.1，6.1，7.1，7.3 的 S1，7.5，第 8 节第 1、2、7–10 条，第 9 节 P5/S3 一行与表后一句）；00 号文档第 7 节"总体设计的修订"第 10 条落了，4.6.4 与决定 365、369 加了括注，第 11 节加主持者的决定 483–486。

1. **`searchFold` 的调用不止三处**（3.2）：`page/domain/grep_keys.go` 的 `keyReader.fold` 也调它，几个测试（`grep_keys_property_test.go`、`search_bound_test.go`）与生成 `grep_tables.go` 的模板（`grep_tables_test.go`）也是；一并改调 `shared.TitleKey`，`grep_tables.go` 重新生成（只改了注释）。审查 A 核对过：`48bbfc99` 的非测试代码里是 `search.go:272`、`:282`、`:493` 与 `grep_keys.go:399` 四处，头上只剩注释与测试里原样抄写的 `searchFoldBefore`。
2. **切罗基文的一步不用 `strings.Map`**（3.2、决定 365）：`strings.Map` 把不是 UTF-8 的字节写成 U+FFFD，`TestTitleKeysKeepASCIIAndUnicodeBoundaries` 要求它们原样留着（审查 A 的变异 M-a：换成 `strings.Map`，它红）。不导出的 `cherokeeCapitals` 逐 rune 解码，遇到切罗基文小写时在一份拷贝里原地换 3 个字节，别的字节不动，没有小写时不分配；`TitleKey` 的非 ASCII 一支与 `TitleKeyContext` 的长的一支末尾经它，`IndexKeyer.Key` 照设计以 `shared.CherokeeCapital` 原地换。与旧的 `searchFold` 逐字节相同只对合法的 UTF-8 成立，搜索折叠的正文、查询与标题都先验过是 UTF-8（`page/domain/content.go:24`、`search.go:462`、`shared/title.go:24`），从库里读的名字与正文也是（PostgreSQL 的 `text` 存不下不合法的 UTF-8，审查 A）；比对的测试逐码位跑全部合法的码位。决定 365 随之加括注。
3. **`render_test.go` 那一格用 Markdown 链接**（3.3）：`platform/markdown` 没有维基链接的扩展，`[[#…]]` 在那里是纯文字。那一格改用 `[a](<#See https://a.example now>)`（同一个 `AnchorID`）；维基链接的一格在 `platform/markdown/obsidian/view_test.go`。这两格只守锚点一侧（锚点经 `AnchorID`，本来就带自动链接的文字），在没改的 `PlainText` 上也绿；在它上红的是 `parse_test`、`links_test` 的新格（审查 A 的 N2）。审查之后另加 `# Go www.example.com now` → `nw-go-wwwexamplecom-now`，四个测试各一格、写死：linkify 的 `www.` 链接的 `URL` 前面加了 `http://`，只用 `https://a.example` 时 `Label` 与 `URL` 相同，写 `URL` 的变异照绿（审查 A 的 M1）；render 与 view 的两格同一文档里有标题也有链接，自己就判得出相遇。
4. **`IndexKeyer.Key` 的两格是一个新测试** `TestAnIndexKeyIsCherokeesCapitals`（`linking/domain/index_test.go`），不加进逐个键写字面值的 `TestIndexKey`；两段大小写的端点另进随机差分的字母表。
5. **快照碰到已有的单元测试**（3.8.3 只写新测试）：`page.New` 与用例要 `Snapshots`，替身的 `historyReads()`（`history_fakes_test.go`）带上 `fakeSnapshots`；`history_test.go`、`path_history_test.go`、`ingest_history_test.go` 里写死的调用表加 `WithinSnapshot` 与 ` in tx`，仍是全表比对；"没有读某样"的否定断言原来按全名比，加了后缀之后恒真，改用新的 `calledAtAll`（按前缀，快照里外都算）。审查 B 逐个找过剩下的用法，没有一处变得恒真。`fakeNames` 不记调用，新测试另包一层 `recordedNames`；替身的快照 `oneSnapshot` 拒绝嵌套（同 `TxManager`），审查 B 的 overlay 变异（`page` 自己再开一个快照）让四格红。
6. **`TestVirtualRead/no_nested` 数这次读多出的快照**（`bootstrap/interleavings_virtual_test.go`）：它数页模块的 `TxManager` 自应用起开的快照，断言是 1；`llmWiki` 建笔记本时经 REST 读日志，现在多一个，变成 2。改为读之前取数、断言多 1，同一文件 `normal_unlocked` 的写法；守的仍是"虚拟文件的读恰好开一个快照、不嵌套"（审查 B）。设计没有提到这处，7 号提交之前它红。
7. **流数的整个程序测试用一次新的登录**（3.9.3 写"alice 以同一个访问令牌开 5 条"）：测试经 `/api/v0/auth/login` 再登录一次，让 5 条流的凭证只属于这个测试，是隔离。照设计用团队的令牌也确定地绿：`settle` 留下、还算着的旧流（至多 4 条）先被顶掉，第 5 次打开仍顶掉 5 条里的第 1 条（审查 C 的探针，`-race` 下 9 次绿）。实施报告原来写的"同一会话里还没关掉的一条会先被顶掉，第 1 条就不是最老的"不成立（审查 C 的 M2），照此改写，测试不动（主持者定）。
8. **hub 的测试在新文件 `events/app/hub_bounds_test.go`**（3.9.3 写在 `hub_test.go`、`openAs` 改为可指定凭证）：自己的几个帮手，`openAs` 与已有的测试不动；`hub_test.go` 是外部测试包，"同包的测试读两张表的长度"做不到，由新的 `export_test.go`（`package app`）导出 `HubCounts`、`AccountStreams`。
9. **分份脚本"去掉 `$$`"的反向对照不会失败**（3.20）：没有别的顶层测试以两个新名字开头，去掉之后每个测试仍恰在一份、新的仍在第七份；`$$` 防的是以后以同名开头的测试。会失败的对照是只加进 `EIGHTH`（报在第一、第八两份）与从 `SEVENTH` 去掉（报在第一份），都 rc=1。
10. **处理器的 429 一格先看状态**（3.9.3 只写断言）：答 200 时读第一帧就失败，不 `io.ReadAll` 一条开着的流。原来的写法在变异 S1-12 与 x7 下把 `events/adapter/http` 挂到 10 分钟的超时，第一次跑变异时发现，提交之前改了。
11. **提交在调用的期限之前失败的，也答"可能已保存"**（决定 483；第一次修复，代码审查 B 的 M1，主持者取甲）：3.7"路径"一段原来说提交跨过 `database.commit_timeout` 时"请求的期限也已过，`ProblemOf` 归为 `FaultDeadline`"，只在提交开始得晚时成立。提交的时限从提交开始算（`WithTimeout(WithoutCancel(ctx), commitTimeout)`），默认 2 秒短于 `server.request_timeout` 的 15 秒，所以更常见的是提交先超时、调用的期限未到，`ProblemOf` 归为 `FaultUnknown`，原来答 "the server failed"，agent 照它重试，不带 `base_version` 的 `edit` 可能重复追加。审查 B 以 overlay 探针核实了两种情形。`toolError` 与 `methodError` 的 `FaultUnknown` 一支也认 `TxCommitFailed`（`committing`、`failedText`）：带标记的答 `the server failed as its changes were being saved, so they may or may not have been; read the notebook to see whether they were before retrying it`，日志照旧 error；码照旧 `internal_error`，REST 不变。`answer_test.go`（"a fault at a commit"）与 `prompt_bounds_test.go`（`unknown_commit`）各一格整句比对，修之前都红。这一支认的是调用的期限还没过时任何带标记的提交失败，不只是超时；"`commit_timeout` 短于 `request_timeout`"只是默认值，配置的校验不要求它，所以注释不把它写成不变式（修复第 2 轮，核对 B 的 C1-N1）。`tx.go` 的注释照代码写：读写事务的提交失败都带标记，结局可能未知，连接在送出 `COMMIT` 之前就断了的是回滚（审查 B 的 N1）。3.7 那一句照此改了。
12. **每个 PAT 各是一个凭证，有测试守；顶掉时减被顶掉那一条自己的账户**（决定 484；第一次修复，审查 C 的 M1）：7.3 的 S1-12 只钉住"按凭证而不是按账户"，去掉 `credentialOf` 的 PAT 一支（全实例的 PAT 共用一个凭证）在单元与整个程序的测试下都活下来。`TestEachTokenIsACredentialOfItsOwn` 守它；顶掉时减被顶掉那一条自己的账户（`oldest.userID`，同 `close`），`TestASupersededStreamLeavesItsOwnAccount` 守它；顶掉与 `close` 共用 `uncount`（减一、到 0 就删），被顶到 0 的账户也不留项，同一个测试把 bob 顶到 0、再全关，断言两张表空（修复第 2 轮，核对 C 的 C1-N2）。去掉 `job` 一支是等价的：HTTP 的认证只造会话与 PAT 的 actor，任务经不到事件流。顶掉的 `debug` 一行仍记开者的 `user_id`（今天两者是同一个账户）。
13. **linking 的替身 `paths.Check` 照真的拒尾随的 `/`**（第一次修复，审查 A 的 M2）：原来收 `"/"`，7.3 点名的 "the root" 一格抓不到 S1-27，只有 `TestListIndexJudgesInTurn` 的调用次序抓到；改了之后 "the root"、"type and parent"、"a notebook hidden" 三格都抓到。`"/"` 单独一个照真的答 `required`（去掉开头的 `/` 剩空串），别的以 `/` 结尾的答 `invalid_format`（修复第 2 轮，核对 A 的 C1-N1）。只改测试。
14. **网页的三处注释写成 503 与 429 都带 `Retry-After`**（决定 485；第一次修复，审查 C 的 N3）：4.1 的"网页的代码与测试都不改"指行为；`W/events/hub.ts` 的 `#lead` 与 `retryAfterOf`、`W/services/event.service.ts` 的 `open` 只改了注释。注释写 429 的两种起因：请求的限流（打开流在限流中算一次请求）与账户到了流数的上限（修复第 2 轮，核对 C 的 C1-N1）。

### 3.22 S2 的实施对设计的修正

S2 的审查与修复见 [S2 的审查记录](reviews/P3-S2-nested-trash-roots-review.md)，结果在 10.2。与上文最初的设计不同之处已改在各节里（头的状态，2.3 的 364，3.10 的文件表、3.10.2、3.10.5–3.10.9，3.19，6.2，7.1，7.3 的 S2，7.5，第 8 节"S2 另"与第 3、4 条，第 9 节 v0.2 的 `M12-history.md` 一行）；00 号文档第 7 节"总体设计的修订"第 8、15 条落了，第 11 节加决定 479–482。写了行号的，代码是 S2 的头 `63cfcd9b` 的，文档是 `48bbfc99` 的（S2 不改 `docs/`）；提交见 10.2。

1. **两格组合根的测试钉着旧的拒绝，3.10 没有列出，随第 2 个提交翻过来**（偏离）：3.10 的文件表只列 `upgrade_rollback_revert_test.go`，3.10.5 只在原型上跑过 page 模块的测试。`make test` 在第一次的头 `737b15e7` 上红了两格，旧期望正是决定 304 去掉的读法，新结果与 3.10.2 的第 1–4 格同形：
   - `revert_batch_test.go` 的 `TestTheRevertOfEachBatchShape/two trash items one under the other, seeded`（M9/P3 3.14.5"恢复到另一组之下"，M9/P3/S2 的偏离 1）：旧 409、A 与 B `changed`；新 200，A 先放回、B 放回到 X 下，笔记本等于两次删除之前（`undoneFrom`）。
   - `interleavings_background_stops_test.go` 的 `TestABackgroundRevertStoppedInAGroup/going back`（M8/P4 3.10 交错 20 第三条）：撤销任务停在一组中间、撤销 R 之后再撤销 C，旧 409 `changed`；新 200，Box 那一项与 c3–c5 各自放回，笔记本等于删除之前。

   两格的测试头的说明随之改写。第 2 个提交单独为绿，所以翻转放进它（推送之前重做第 2–5 个提交，非测试代码不变）。审查 C 以 `topOf` 的新一步恒不成立（同 S2-1）跑这两格与改名的那一个：四格红（`seeded`、`going back`、`nested, request`、`nested, background`），`flat` 照过。
   - 交错 20 的"往回"是 3.10.2 **来源 3 的一例**：C 是一次 REST 的删除，停在组中间的后台撤销（c3–c5 各成一项）与 R 的撤销（Box、c1、c2 以新的时刻删成一项）把 C 删下的一组分成几项。它不是 [M12 的历史移交](handoffs/M8-history.md)第 6 条问的"任务自己的变更集分批删出嵌套"，那个照 00 号文档 4.6.1"不去造的"与第 9 节仍没有造过（审查 C 的 M1）。
   - 来源 3 的另一例（审查 A 的 N2；核对第 1 轮 A 的 C1-N1）：后台的回收站恢复停在一组中间，没恢复的部分各成一项、时刻仍是原来那一项的（`MarkTrashRoots`），之后恢复了的部分又被删掉，再撤销原来那次删除，就是 3.10.2 第 3 格的形状；旧规则答 `changed`，S2 依次放回。与交错 20 是同一种分法，差别只在部分恢复的是后台的回收站恢复还是后台撤销。**只在 domain 的 `world` 上核实过**（审查 A 的探针），没有造整个程序的测试。
   - 历史文档加后注、不改写原文（决定 481）：M8/P4 `04-P4-background.md:530`、`:735`，M9/P3 `03-P3-batch-revert.md:1899`；M9/P3 另有 6 处写着同一个 409（`:98`、`:156`、`:1869`、`:2120`、`:2720`、`:2770`），是当时的落实与结果，不动；M11 已是 `done` 的移交 `M8-P4-background.md:15` 不动；M12 的历史移交的"处理进展"加 S2 一行；00-M11 第 302 条加 00 号文档第 7 节第 15 条的括注。
   - 网页的 `transfer.revertLeftGroup`（"……逐项恢复就完成这一组；不要先撤销这次撤销"）由 **P3/S3** 改，S2 不碰网页（决定 482）。后半句"不要先撤销这次撤销"不再必要：撤销 R 之后，整组撤销 C（"going back"），或先恢复 R 的撤销删成的那一项再逐项恢复（"R reverted first"），都回到之前；只有先逐项恢复剩下的、后恢复那一项时，它们才因 `parent_in_trash` 落到根下。前半句仍对：任务停下之后直接再撤销 C，Box 已由 R 恢复、活着，答 `changed`（`page/domain/revert.go:512-514`），逐项恢复才完成这一组（"going on"）。（审查 A 的 N1；核对第 1 轮 A 的 C1-N2；初稿"仍对"的理由写错了。）

   实施中定，主持者可以改判。
2. **`TestARestoreWaitsForTheGroupItGoesUnder` 照新规则读**（3.10 的文件表只写"补第三个参数"）：它的说明写"No batch nor revert makes such a changeset, and PlanRevert reads the deletion of a node under another it deleted as one group"，决定 304 之后两句都不对了。这一处传 `s.Nodes`，去掉手工改 `tops`、`top`、`members` 的那一行（传了之后 `readReverted` 交回的正是它们），说明照新规则改写；其余四处传 `nil`（它们的条目没有删除）。S2-1 因此也让它红（7.3 的表多一格）。实施中定。
3. **domain 的表十二格**：3.10.2 的十一格之外另加 10b（"最深的页在 G 的另一个子页 K 下、不在 H 下"，只有 F 一条 `too_deep`）。3.10.3 的 `movesBack` 一条说"多不多那一条，看最深的页在不在嵌套的根下"，第 10 格只钉住了一半。第 2 格的 H1、H2 各在自己的时刻（t1 与 t1 之后一分钟），`RevertGroups` 的时刻断言分得开两项。实施中定。
4. **`f.recorded` 不改签名**（3.10.7 写"加一个变更集种类的参数"）：它在 `rewrite_test.go`、`source_job_batches_test.go`、`revert_job_test.go` 里还有四处调用，都是撤销的。另加 `recordedAs(kind, id, reverted)`，`recorded` 调它，不碰 S2 文件表之外的文件。实施中定。
5. **回滚的性质测试的请求一条也经核对的替身**：设计写"核对的替身没有记下违例"，而 `f.revert` 走不核对的 `f.writer()`。`checkedWrites` 里建 `Writer` 的部分拆成 `checkedWriter`，请求的撤销经它建 `RevertChangeset`（与后台的一条同一组依赖）。实施中定。
6. **回滚结束之后笔记本没有进行中的升级**：替身的 `rollingBack` 把升级记在笔记本上，`Done` 之后替身不清；撤销之前设成 `NotebookState{Mode: ModeStandard}`，同回滚结束之后的生产（审查 A 的探针默认也这样）。设计没写。实施中定。
7. **回滚的性质测试的 agent 删子树时先读整棵**（设计没有）：M11 的 agent 边走边删，一个节点往上的路经过已删的节点时读不到，被留成"活着、父节点不在"。照约定量下限时，在别的种子基上 56,000 次里 15 次因此失败：7 次在回滚的第一遍被核对的替身抓到（`48bbfc99` 上原样复现，这个测试从来只跑 0–1,999；6 次是 "after a deletion of …, not alive"，10334 是 "after a place of …, not alive"），8 次撤销答 `page.revert_conflict`（实施者诊断过 34196，是 `parent_gone`；审查 B 在头上以旧的 agent 跑这 8 个，都是 `parent_gone`）。测试自己的 2,000 个种子没有一个留下孤儿（`48bbfc99` 上核对），改后它们的计数逐字不变。实施中定。
8. **回滚的性质测试另数两条路**：请求与后台的撤销各数一次，与 `nested` 一样有下限（665/132），去掉任一条路的变异（X6、X7）红；设计只数 `nested`（364）。测试头的 "Not idle" 一句写上这两个（修复第 1 轮，审查 B 的 N1）。实施中定。
9. **`modelOf` 填的 `at` 没有人读**：照 3.10.7 填上，但 `load` 只装生成器的 `post`、从不装 `modelOf` 的结果，所以没有变异钉得住它；留着是为了模型与替身一致。
10. **后台撤销的性质测试的下限全部照约定重算**（修复第 1 轮；决定 479，审查 B 的 Q1）：`random` 的 `delApart` 与 `folder` 的 z 改了随机数的消耗，原有的 15 个下限已不全是"最少次数的三分之二"（审查 B 照 `a714787f` 的生成器数出 11 个会更低）。修复第 1 轮又加了 `folder` 的 w（第 12 条），随机数再变一次，所以在加了 w 之后的生成器上把 20 个下限（原有 15 个、嵌套 4 个、新的 1 个）都照约定重算：比原来低的 9 个（`-short` 的 `aside`、`restoreWrite`、`across`、`goneLater`、`goneSame`、`key`、`deepPlace`、`trashRestore`，全量的 `goneLater`），比原来高的 18 个，其余不变（10.2 的表）。测试头的说明写明出处。**偏离**：主持者的决定原写"降那 11 个"，那是加 w 之前的生成器上的 11 个；加 w 之后照同一约定，有的要降、有的要升，只降不升注释就仍不成立。主持者接受：它是同一约定在新生成器上的结果，不算改判。
11. **确定的一格加成员 Q**（修复第 1 轮；审查 A 的 M1，主持者的决定）：P1 原是 G 的最后一个成员，"放回到 P1 为止"等于"整组放回"，`needed` 的"只放回到那个成员"没有测试钉住。G 在 P1 之后加 Q，期望的步末尾是 `place y, restore Q, restore zz`，批照 `Joined` 切成 `[0,8)`、`[8,10)`。7.3 的表加两行：S2-3b"段内后面只要有一个放回就整组放回"、S2-3c"后面的放回挂在本组的某个成员下就整组放回"，都由这一格抓到；性质测试只核对结果、不核对段有多小，两条在它上面活下来（S2-3b 在 `-short` 上照实跑过）。
12. **后台撤销的性质测试造三层的单独删的项，另数一种**（修复第 1 轮；决定 480，审查 B 的 M1）：嵌套原来几乎都来自 `folder` 的 z（两层），"单独删的项套在另一个单独删的项下"每 2,000 次 0–6 次、没有计数，分时刻的装载（P3-3）只有全量偶尔钉住、`-short` 钉不住。`folder` 造 z 时另一半在 z 下加一页 w，先单独删 w、再单独删 z：w 的项在 z 的项下、z 的项在 G 的项下，z{zz} 不拆。第五个嵌套的计数 `nestedDeep`："一组放回到另一组的成员下，而那一组本身也放回到第三组的成员下"，下限照约定量（`-short` 6、全量 48）。所有单独的项同一时刻的变异（审查 B 的 B1）在 `-short` 上也红了；只让 w 随 z 一起删的变异（核对第 1 轮 B 的 B3）只让 `nestedDeep` 报 idle，说明它数的是"单独删的项套在单独删的项下"，不是"z 下有 w"。
13. **改名的那一个的两格 nested 先核对回收站里的两项**（修复第 1 轮；审查 C 的 N6）：撤销之前核对 Inner 与 Made 各是一项、Inner 在 Made 下、删得早于 Made，同另两格翻过来的测试。生产代码里造不出"回滚同一时刻删掉 Inner 与 Made、仍成功"的变异：让回滚不先删自由的子树（X9 的两种写法），回滚本身就以失败告终（任务答 `failed`、`failure: internal`、冲突数 0；原因没有查），第一种写法三格、第二种两格 nested 都红在回滚上，`flat` 在第二种下照过（核对第 1 轮 C 的 C1-N1）。所以只以测试里的对照核对这一行确实被判：X12（把"早于"反过来），审查 C 另以"各是一项"反过来（c1-notroot）核对。

### 3.23 S3 的实施对设计的修正

S3 的审查与修复见 [S3 的审查记录](reviews/P3-S3-transfer-jobs-review.md)，结果在 10.3。与上文最初的设计不同之处已改在各节里（头的状态，3.11 的文件表，3.12.1 第 2 条，3.12.2、3.12.3、3.13.3、3.14.2，3.14.3 的第 6 行，3.15，4.2，6.3，7.1 的网页一行，7.3 的 S3 加实施与修复的变异，7.5，第 8 节"S3 另"与第 5、6、12、13 条，第 9 节的导入名称）；00 号文档第 7 节"总体设计的修订"加第 28 条并标已落，第 11 节加主持者的决定 530–532。写了行号的，代码是 S3 的头 `cbda8435` 的，文档是 `4136991c` 的（S3 不改 `docs/`）；提交见 10.3。

1. **文档不随代码的提交**（6.3 把 `README.md:192`、`:193` 放进各自的代码提交）：文档的提交是主持者的，README 的两句与第 8 节第 5、6、12、13 条随 S3 的文档提交落；代码的提交不碰 `docs/` 与 `README.md`（决定 531）。
2. **决定 482 是第 4 个代码提交**（`a48ace20`，6.3 没有列）：`transfer.revertLeftGroup` 的两种语言去掉后半句（英文止于 "completes that group."，中文止于"逐项恢复就完成这一组。"）。文字在 `W/i18n/messages/{en,zh-CN}.ts`（3.11 的文件），断言它的是 `W/pages/notebook/transfer-background.test.tsx` 的两处（3.11 没有列），一并改了；单独一个提交，留着（决定 530）。中文的那一句起初没有测试整句比对，修复第 1 轮由 `transfer-report.test.ts` 两种语言都比对（审查 A 的 N2）。M8/P4 `04-P4-background.md:912` 引着被去掉的后半句，本 Step 的文档提交照决定 481 的先例加后注（审查 A 的 N1）。
3. **app 的替身认得来源**（3.12.2 写"替身照旧交回原始名字；app 的测试直接构造带 `From` 的 `RawEntry`"）：替身的 `archives` 加一张 `from`（名字 → 来源），`zipArchive.Entries` 照它填 `From`（`app/fakes_test.go`，不是 `import_fakes_test.go`）；app 的测试照旧以 zip 造导入，经 `Import.Run` 的整条路走到报告。
4. **zip 的帮手**（3.12.3 写 `zipEntry` 加 `noFlag`、整个程序的 `vaultZip` 同样处理）：适配器的表直接交 `*zip.FileHeader`（`NonUTF8: !flagged`，要标志时 `Flags: 0x800`），已有的 `zipped` 收它；整个程序的测试在自己的文件里有 `unflaggedZip`（每个条目 `NonUTF8: true`，可带扩展字段），`vaultZip` 与它的用者不动。
5. **差分在内部测试**（3.11 只列 `names_test.go`、`names_fuzz_internal_test.go`）：`readNames` 不导出，差分 `TestReadNamesReadsAsGBKOnlyAZipOfGBKNames` 在新的 `names_internal_test.go`（`package archiveadapter`）；表在 `names_test.go`（外部测试包，经 `Archives.OpenImport` 与 `Entries`）。生成器是 Go 的 `math/rand/v2` PCG（整数），种子 1–3，各 1,000 个包；13 个计数的下限是另 20 组种子（`3b+1`–`3b+3`，b 为 1–20）的最少次数的三分之二、向下取整，最少次数在 10.3。
6. **第一个 0x7075 字段说了算**（3.12.1 第 2 条没说一个条目有几个时取哪个；实施中定）：只看第一个，它无效就当没有，不往后找；模糊的种子里有"第一个 CRC 错、第二个有效"的一格，模糊的断言是"当且仅当第一个有效才取，取的是它的名字"（比 3.12.3 写的"交回真时……"多了"只有"一半）。修复第 1 轮另加三个种子钉住 `unicodePath` 的三处边界（审查 B 的 M1）：整个只有 1 字节的字段（`75 70 01 00 01`，短于版本与 CRC）、尺寸比余下的多 1 字节的（`full[:len(full)-1]`）、截在 CRC 与名字的第一个字 `测` 之后的（`full[:12]`，放松了的边界会把 `测` 读成名字）。测试里这个种子的注释写成"截在 `测` 的中间"，不对；注释照旧，不另加提交（核对第 1 轮 B 的 C1-N1：真截在中间的 `full[:11]` 读出的不是合法 UTF-8，抓不到"把尺寸截到余下的"那个变异）。
7. **`Read` 收哪些**（3.12.2"没有被 `ignored` 滤掉的"）：按 `pathOf` 的 `hidden` 判，被忽略的不进；越出目录等被跳过的进；作为库的顶层文件夹、本身不导入的那一个条目也进（它的名字确是这样读的；探针：`gbk:图片/`、`gbk:图片/.obsidian/app.json`、`gbk:图片/a.md` 三个条目，`Read` 是 `图片/` 与 `图片/a.md`）。`Read` 的路径是 zip 里读出的名字，含库的文件夹，同跳过的问题。修复第 1 轮另钉两处：表第 11 格加一个 GBK 名字以 0x5C 结尾的符号链接 `资料/俓`，它按读出的名字判 `Special`（审查 B 的 N1）；失败的导入（`too_many_nodes`）照样列出读过的名字（审查 B 的 N2）。
8. **来源到码**：`NameSource` 的一个不导出的方法 `problem()`（一个 `switch`），不是包级的表：`gochecknoglobals` 不许包级的 `map`。
9. **按人的上限的测试**（3.13 的格）：app 的格是三个新测试 `TestAPersonsSixthExportIsRefused`、`TestAPersonsSixthImportIsRefused`、`TestAPersonsSixthBackgroundStartIsRefused`，`Rollbacks.Start` 的一格在已有的 `TestARollbackStartsInItsUpgradeAlone` 里；http 的四格（导出、导入、恢复、撤销各一，"five jobs of …"）断言 409 `transfer.too_many_jobs`、没有 `Retry-After`；http 的替身 `maxQueued` 是 3，五个任务同时让全局也满了，所以这四格也守先后（S3-15 在它们上也红）。网页 `transfer-import.test.tsx` 里 `importCut` 那一格原来只断言包含，改为整句比对，没有另加一格。后台的"锁下再数一次"原来没有一格钉住（锁外那一次就拒了），修复第 1 轮加 "four of his, his fifth after the first count"：替身在锁外的计数之后给他加第 5 个，锁下那一次拒他的第 6 个（审查 C 的 M1）。
10. **T4 的测试帮手**：模块的测试加 `root.module`（交回模块与它的 `Inserter`，`contributors_test.go`），`start` 改调它；app 的替身 `InterruptJobs` 记进调用表，`TestTheRescue` 照调用的次序断言。
11. **契约说明的措辞**（审查 C 的 N1）：3.15 要在讲 `server_busy` 的那一句之前另加一句 "One whose person has 5 jobs queued or running, with the imports they are uploading, over every notebook, is transfer.too_many_jobs: wait for one to end, or cancel one."；实施把它并进分号列着的那一串，写成 "one whose caller has 5 jobs queued or running, with the imports they are uploading, over every notebook, is transfer.too_many_jobs: wait for one to end, or cancel one"，导入的写 "the other imports they are uploading"（自己这一次不算）；导出那一段的 `transfer.busy` 挪到 `server_busy` 之后，照 3.16 的次序。`TransferProblem` 的说明比 3.15 多半句 "path being the name so read"（`path` 是读出的名字）。意思与设计一致。
12. **"不是 UTF-8"的文字不改**（审查 B 的 N3，主持者定）：契约、domain 的注释、网页两种语言都说这两种名字"不是 UTF-8"。从字段读出的 ASCII 名字（`??.md` → `说明.md`），决定 376 下读成 GBK 的合法 UTF-8 名字（`ü.md` → `眉.md`），其实也是合法 UTF-8；对读者，这几个字的意思是"名字照原样读不出"（决定 532）。
13. **三处种子写成一条语句**（审查 A 的 N3，修复第 1 轮）：3.14.3 第 6 条的两个测试与 T2 的整个程序的测试，排队行与 River 行由 `riverHeldJobSQL` 写成一条语句（`WITH j AS (INSERT … RETURNING id) INSERT INTO river_job … FROM j`），不是 3.14.3 写的两条 `Exec`：启动时的 `lost` 先读行、再读 River，落在两条语句之间就把那一行记成失败。审查 A 量过，本机竞态下 20 次，`lost` 都比 `newAcmeTeam` 返回早约 0.7 秒，`-race -count=20` 证明的是"不偶发"，没有碰上赛跑；现在窗口没有了。`seedJobs` 在起应用之前写，`rollingBackIn` 在同一个事务里写；矩阵的排队导出在 prepare 的应用起来之后、以两条语句写，那时那个应用启动时的 `lost` 早已跑完、周期的收拾 5 分钟之后才跑，各格的应用在种完之后才在库的副本上起：都不改（核对第 1 轮 A 的 C1-N1）。
14. **启动时 `lost` 失败就让启动失败**（审查 A 的 N4，修复第 1 轮）：T4 之后启动多读行与 River，任何一步出错都经 `jobs.Job.Start` 让启动失败（与 `InterruptJobs` 失败时一样，改之前同样的错只让每 5 分钟的 `Run` 失败一次）。`TestTheRescue` 末尾一格钉住。

## 4. 网页

P3 不改网页的行为，只让新码、新原因有文字。S3 的网页部分与 P2 重叠的地方和合并的次序见第 0 节。

### 4.1 S1

- 网页的代码与测试都不改（审查 B 的过度设计：初稿的 429 一格走的是 `hub.test.ts:194` 已经走过的路）。对 429 有效的依据是 `ApiError` 对任何状态码都取 `Retry-After`（`W/services/api.ts:61`、`:70`），`hub.ts` 的 `retryAfterOf` 不看状态码（`:269-273`）。一个浏览器只有持有领导权的标签页开一条流，其余经 `BroadcastChannel` 收（`W/events/hub.ts:6-10`、`W/events/leadership.ts`），所以 4 条只在半开的连接、几个配置文件时碰到；`reset` 的任何原因都立刻重连（`hub.ts:198-226`），`superseded` 不另处理。S1 修复只改了 `hub.ts` 的 `#lead`、`retryAfterOf` 与 `W/services/event.service.ts` 的 `open` 的注释，写 503 `not_ready` 与 429 `rate_limited`（请求的限流，或账户到了流数的上限）都带 `Retry-After`；"不改"指行为（决定 485，3.21 第 14 条）。
- 生成的类型（`schema.gen.ts`）随 `make gen` 多出 `superseded`；没有以这个枚举为键的 `Record`，tsc 照过（实施时 `rg "notebooks_deleted" W/` 核对）。
- 标签列表少了 `/`：网页的标签页与补全照列表显示，不改；`#//` 一直不能点进去（`getTag` 叫不出它）。
- 名字为空、标题 id、标题键、MCP 的改动网页都不经手：表单把 `required` 照 `field.required` 显示（`problem-messages.ts` 的 `fieldMessages`），与今天的空名字相同。

### 4.2 S3

- **新码的文字**：`W/app/problem-messages.ts` 的 `problemMessages` 加 `"transfer.too_many_jobs": "problem.transfer.too_many_jobs"`。`problem-messages.test.ts` 逐个操作核对契约的码（`:55-60`），契约加了码而这里没加时它红，所以同一个提交里加。
  - en：`"problem.transfer.too_many_jobs": "You have 5 jobs queued or running, in any notebook. Wait for one to end, or cancel one."`
  - zh-CN：`"problem.transfer.too_many_jobs": "你已有 5 个排队或进行中的任务（各笔记本合计）：等一个结束，或取消一个。"`
- **四处开任务的地方**都经 `errorText` 显示它（导出与导入的对话框的 `texts` 不覆盖它，`export-dialog.tsx:76-80`、`import-dialog.tsx:217-223`；回收站一行的 `refusedText` 落到 `refusalText` 再到 `errorText`，`trash-row.tsx:154-164`、`:125-140`；撤销对话框的 `explain` 不认它、落到 `useForm` 的通用文字，`revert-dialog.tsx:186-200`）。
- **回收站一行的"在后台恢复"**：`offeredOf`（`trash-row.tsx:172-177`）在后台开始被拒时，`transfer.busy` 与 `server_busy` 之外也对 `transfer.too_many_jobs` 留着按钮（等自己的一个任务结束再按），P3-17；注释照改。
- **假服务端**（审查 C 的 I2）：只改 `W/test/jobs-server.ts`，加 `tooManyJobs`，答 409 `transfer.too_many_jobs`，照真服务端的次序（3.16）：恢复的路由在 `sourceRestoreRefusal` 之后、`return admission ?? queued(…)` 之前（`:328-332`）；撤销的路由在 `page.locked` 之后、503 之前。不放进 `refused()`（`:209-215`）：它的 409 被 `status === 409` 提前返回（`:329`、`:366-368`），放进去会在撤销的冲突之前答出新码。导出、导入的路由今天不判 `busy` 与 `queueFull`，那几格照旧经测试自己的 `answers` 给答复。`W/test/history-server.ts` 只有同步的撤销（`:429`）、从不开任务，不改；`W/test/upgrade-server.ts` 不改（回滚不被按人的上限拒绝）。
- **报告的两种新原因**：`transfer-report.tsx:92-108` 的 `problems` 是以生成的联合类型为键的 `Record`，契约加了值、这里不补就编译不过：
  - `name_unicode_path: (problem, t) => t("transfer.problem.name_unicode_path", { path: problem.path })`
  - `name_gbk: (problem, t) => t("transfer.problem.name_gbk", { path: problem.path })`
  - en：`"transfer.problem.name_unicode_path": "{path}: its name was not UTF-8 and was read from the Unicode name the zip tool stored with it."`；`"transfer.problem.name_gbk": "{path}: its name was not UTF-8 and was read as GBK, as Chinese Windows writes names. If it reads wrong, rename it."`
  - zh-CN：`"transfer.problem.name_unicode_path": "{path}：名称不是 UTF-8，取了压缩工具另存的 Unicode 名称。"`；`"transfer.problem.name_gbk": "{path}：名称不是 UTF-8，按 GBK（中文 Windows 的写法）读出；读错了的话，给它改个名。"`（条目也可能是附件或文件夹，审查 C 的 N8）
  - 导入报告的标题"没有照原样导入的条目"（`transfer.importProblemsTitle`）照样适用。两条只在导入的报告里出现，不加 `transfer.importProblem.*` 的变体。
- **导入的"连接中断"**（审查 C 的 M4）：服务端在读完请求体之前答复时，浏览器只看到连接被重置，导入对话框显示 `transfer.importCut`（`import-dialog.tsx:224`），它列举服务端会在收下 zip 之前拒绝的原因。按人的 409 也在读文件之前（`Check`），两种语言都加上这一条：
  - en：`"The upload did not finish: the connection was cut. The server may have refused the zip before taking it (an import, a restore or an undo under way in this notebook, 5 jobs of yours queued or running, a full queue, too little room, or a zip too large), or the upload was too slow. Try again later."`
  - zh-CN：`"上传没有完成：连接中断了。服务器可能在接收之前就拒绝了它（这本笔记本有进行中的导入、恢复或撤销、你已有 5 个排队或进行中的任务、排队的任务已满、存储空间不足或 zip 太大），也可能是上传太慢。请稍后再试。"`
- **测试**（vitest；测试文件照实，审查 C 的 N1）：
  - `problem-messages.test.ts` 照过（它就是核对）；
  - 导出（`transfer-page.test.tsx`、`page-menu.test.tsx`）与导入（`transfer-import.test.tsx`）各加一格：开始答 `transfer.too_many_jobs` 时横幅是新文字，对话框不关；`transfer-import.test.tsx` 里连接中断的那一格原来只断言包含，S3 实施改为整句比对、没有另加（3.23 第 9 条）；横幅的几格照 P2/S1 合并之后的写法（第 0 节）；
  - 回收站一行（`trash-background.test.tsx`）：后台开始答 `transfer.too_many_jobs` 时文字是新文字、按钮留着（`offeredOf` 交回那一项）；
  - 后台撤销（`revert-background.test.tsx`；`revert-dialog.test.tsx` 不管后台）：答 `transfer.too_many_jobs` 时横幅是新文字；
  - 报告（`transfer-report.test.ts`）：一个导入的报告带 `name_gbk` 与 `name_unicode_path` 各一条，文字照写死的期望；
  - 决定 482（S3 实施，第 4 个提交 `a48ace20`，决定 530）：`transfer.revertLeftGroup` 去掉后半句，`transfer-background.test.tsx` 的两处断言照改；`transfer-report.test.ts` 两种语言整句比对它（修复第 1 轮，S3 审查 A 的 N2）；
  - 两种语言的键一致由现有的 i18n 测试核对。
- 文字照 13.2 第 11 条：不写"请"之外的客套，不写实现的词（"River"、"队列"不出现）。

### 4.3 不改的

- 升级页的回滚（`upgrade-page.tsx`）：回滚不被按人的上限拒绝。
- 事件流的退避与 `Retry-After` 的读法。
- 历史页：快照只改服务端读的时刻，答复的形状不变。

## 5. 端到端测试

不加故事，不改已有的故事（00 号文档第 7 节 P3 一行"不加故事分组"）。

- TR4（`e2e/stories/transfer/tr4-malicious.spec.ts`）的 `f\xff.md`（`:68`）照旧是 `name_not_utf8`（`:105`）：0xFF 按 GBK 读出 U+FFFD，整包不按 GBK 读（3.12.1 的实验）；包里别的名字都是 ASCII 或带标志的合法 UTF-8。
- 事件流：e2e 的帮手每个账户、每个列开一条流（`e2e/fixtures/events.ts:55-57`），一个浏览器只开一条，到不了 4 条。
- 历史、标签、名字的改动没有故事断言旧的行为（`rg` 过 `#//`、`ran out of time`；审查 C 另核对了切罗基文与带自动链接、原始 HTML 的标题 id）。
- T4：没有故事以 SQL 种排队的任务或重启服务端（审查 C 核对），不受影响。一个故事至多两次 `anotherPage`（`e2e/fixtures/test.ts:214-221`），加上主页面，同一凭证到不了 4 条流。
- 每个 Step 合并之前照 06-environment 经 `heavy.sh` 跑全量 `make e2e` 一次，证明没有故事变了：S1（事件流与历史的读）、S3（导入与开任务）必跑；S2 只改 page 的 domain 与 app，也跑一次（撤销的故事 H 系列走同一个计划）。

## 6. 实施的次序

每个提交单独为绿（02-workflow 第 3.1 节）。下面每一行是一个提交，括号里是它带的测试。

### 6.1 S1

1. 标题键：`shared` 的一步、`TitleKeyContext`、`IndexKeyer.Key`；`searchFold` 删掉，三处改调；新旧比对（3.2 的全部测试）。
2. 标题 id：`PlainText`（3.3 的测试）。
3. 标签列表：SQL、`linking.yaml` 的说明、`make gen`（sqlc 与契约的生成物一起，3.4 的测试）。
4. 名字：`shared.Blank`、`CheckName`、`CleanClientName`（3.5 的测试）。
5. `log` 的 `path`：`AtPath`、MCP 的说明、`tools.json`（3.6 的测试）。
6. MCP 超时的文字与提交错误的标记：`platform/postgres/tx.go` 的 `commitError`，`mcpserver` 的 `deadlineText` 与两句文字（3.7 的三处整句比对、`TestACommitThatFailsIsMarked`、`answer_test.go` 的提交一格）。
7. 快照：`HistoryReads.Snapshots`、四个用例、注释（单元的测试）；整个程序的交错 `TestTheLogReadsOneMomentAcrossARevert` 与 `Makefile` 的 `SEVENTH`（3.19 的脚本）。
8. 事件流：hub、`Stream`、`OpenStream`、domain 的原因与错误、`events.yaml`（含 `:59-61` 那一句）、`make gen`；hub 与处理器的测试；`TestASignInsFifthStreamSupersedesItsOldest` 与 `SEVENTH`。
9. 文档：`README.md:198`、`:207`、`:210`、`:223`；总体设计 13.1 第 28、30 条与第 15 节一行（第 8 节）；本文 3.21 与第 10 节。

1–6 互不依赖，可以换次序；7 与 8 各自独立。

S1 照第 1–8 个的次序做；审查之后修复第 1 轮加 6 个提交、修复第 2 轮加 4 个，两次合并 `main`（`d1e90929`、`892dd827`），文档在它们之后（10.1）。

### 6.2 S2

1. domain 测试的 `world.state()` 照根的时刻读组（P3-3）；现有的格照过（只动测试）。
2. 规则：`reverted.now`、`readReverted` 的签名、`topOf`、五处构造计划传 `Nodes`、注释；domain 的表 `TestPlanRevertOfNestedTrashRoots` 与 `TestRevertGroupsAndRefs` 的新格；**同一个提交里** `bootstrap/upgrade_rollback_revert_test.go` 的 nested 一格翻过来、改名，`Makefile` 的两个模式换名字（不然这个提交的 `make test` 红在旧的 409 上，02-workflow 第 3.1 节）。
3. `needed`：条件与注释；性质测试的分时刻的装载与 `delApart` 的帮手（确定的一格要用，审查 A 的 N3）；`TestARestoreUnderAMemberCutShortInASegmentBringsItBack`。先于第 4 个提交：扩开的性质测试造得出 `needed` 的形状，没有这个修正它红（审查 A 核对过）。
4. `TestTheJobsStepsReplayToStar` 扩开：`random` 的 `delApart`、`folder` 的 z、四个计数与下限。
5. `TestTheRollbacksStepsReplayToTheBaseline` 扩开：撤销回滚的变更集，请求与后台轮流，`nested` 一个计数与下限。
6. 文档：总体设计 5.8 第 6 步、13.1 第 2 条（第 8 节）；本文 3.22 与第 10 节。

S2 照第 1–5 个的次序做，第 5 个另数请求与后台两条路（3.22 第 8 条）；审查之后修复第 1 轮另加四个只改测试的提交，文档是第 10 个（10.2 的表）。

下限的数（第 4、5 个提交）照 3.10.7 的约定在实施时量出：先以 20 个种子基跑 2,000 次、40 个基跑 400 次，记下每种的最少次数，写进测试的注释与第 10 节。

### 6.3 S3

1. 启动时的收拾（T4）：五种任务的 `DiscardRunning`、`AtStart` 的 `lost`、`lost` 带日志的话、`Run` 的注释；River 适配器、模块的测试，`TestTheRescue` 改断言；`riverHoldsSQL`、`riverTakenSQL` 与 3.14.3 的六处种子；`TestServeFailsTheJobsLeftRunning` 的三种排队行；`README.md:193` 讲窗口的那一句（第 8 节）。没有这六处种子这个提交就红（审查 C 的 I3），所以它们同在这个提交里。
2. 按人的上限（T2）：`MaxPersonJobs`、`ErrTooManyJobs`、`CountActive`、`Uploads` 记人、`room` 与五处调用；`transfer.yaml` 的四处码与说明、`make gen`；网页的码、两种语言的文字（含 `transfer.importCut`）、`offeredOf`、`jobs-server.ts`（4.2）；各层的测试；`TestAPersonsSixthJobIsRefusedAndAnothersStarts`（种子带 River 的行）与 `SEVENTH`；`README.md:193` 讲队列的那一句。
3. 名称的编码（T3）：`names.go`、`Entries`、domain 的 `NameSource` 与 `Read`、报告的两种原因、`importRun.finish`；`TransferProblem` 的枚举与说明、`make gen`；网页的报告与两种语言的文字（4.2）；适配器的表、差分、模糊，domain、app 的测试；`TestAnImportReadsNamesNotInUTF8` 与 `SEVENTH`；`README.md:192`。
4. 决定 482（S3 实施加的，3.23 第 2 条，决定 530）：`transfer.revertLeftGroup` 去掉"不要先撤销这次撤销"，两种语言与 `transfer-background.test.tsx` 的两处断言（`a48ace20`）。
5. 文档：本文 3.23 与第 10 节；S3 实施起还有 `README.md:192`、`:193` 的两句与第 8 节第 12、13 条，第 1–3 个代码提交不碰 `docs/` 与 `README.md`（决定 531）。

第 2、3 个提交都改 `transfer.yaml`、`make gen` 的生成物、`en.ts`、`zh-CN.ts`、`Makefile` 的 `SEVENTH`，第 1、2 个都改 `README.md:193`（审查 C 的 N5）：照 1、2、3 的次序做，后一个在前一个之上改，不并行。第 2、3 个的网页部分在 P2/S1 合并之后写（第 0 节）。S3 照此做了；`README.md:193` 改在第 5 个（决定 531）。

## 7. 测试与验证

### 7.1 新的与改的测试

| Step | 层 | 测试 | 证明什么 |
|---|---|---|---|
| S1 | shared | `TestTitleKeyIsStable`、`TestTitleKeysMatchAcrossCaseAndNormalization` 的新样本；`title_key_context_test.go` 的长标题 | 切罗基文的键幂等、大小写相同 |
| S1 | linking/domain | `TestAnIndexKeyIsCherokeesCapitals`（`IndexKeyer.Key` 的两格，3.21 第 4 条） | 索引的键同一步 |
| S1 | page/domain | `TestSearchFoldIsTheTitleKeyNow` | 删掉 `searchFold` 之后搜索的键逐字节不变 |
| S1 | page/app | 建页的一格（`Ꭰ` 与 `ꭰ`） | 只差切罗基文大小写的兄弟页被拒 |
| S1 | platform/markdown | `TestHeadingsGetPrefixedUniqueIDs`、`TestHeadingIDIsAHeadingsID`、`TestALinkToAnAnchorAloneLeadsToItsHeading` 的新格；`obsidian/view_test.go` 的维基链接一格（3.21 第 3 条）；四个测试的 `www.` 一格（S1 审查 A 的 M1） | 标题 id 与锚点对自动链接与原始 HTML 相遇；写的是自动链接的 `Label`、不是 `URL` |
| S1 | linking/postgres | `TestTheTagsOfANotebook` | 不列 `/` |
| S1 | shared、identity | `TestCheckName`、`TestCleanClientName`、显示名的一格；`Blank` 逐码位的比对 | 看不见的名字算空；emoji 序列、ZWNJ、U+2800、`۝` 照旧；与 Unicode 的推导一致 |
| S1 | page/app、bootstrap | `AtPath` 的两格、`historyCodeCalls`、`TestTheMCPHistoryIsRESTs` 的 `log` | `"/"` 是整本，`""` 答 `too_short` |
| S1 | linking/app | `backlinks_test.go` 的替身（第一次修复） | 替身照真的 `Check` 拒尾随的 `/`，S1-27 由 "the root" 等三格抓到（3.21 第 13 条） |
| S1 | mcpserver、bootstrap | 三处整句比对 | 超时的新文字 |
| S1 | platform/postgres | `TestACommitThatFailsIsMarked`（三格） | 读写事务的提交错误带标记、只读的不带；提交跨过时限时错误包着 `DeadlineExceeded`、带标记 |
| S1 | mcpserver | `answer_test.go` 的提交一格；第一次修复加 "a fault at a commit" 与 `prompt_bounds_test.go` 的 `unknown_commit`（决定 483） | 带标记的超时答另一句；调用的期限没过、带标记的故障也答"可能已保存" |
| S1 | page/app | `TestTheLogAndAChangesetReadInOneSnapshot`；替身的快照拒绝嵌套，已有的调用表照改（3.21 第 5 条） | 四个用例从各自第一条属于答复的读起在一个快照里 |
| S1 | bootstrap | `TestTheLogReadsOneMomentAcrossARevert`（交错，13.4 第 4 条）；`TestVirtualRead/no_nested` 改数这次读多出的快照（3.21 第 6 条） | 真库上五种读看到同一时刻 |
| S1 | events/app | 五个 hub 测试（`hub_bounds_test.go`，3.21 第 8 条）；另 `TestTheHubForgetsWhatClosed`、`TestTheRefusalSaysItsBound`，第一次修复的 `TestEachTokenIsACredentialOfItsOwn`、`TestASupersededStreamLeavesItsOwnAccount` | 按凭证顶掉、按账户拒绝、互顶会停下、关掉之后两张表空；每个 PAT 各是一个凭证；顶掉时减被顶掉那一条的账户，到 0 也删（决定 484） |
| S1 | events/http | `TestAStreamThatCannotOpenIsAProblem` 的一格 | 429、`Retry-After: 30` |
| S1 | bootstrap | `TestASignInsFifthStreamSupersedesItsOldest` | 真的流上第 5 条顶掉第 1 条 |
| S2 | page/domain | `TestPlanRevertOfNestedTrashRoots`（十二格，3.22 第 3 条）、`TestRevertGroupsAndRefs` 的一格；`revert_schedule_internal_test.go` 的四处照新签名，`TestARestoreWaitsForTheGroupItGoesUnder` 照新规则读（3.22 第 2 条） | 第 302 条的规则与反例 |
| S2 | page/app | `TestARestoreUnderAMemberCutShortInASegmentBringsItBack` | `needed` 的修正，只放回到要用的成员（成员 Q，3.22 第 11 条） |
| S2 | page/app | `TestTheJobsStepsReplayToStar`、`TestTheRollbacksStepsReplayToTheBaseline` 扩开 | 随机的嵌套形状重放到期望，五种嵌套各有下限（`nestedDeep` 是 S2 加的，3.22 第 12 条）；回滚的变更集撤销得了，请求与后台两条路各有下限 |
| S2 | bootstrap | `TestRevertingARollbackPutsBackWhatTheAgentLeft` | 真库上请求与后台的撤销都成功；撤销之前回收站里是两项（3.22 第 13 条） |
| S2 | bootstrap | `TestTheRevertOfEachBatchShape` 的 seeded、`TestABackgroundRevertStoppedInAGroup` 的 going back（翻过来） | 上下相套的两项、停在组中间之后撤销 R 再撤销 C，都撤销得了，笔记本等于之前（3.22 第 1 条） |
| S3 | transfer/archive | `TestTheImportReadsNamesNotInUTF8`（十三格）、`TestReadNamesReadsAsGBKOnlyAZipOfGBKNames`（差分）、`FuzzUnicodePathReadsOnlyAValidField` | 3.12.1 的规则 |
| S3 | transfer/domain、app | `TestClassify` 的格；报告的次序 | 记录与次序 |
| S3 | bootstrap | `TestAnImportReadsNamesNotInUTF8` | 真的导入 |
| S3 | transfer 各层 | 按人的上限的格（3.13.3） | 409、先后、回滚、上传 |
| S3 | bootstrap | `TestAPersonsSixthJobIsRefusedAndAnothersStarts` | 真库上按人 |
| S3 | transfer 各层 | 收拾的格（3.14.2）；`TestTheRescue` 改断言 | 丢弃与 `lost` 的次序 |
| S3 | bootstrap | `TestServeFailsTheJobsLeftRunning` 的三种排队行；3.14.3 的五处种子 | 真的启动；已有的测试照旧绿 |
| S3 | web | 4.2 的几格（六个测试文件；决定 482 另改 `transfer-background.test.tsx`） | 文字与按钮、连接中断的整句；`revertLeftGroup` 两种语言的整句 |

### 7.2 交错

`TestTheLogReadsOneMomentAcrossARevert` 是 P3 唯一的新交错（13.4 第 4 条）：它以 `LOCK TABLE` 把读停在快照里第一条碰 `page_revisions` 的语句上，在那时提交一条撤销，确定地造出"第一条读之后、后面的读之前"的时刻；不靠 sleep。在没改的代码上它的五格都红（3.8.3），实施时先看到它们红再改代码，写进第 10 节。

**前提**：五种读里，快照的第一条语句不碰 `page_revisions`，后面有一条碰。设计时读过 SQL：`ListChangesets`、`FindChangeset`（`queries/history.sql:3-23`）只读 `changesets`；碰的是 `ChangesetFacts`（`:63-71`），带路径的日志是 `ListChangesetsOfPage`（`:37-62`）。找路径的读（`NodePaths.Find`）只读 `notebooks`（`StateOf`）与 `nodes`（`RootByKey`、`ChildByKey`，`queries/nodes.sql:52-70`），不碰 `page_revisions`（审查 B 核对过，审查记录"核对过的"B 一条；修订核对 B 的 N15）。

事件流的互顶是确定的模拟（3.9.3 的 `TestClientsThatReconnectAtOnceComeToRest`），不是交错。

### 7.3 变异

每条先在没改的树上跑同一个命令看它绿，再改出变异看它红（13.4 第 1 条），命令与结果写进第 10 节。

**S1**

| # | 变异 | 应红的测试 |
|---|---|---|
| S1-1 | `TitleKey` 不换切罗基文 | `TestTitleKeyIsStable`、`TestSearchFoldIsTheTitleKeyNow` |
| S1-2 | `TitleKeyContext` 长的一支不换 | `title_key_context_test.go` 的长标题 |
| S1-3 | `IndexKeyer.Key` 不换 | linking 的两格 |
| S1-4 | 只换 U+AB70–ABBF 一段 | `TestTitleKeyIsStable` 的 `ᏸ` |
| S1-5 | `PlainText` 不写自动链接 | `parse_test.go`、`links_test.go` 的那一格 |
| S1-6 | `PlainText` 不写原始 HTML | 同上另一格 |
| S1-7 | `Tags` 照列 `/` | `TestTheTagsOfANotebook` |
| S1-8 | `Blank` 恒假 | `TestCheckName`、`TestCleanClientName`、显示名的一格 |
| S1-9 | `Blank` 漏掉 `Other_Default_Ignorable_Code_Point` | U+3164 的格 |
| S1-10 | `Blank` 把 U+2800 也算 | U+2800 的格 |
| S1-11 | `CheckName` 先判长度、再判 `Blank` | 300 个 U+200B 的格 |
| S1-12 | 凭证只看 `UserID`（按账户计） | `TestACredentialsFifthStreamSupersedesItsOldest`（另一会话受影响）、`TestClientsThatReconnectAtOnceComeToRest` |
| S1-13 | 顶掉的流算到它关掉为止 | `TestASupersededStreamLeavesTheCountsAtOnce` |
| S1-14 | 凭证满时先看账户 | `TestAFullAccountsCredentialSupersedesItsOwn` |
| S1-15 | 顶掉最新的一条 | `TestACredentialsFifthStreamSupersedesItsOldest`、整个程序的那一个 |
| S1-16 | `close` 不从 `held` 去掉 | `TestAnAccountsSixtyFifthStreamIsRefused` 的"关掉之后能开"；S1 实施：这个写法账户照减、仍能开，红在"全关之后两张表空"（另 `TestTheHubForgetsWhatClosed`） |
| S1-17 | `ErrTooManyStreams` 不带 `RetryDelay` | 处理器的一格 |
| S1-18 | `"/"` 照旧经 `ParsePath` | `TestListChangesetsAtPathListsAsExecute` 的新格、`TestTheMCPHistoryIsRESTs` |
| S1-19 | `""` 当作整本 | `TestListChangesetsAtPathAnswersItsCodesInOrder`、`historyCodeCalls` |
| S1-20–S1-23 | `Execute`、`AtPath`、`GetChangeset.Execute`、`InNotebook` 各去掉快照 | 单元的那一格；整个程序的那一格 |
| S1-24 | `GetChangeset.Execute` 的快照只包 `detail`（`FindChangeset` 在快照之外） | 单元的那一格（`FindChangeset` 不带标记）；整个程序的那一格照绿，写明原因（3.8.2） |
| S1-25 | `CleanClientName` 不判 `Blank` | `TestCleanClientName` 的新格 |
| S1-26 | 账户的硬上限不拒（或上限交成很大，00 号文档 9.2） | `TestAnAccountsSixtyFifthStreamIsRefused`（审查 B 的 M3） |
| S1-27 | `index` 的 `parent:"/"` 被拒 | `linking/app/list_index_test.go:325` 的 "the root" 一格（已有）；S1 实施时替身收 `"/"`，只由 `TestListIndexJudgesInTurn` 的调用次序抓到；第一次修复让替身照真的拒尾随的 `/` 之后，"the root"、"type and parent"、"a notebook hidden" 三格都抓到（3.21 第 13 条） |
| S1-28 | 超时的文字照旧 | 3.7 的三处整句比对 |
| S1-29 | `AtPath` 的快照从 `page` 才开始（`Find` 在外） | 单元的那一格（`StateOf`、`RootByKey` 不带标记）；整个程序的照绿（3.8.2） |
| S1-30 | `alivePaths` 在快照之后 | 单元的那一格（`PathsOf` 不带标记）；整个程序的照绿：SQL 写的 R 不碰 `nodes` |
| S1-31 | `Blank` 不减 `Prepended_Concatenation_Mark` | `TestCheckName` 的 `۝` 一格；`Blank` 逐码位的比对（审查 B 的 M1） |
| S1-32 | `run` 的提交错误不带标记（照旧 `fmt.Errorf`） | `TestACommitThatFailsIsMarked` 的 (a)、(b)（修订核对 B 的 N14） |
| S1-33 | 只读的快照的提交错误也带标记 | `TestACommitThatFailsIsMarked` 的 (c) |
| S1-34 | `deadlineText` 不看标记（恒为 `internalDeadline`） | `answer_test.go` 的提交一格 |
| x1–x11 | S1 实施者加的（下同）：`Blank` 不减 U+FFF9–FFFB、U+13430–13440；不算空白；先判控制字符；`close` 不删空项（两种）；原因写成 `overflow`（单元、整个程序）；`MaxAccountStreams = 65`；新的一条加在切片头上；`cherokeeCapitals` 只换第一个；`commitError` 没有 `Unwrap` | 都抓到；x1 只有逐码位的比对抓到 |
| b-m1-text、b-m1-tool、b-m1-method | `failedText` 不看标记；`toolError`、`methodError` 的 `FaultUnknown` 一支照旧（S1 第一次修复加的，下同） | `answer_test.go` 的 "a fault at a commit"、`prompt_bounds_test.go` 的 `unknown_commit` |
| b-m1-mark | `committing` 只看类型、不看 `TxCommitFailed()` 的值 | 等价：实现这个方法的只有 `commitError` 与测试的替身，都答 `true`（S1 的审查记录） |
| c-m1-pat、c-m1-opener | 去掉 `credentialOf` 的 PAT 一支；顶掉时减开者的账户 | `TestEachTokenIsACredentialOfItsOwn`；`TestASupersededStreamLeavesItsOwnAccount` |
| c-m1-job | 去掉 `credentialOf` 的 `job` 一支 | 等价：HTTP 的认证只造会话与 PAT 的 actor，任务经不到事件流 |
| a-m1-url | `PlainText` 写自动链接的 `URL` | 四个测试的 `www.` 一格 |
| c-c1-n2 | 顶掉时不经 `uncount`，不删 0 项（S1 修复第 2 轮） | `TestASupersededStreamLeavesItsOwnAccount` |

**S2**

| # | 变异 | 应红的测试 |
|---|---|---|
| S2-1 | 去掉新规则（`topOf` 照旧） | domain 的表、两个性质测试（`page.revert_conflict`）、整个程序的 nested 两格；S2 实施另有 `TestARestoreWaitsForTheGroupItGoesUnder`、`TestRevertGroupsAndRefs` 与翻过来的两格组合根的测试（3.22 第 1、2 条） |
| S2-2 | `topOf` 只看起点 | domain 表的第 4 格；`TestTheJobsStepsReplayToStar` 的 `nestedGroup` 试验 |
| S2-3 | `needed` 不看段内的放回 | `TestARestoreUnderAMemberCut…`；`TestTheJobsStepsReplayToStar` 的 `nestedCut` 试验 |
| S2-3b | `needed` 遇到段内后面的任何放回就整组放回（S2 审查 A 的 N5） | `TestARestoreUnderAMemberCut…`（成员 Q，3.22 第 11 条）；性质测试只核对结果，活下来 |
| S2-3c | 后面的放回挂在本组的成员下就整组放回（S2 审查 A 的 N6） | 同上 |
| S2-4 | `RevertGroups` 不传 `Nodes` | domain 表的 `RevertGroups` 断言；`TestRevertGroupsAndRefs` |
| S2-5 | `scheduled` 不传 `Nodes` | domain 表的放回的步；两个性质测试 |
| S2-6 | `CheckRevertPart` 不传 `Nodes` | `TestTheJobsStepsReplayToStar`（嵌套的根与外层同一批）；整个程序的 `nested, background` |
| S2-7 | `CheckRevertSteps` 不传 `Nodes` | 等价（审查 A 跑过，活下来）。证据：调用链 `treeSim`、`canPlace`、`canRestore`、`restore`、`added`、`canWrite` 不读 `c.top`、`c.tops`、`c.members`；对照：这一处改成 `readReverted(nil, …)`，任何一组的放回都让它 panic（没扩开的 `TestTheJobsStepsReplayToStar` 也崩），只证明这一处被调到；它在嵌套的形状上跑过，由扩开的测试的 `nested`、`nestedCut` 计数大于 0 证明，每一批都经 `CheckRevertSteps`（3.10.3；审查 A 的 M3，修订核对 A 的 N7） |
| S2-8 | `RevertActionFacts` 不传 `Nodes` | 等价（活下来）。证据：只在 `treeSim` 里读 `c.ids`；对照：去掉版本，`TestAContentOnlyRevertsFactsNeedNoTree` 红，证明这一处被调用；去掉条目的对照活下来（`c.ids` 也由版本组成），所以等价的依据是代码上的事实 |
| S2-9 | 性质测试的 `load` 不分时刻（测试自己的变异） | 冲突（`rootHolds`：外层那一项的计数把嵌套的根算了进去），各计数为 0（审查 A 的原型：`-short` 400 次里 33 次失败，在 `movegone` 与 `PlanRevert` 的 `page.revert_conflict`；审查 A 的 N4） |
| X1 | `world.state()` 照旧读所有时刻（S2 实施加的，下同） | domain 的表第 1、2 格起 |
| X2 | `del` 也吞并单独的项 | `TestTheJobsStepsReplayToStar`：嵌套的计数都是 0，idle |
| X3 | `folder` 不造 z | 同上：idle（嵌套几乎都来自 `folder`） |
| X4 | `done` 给单独的项 `p.at` | 同上：`PlanRevert` 冲突（同 S2-9） |
| X5 | 回滚的 `nested` 恒假 | `TestTheRollbacksStepsReplayToTheBaseline`：idle |
| X6、X7 | 回滚的撤销恒走请求、恒走后台 | 同上：另一条路的计数 idle |
| X8 | agent 的删除回到边走边删 | 测试自己的种子活下来，别的种子基上抓到（3.22 第 7 条） |
| B1 | 所有单独删的项同一个更早的时刻（S2 审查 B 的 B1） | `TestTheJobsStepsReplayToStar` 的 `-short`（加 w 之前活下来，3.22 第 12 条） |
| X10、X11 | `folder` 不造 w；`nestedDeep` 恒假 | 同上：`nestedDeep` idle |
| X12 | 改名的那一个核对两项的"早于"反过来 | 整个程序的两格 nested 停在核对上（3.22 第 13 条；X9 见那里） |
| 分份 | `Makefile` 只在一份换名字，或都不换 | 3.19 的脚本报 FAIL |

**S3**

| # | 变异 | 应红的测试 |
|---|---|---|
| S3-1 | 不看标志就按 GBK | 适配器表的第 8 格；差分 |
| S3-2 | 读 `NonUTF8` 而不读 `Flags` | 第 8 格 |
| S3-3 | 没有候选的包里合法的 UTF-8 名字也按 GBK 读 | 第 7、10 格；差分的"没有候选的包里无标志的非 ASCII 合法 UTF-8"（这几格与差分要 `NonUTF8: true` 才造得出，审查 C 的 I1） |
| S3-4 | GBK 的包里恰好合法的 UTF-8 名字不读（00 号文档原来的写法） | 第 6 格；差分的"GBK 的包里无标志、恰好合法的"；整个程序的 `图片/a.md` |
| S3-5 | 结果里有 U+FFFD 也回退 | 第 9、13 格；差分 |
| S3-6 | 一个候选解不开时其余的照样回退 | 第 9 格；差分 |
| S3-7 | 0x7075 的 CRC 不核 | 第 2 格；模糊的种子 |
| S3-8 | 按解读之前的名字判文件夹 | 第 11 格的 `资料/乗`（`乗.md` 抓不到，审查 C 的 M1） |
| S3-9 | 报告不写解读过的名字 | domain 的格；app 的格；整个程序的那一个 |
| S3-10 | 解读过的名字的记录排在别的问题之前 | app 的 1,100 条一格（`too_large` 被挤出去） |
| S3-11 | `room` 不按人数 | app 的四格；整个程序的那一个 |
| S3-12 | 按人的拒绝答成 `server_busy` | app、http 的格 |
| S3-13 | 回滚也被按人的上限拒绝 | `Rollbacks.Start` 的一格 |
| S3-14 | 正在上传的导入不按人记 | `Check` 与 `write` 的两格 |
| S3-15 | 先判全局、再判按人 | "两样都满"一格 |
| S3-16 | `AtStart` 不收拾排队的行 | `TestTheRescue`；模块的测试；`TestServeFailsTheJobsLeftRunning` 的 `waiting dropped` 与 `taken` |
| S3-17 | 某一种任务不设 `DiscardRunning` | River 适配器的五格之一；是导入时，模块的测试与 `TestServeFailsTheJobsLeftRunning` 的 `taken` 也红 |
| S3-18 | 收拾的任务排在五种任务之前 | 模块的次序一格；`TestServeFailsTheJobsLeftRunning` 的 `taken` |
| O-1 | 第一个 0x7075 无效时往后找（S3 实施加的，下同） | 模糊的种子 #4（第一个 CRC 错、第二个有效） |
| O-2 | 空的名字也取 | 模糊的种子 #6 |
| O-3 | 字段里不是 UTF-8 的名字也取 | 模糊的种子 #7 |
| O-4 | 版本不核 | 表第 3 格；差分；模糊的种子 #2 |
| O-5 | 字段与原名相同也记 `unicode_path` | 表第 4 格；差分 |
| O-6 | ASCII 的名字也经 GBK | 差分 |
| O-7 | `Read` 也收被忽略的 | domain 的格（`__MACOSX/…`） |
| O-8 | 读出的名字记成跳过 | app 的 1,100 条一格 |
| O-9 | `AtStart` 只收拾第一种 | `TestTheRescue` |
| O-10 | 别人正在上传的也算我的 | `TestTheUploadsCountAsTheirRowsAreWritten`；三个新测试里别人的格 |
| O-11 | 第六个也让过（`>` 代替 `>=`） | 同 S3-11 的 app 各格 |
| O-12 | `CountActive` 的 `mine` 不按人 | `TestThePersonsJobsCountAmongAll` |
| W-1 | `offeredOf` 不收 `transfer.too_many_jobs`（网页） | `trash-background.test.tsx` |
| W-2 | 码到文字的表去掉 `transfer.too_many_jobs` | 五个测试文件各一格 |
| W-3 | `name_gbk` 说成 `unicode_path` 的文字 | `transfer-report.test.ts` 两种语言 |
| B5 | 去掉 `len(field) < 5`（S3 审查 B；修复第 1 轮之前绿，下同） | 模糊的种子 #9（越界 panic） |
| B6 | `size > len(extra)-4` 改成 `size > len(extra)` | #10（越界 panic） |
| B7 | 越界时把尺寸截到余下的 | #10、#11（`full[:12]`，读出完整的 `测`） |
| B11 | `Special` 按原名结尾的 `\` 判 | 表第 11 格的 `资料/俓` |
| B12 | 读过的名字只在成功时加进报告 | `TestAnImportListsTheNamesReadOtherwiseLast` 的失败的导入 |
| MC-1 | 锁下的 `room` 不传人（S3 审查 C） | `TestAPersonsSixthBackgroundStartIsRefused` 的 "four of his, his fifth after the first count" |
| A-N4 | 启动时 `lost` 失败照样往下（S3 审查 A） | `TestTheRescue` 末尾一格 |
| A-N2 | 中文的 `transfer.revertLeftGroup` 放回后半句（S3 审查 A） | `transfer-report.test.ts` 的 zh-CN |

00 号文档 9.6 的 P3 三行点名的变异，在上面的对应（初稿写"都在上面"，其实缺了 S1 的三条，审查 B 的 M3）：

- P3/S1：切罗基文一步 S1-1；三处只改一处 S1-2、S1-3；`PlainText` 仍跳过自动链接 S1-5；`listTags` 列 `/` S1-7；日志或详情的读不在快照里 S1-20–S1-23；按账户而不按凭证数 S1-12；重置最新的那条 S1-15；账户的硬上限不拒 S1-26；先看账户再看凭证 S1-14；不认只由 U+200B、U+2060、U+3164 组成的名字 S1-8、S1-9；`index` 的 `parent:"/"` 被拒 S1-27；`log` 的 `""` 被当作整本 S1-19；超时的文字照旧 S1-28。
- P3/S2：去掉新规则 S2-1；`topOf` 走过回收站根 S2-2；`needed` 不看段内后面的放回 S2-3。
- P3/S3：不看标志就按 GBK S3-1；读 `NonUTF8` S3-2；合法的 UTF-8 名字也被解读，照 2.1 第 13 条成了 S3-3；有 U+FFFD 也回退 S3-5；一个解不开其余照样回退 S3-6；CRC 不核 S3-7；报告不写 S3-9；`room` 不按人数 S3-11；按人的拒绝答成 `server_busy` S3-12；`AtStart` 不收拾排队的行 S3-16。

### 7.4 闸门

每个提交：`go build ./...`、`go vet ./...`、`make lint`（`lint-go` 是整个模块；`lint-web` 含各包的类型检查）、`make test-go-short`；改契约的另 `make gen` 之后 `make gen-check`；改网页的另 `make knip`、`make test-web`（vitest）。

每个 Step 合并之前（06-environment 第 2 节，重的经 `heavy.sh`，之前 `df -h /`）：

- `make test` 全量（含组合根的八份与竞态的一份）；
- `make e2e` 全量（第 5 节）；
- 3.19 的分份脚本；
- 审查与修复核对之后再跑一次被改到的部分。

CI 绿之后才合并（02-workflow 第 4 节）；合并之后 `main` 上两次的分份用时照 06-environment 第 4 节记进第 10 节。

**守卫**：P3 不加计时或成本的守卫，没有一个界高于请求的期限（决定 359；三位审查都核对过）。S3 的第 1 个提交另把 3.14.3 点名的四个测试以 `-race -count=20` 跑一次（经 `heavy.sh`，约 2 分钟另加编译），看启动时的 `lost` 不与它们赛跑。

### 7.5 CI 的余量

| 份 | 今天（两次 `main` 的平均） | P3 加 | 之后 | 450 秒的余量 |
|---|---|---|---|---|
| 第七份 | 279.5 秒；落在 EPYC 7763 上约 315 秒 | 约 4–6 秒（四个新测试，3.19）；S1 的两个实测竞态共约 4.6 秒（10.1），S3 的两个约 4.4 秒（10.3） | 7763 上约 321 秒；照 S1、S3 的实测推约 324 秒（10.3） | 约 129 秒（按 7763 那一档，审查 C 的 N3）；照实测约 126 秒（推断） |
| 第八份 | 307 秒 | 约 2–4 秒（302 一格多一次回滚与撤销）；S2 实测竞态多 0.1 秒（10.2） | 约 311 秒 | 约 139 秒 |
| 竞态 | 见 06-environment 第 4 节 | 后台撤销的性质测试多约 0.2 秒、回滚的多约 1.3 秒（审查 A 实测与推算，3.10.9；S2 实测多 0.34–0.40 秒与约 1.5 秒，10.2）；S3 的差分（几组种子，每组 1,000 个 zip）与模糊的种子也在这一份（审查 C 的 N9），S3 实测差分 0.14 秒、表 0.04 秒、模糊的种子不到 0.01 秒（10.3） | S2、S3 各量一次 | S2、S3 量；S3 的分支 CI 上竞态一份 9:04–9:08（10.3） |

数是推断。每个新测试在实施时以 `go test -run '^Name$' -count=1 -v ./internal/bootstrap` 量一次（经 `heavy.sh`），写进第 10 节；竞态下的两个性质测试在 S2、S3 的差分与模糊在 S3 以 `make test-go-race` 的命令量（审查 A 的 13.79–14.09 秒与 1.83–1.85 秒是对照）。合并之后哪一份过了 400 秒，先挪给邻份：第七份的点名去掉之后 `TestA…` 回第一份、`TestTheL…` 回第三份，不提高 `-timeout`。

## 8. 完成的标准

每个 Step：

1. 第 6 节的提交都在，每个单独为绿；第 7.4 节的闸门都绿；CI 绿。
2. 第 7.3 节这个 Step 的每一条变异先绿后红，等价的写明理由；命令与结果在第 10 节。
3. 设计审查与修复核对（02-workflow 第 2、3 节）；对设计的修正写进 3.21–3.23。
4. 文档的改动随 Step 的最后一个提交（下面）；第 10 节写结果。

S2 另：3.10.7 的每个计数的下限在测试里、实测的最少次数在第 10 节；整个程序的 nested 两格在 `main` 上绿。S2：23 个下限在测试里，最少次数在 10.2 的表；两格在分支的 CI 上绿，`main` 上合并之后看（00 号文档第 13 节）。

S3 另：3.14.3 的六处种子都改了，每处的反向对照跑过（第 4 条的不确定，照实记跑了几次、红了几次）；3.14.3 点名的四个测试的 `-count=20` 写进第 10 节。S3 照此做了：第 4 条 5 次 4 红，四个测试 80 次全过（10.3）。

**文档的新文字**（各 Step 的文档提交用脚本逐条替换，断言旧文字恰好出现一次，02-workflow 第 1 节第 8 步；总体设计每改一次，第 15 节加一行，日期、变更、依据、`M12/P3/S<m>`）：

1. **总体设计 13.1 第 28 条**（S1）：`NFC → 完整的大小写折叠（\`cases.Fold\`，与语言无关）→ NFC。` → `NFC → 完整的大小写折叠（\`cases.Fold\`，与语言无关）→ NFC → 切罗基文的小写换成大写（\`cases.Fold\` 把切罗基文的大小写对调，这一步让键幂等；链接索引的 \`IndexKeyer\` 与搜索的分词同一步，M12/P3/S1）。`S1 的文档提交照它落了。
2. **总体设计 13.1 第 30 条**（S1）：在 `   - **连接的建立**` 那一项之后加一项：`   - **流数**（M12/P3/S1）：每个凭证（登录会话、PAT、任务）至多 4 条，第 5 条顶掉它最老的一条（\`reset\`，原因 \`superseded\`），从不被拒；每个账户至多 64 条，净增的第 65 条答 429 \`rate_limited\`，\`Retry-After: 30\`。两个上限是常量；条数在打开、顶掉与关闭时维护，顶掉的那一条立刻不算。`S1 的文档提交照它落了。
3. **总体设计 5.8 第 6 步**（S2）：照 00 号文档"总体设计的修订"第 8 条的第一项。S2 的文档提交照它落了。
4. **总体设计 13.1 第 2 条**（S2）：从 `回滚的变更集由几遍、几个单元在不同时刻写下` 起、到 `没有造过。` 为止（`v0.1-design.md:1062`）换成：`回滚的变更集由几遍、几个单元在不同时刻写下：先删的子树在自己的单元里删成回收站根，收着基线节点的新建节点在后面一遍才删。撤销计划里，被这个变更集删了、现在本身是回收站根的节点是它自己的顶，几项依次放回（外层先）；后台撤销在一段"让开"里放回外层一组时，至少放回到段内后面的放回要用的成员（M11 第 302 条，M12/P3/S2 修，[M12 总设计](M12-release/00-M12-design.md) 4.6.1）。`S2 的文档提交照它落了。
5. **`README.md:192`**（S3）：`不能安全导入的条目（越出目录、符号链接、加密、压缩比过大、名称不是 UTF-8、过深、过大等）跳过，报告逐条列出。` → `不能安全导入的条目（越出目录、符号链接、加密、压缩比过大、过深、过大等）跳过，报告逐条列出。没有 UTF-8 标志的名称先读 Info-ZIP 的 Unicode 路径；包里不是 UTF-8 的名称都能按 GBK（中文 Windows 打包的写法）读出时，按 GBK 读，报告列出每个这样读的名称；这样的包里，没有标志、恰好也是合法 UTF-8 的非 ASCII 名称一并按 GBK 读（GBK 的\`目录\`、\`图片\`就是合法的 UTF-8）。有一个读不出时，这个包不按 GBK 读，不是 UTF-8 的条目跳过。已知限制：日文、韩文、繁体中文的旧编码会被读成乱码的中文；同一个包里既有 GBK 名称、又有没有标志的 UTF-8 名称时，后者也会被读成 GBK。`（审查 C 的 N7）S3 的文档提交照它落了。
6. **`README.md:193`**（S3），两处：
   - `全部排队与运行中的任务至多 \`transfer.max_queued\`（默认 20），超出答 503 \`server_busy\`；` → `一个人排队与运行中的任务（各笔记本合计，含正在上传的导入）至多 5 个，超出答 409 \`transfer.too_many_jobs\`（升级的回滚算进去，但从不因此被拒）；全部排队与运行中的任务至多 \`transfer.max_queued\`（默认 20），超出答 503 \`server_busy\`；`
   - 从 `进程恰在 River 取到任务、任务开始之前停掉时，任务一直排队，` 到 `开始它的人与笔记本的管理员可以取消它；`（各恰好出现一次）换成 `进程恰在 River 取到任务、任务开始之前停掉时，任务在服务下次启动时记成失败 \`interrupted\`（M12）。开始它的人与笔记本的管理员可以取消排队的任务；`（其后的"升级的回滚只有…"一句不动）。S3 的文档提交照它落了。
7. **`README.md:198`**（S1）：`一个账户一条：` → `一个浏览器一条（每个凭证至多 4 条、每个账户至多 64 条，见下面"流数"）：`S1 的文档提交照它落了。
8. **`README.md:207`**（S1），两处：`、\`overflow\`（客户端读得太慢，缓冲的 64 个事件满了）。` → `、\`overflow\`（客户端读得太慢，缓冲的 64 个事件满了）、\`superseded\`（同一个凭证开了第 5 条，这是它最老的一条）。`；`因 \`access\`、\`notebooks_deleted\`、\`reconnected\`、\`overflow\` 而 \`reset\` 时，之前已经送到这条流的事件先写出` → `因 \`access\`、\`notebooks_deleted\`、\`reconnected\`、\`overflow\`、\`superseded\` 而 \`reset\` 时，之前已经送到这条流的事件先写出`（审查 B 的 N9）。S1 的文档提交照它落了。
9. **`README.md:210`** 之后（S1）加一条：`- **流数**：每个凭证（登录会话、PAT）至多 4 条，第 5 条让它最老的一条以 \`reset\`（\`superseded\`）结束；每个账户至多 64 条，再开答 429 \`rate_limited\`（\`Retry-After: 30\`）。网页一个浏览器只开一条（持有它的标签页转给其余的）。`S1 的文档提交照它落了。
10. **`README.md:223`**（S1）：`可按 \`path\`、` → `可按 \`path\`（\`"/"\` 与不给相同，是整本；\`""\` 答字段错误）、`S1 的文档提交照它落了。
11. **00 号文档**：Phase 设计合并时照 2.1 改（脚本 `apply_p3_design.py`，02-workflow 第 2 节第 3 步）；第 12 节的进度随每个 Step。
12. **总体设计 13.1 第 4 条**（S3；审查 C 的 M3、I2）：`409 \`transfer.busy\` → 503 → 事务里再判定、写行，没有 422；` → `409 \`transfer.busy\` → 409 \`transfer.too_many_jobs\`（M12）→ 503 → 事务里再判定、写行，没有 422；`；`409 \`page.locked\`（计划要删或写回的页）→ 503，与 \`revertChangeset\` 共用这一遍核对（[M8 总设计](M8-history-search/00-M8-design.md) 4.14）。` → `409 \`page.locked\`（计划要删或写回的页）→ 409 \`transfer.too_many_jobs\`（M12）→ 503，与 \`revertChangeset\` 共用这一遍核对（[M8 总设计](M8-history-search/00-M8-design.md) 4.14）。M12/P3/S3 的按人的上限在导出与导入里：\`startExport\` 是 409 \`transfer.too_many_jobs\` → 503 \`server_busy\` → 409 \`transfer.busy\`，\`startImport\` 在读文件之前是 409 \`transfer.busy\` → 409 \`transfer.too_many_jobs\` → 503。`S3 的文档提交照它落了。
13. **总体设计 13.1 第 23 条**（S3；审查 C 的 M3）：`模块在 River 取任务之前的恢复经 \`jobs.Job.Start\`（导出、导入与 M8/P4 的恢复、撤销的收拾：上次进程留下的"运行中"一律记成中断）。` → `模块在 River 取任务之前的恢复经 \`jobs.Job.Start\`（导出、导入与 M8/P4 的恢复、撤销的收拾：上次进程留下的"运行中"一律记成中断）。M12/P3/S3 起 transfer 的五种任务也设 \`jobs.Job.DiscardRunning\`，平台在收拾的 \`Start\` 之前丢掉上一个进程留下的 \`running\` 的 River 行，收拾随即把 River 不再持有的排队行记成中断：上一个进程取到而没开始的任务不再等 River 的救援；以 SQL 种排队行的测试要同时种它的 River 行，应用起来之后种的写在同一条语句里。`S3 的文档提交照它落了（末句的后半照 S3 审查 A 的 N3 补，3.23 第 13 条）。

第 1、2、4–10、12、13 条的旧文字在 `86565dd6` 上以脚本核对过，各恰好出现一次（修订时）；第 3 条的旧文字 00 号文档已核对过。实施时在当时的 `main` 上再核对一次，不唯一就带上前后文。S3 的第 5、6、12、13 条在 `4136991c` 上由 S3 的文档脚本再核对过，各恰好出现一次。

## 9. 移交

| 给谁 | 什么 |
|---|---|
| P4/S3（性能实测） | 日志与变更集的详情改在只读的 REPEATABLE READ 快照里读（3.8），每次多一个事务的开销与一条连接的占用；日志的实测照新的读法量。事件流的 hub 多两张表，打开与关闭多一次常数时间的维护 |
| P5/S3（发布说明的已知限制，4.13） | **切罗基文的标题键**（审查 B 的 M2，照代码）：(1) 升级之后不跑 `reindex` 时，已有的切罗基文标题留着旧键，按路径找（`RootByKey`、`ChildByKey`）与链接解析都找不到它（MCP 读 `Ꭰ.md` 答 `page.not_found`），同一父页下还能再建一个同名或只差切罗基文大小写的兄弟页（S1 的探针：旧键是对调的大小写，新键不撞它）；(2) `nervewiki reindex` 重算标题键，把它们改对；(3) 一本笔记本里已有只差切罗基文大小写的兄弟页（旧键下合法），或升级之后、`reindex` 之前建成的同名兄弟页时，`reindex` 报撞键、这本整个跳过（链接也不重建），要先给其中一页改名。发布说明写：有切罗基文标题的实例升级之后跑一次 `nervewiki reindex`，报撞键的照提示改名再跑。<br>**标题 id**：标题里有自动链接或原始 HTML 的，id 变了，旧的指向它的锚点链接要重写。<br>**导入名称**：Shift-JIS、Big5、EUC-KR 的名字读成乱码的中文；GB18030 的四字节字符让整包不按 GBK 读；只有恰好合法 UTF-8 的 GBK 名字的包认不出；**相对今天的回归**：GBK 的包里没有标志的真 UTF-8 名字会被读成 GBK（P3-16），报告里逐个列出。S3 的适配器的表第 7、10、12、13 格各钉一样，改了它们的人要同时改发布说明（10.3）。<br>**其余**：U+2800 单独作名字仍然通过；一个凭证至多 4 条事件流，一个账户至多 64 条、再开答 429（脚本或很多个配置文件才会碰到） |
| v0.2 的 `M12-transfer.md`（收尾时写） | 导入时选编码的选项；CP437；Shift-JIS 等的识别；按人的上限做成配置项（若有人要） |
| v0.2 的 `M12-history.md`（收尾时写） | "同一形状在后台恢复、后台撤销里"仍没有造过（00 号文档 4.6.1"不去造的"）：任务自己的变更集分批删出的那一种；交错 20 的"往回"与后台恢复停在组中间的那一例是来源 3，S2 照常撤销（3.22 第 1 条） |

切罗基文的那一条是审查 B 照代码核对过的（`linking/app/reindex.go:106-109`、`page/link_targets.go:252-274`、`page/domain/rekey.go:24`），不再留给 S1 核对；不加迁移（00 号文档 4.6.3）。S1 实施另以 go1.27.4、x/text v0.42.0 的探针核对旧键（U+13A0 的旧键是 U+AB70，反之亦然）：只含小写的标题旧键已是大写、照旧找得到，所以 (1)、(3) 加了"同名"的一种（审查 A 核实）。

## 10. 结果

每个 Step 合并之前在这里写它的实施、审查与验证的结果；合并的提交见 00 号文档第 12 节。

### 10.1 S1：小的正确性（2026-11-16，合并见 00 号文档第 12 节）

- **提交**（从 `main` 的 `48bbfc99` 开始，实施 8 个，照 6.1 的次序；文档是本提交）：
  - 标题键：切罗基文的小写换成大写（`shared.CherokeeCapital`；`TitleKey`、`TitleKeyContext` 末尾一步，不合法的字节原样留着；`IndexKeyer.Key` 原地换）；`searchFold` 删掉，搜索与 grep 的键改调 `shared.TitleKey`，与 `194ca350` 的 `searchFold` 逐码位相同，分词的版本不升（`c3eade4e`）。
  - 标题 id 带上自动链接的标签与原始 HTML 的源文本，与锚点相遇（`76a159bc`）。
  - 标签列表不列 `/`（`4e813da1`）。
  - 名字：`shared.Blank` 照 Unicode 的推导；`CheckName` 先判空；`CleanClientName` 什么也不显示时是 `""`（`0f0d7ea5`）。
  - MCP 的 `log`：`"/"` 是整本，`""` 答 `too_short`（`3b7e7db9`）。
  - MCP 调用超时的两句文字；读写事务的提交错误带 `TxCommitFailed` 标记（`a89ee212`）。
  - 日志与变更集的详情各在一个只读的 REPEATABLE READ 快照里，从答复的第一条读到 `alivePaths`；交错 `TestTheLogReadsOneMomentAcrossARevert`（`81b38143`）。
  - 事件流：每个凭证 4 条、第 5 条顶掉最老的一条（`superseded`），每个账户 64 条、再开答 429；`TestASignInsFifthStreamSupersedesItsOldest`（`2200b5d6`）。
  - 每个提交在干净的 worktree 里 build、vet、lint、`test-go-short`、`gen-check`、knip 都绿，改了网页生成物的两个另 `test-web` 绿；审查 C 在自己的工作树里逐个再跑，7、8 号另跑分份（555、556 个）。
- **与设计不同之处**（3.21 列出）：`searchFold` 的调用多一处；切罗基文的一步不用 `strings.Map`；`render_test.go` 那一格用 Markdown 链接；`IndexKeyer` 的两格是一个新测试；快照碰到已有的单元测试与 `TestVirtualRead/no_nested`；流数的整个程序测试用新的登录；hub 的测试在新文件；分份脚本"去掉 `$$`"的对照不会失败；处理器的 429 一格先看状态；修复带来的 3.21 第 11–14 条。
- **先红后绿**：交错的五格在没有快照时都红（C 被 R 撤销、R 没列出；审查 B 另把快照降成 READ COMMITTED，五格也都红）；`TestACommitThatFailsIsMarked` 的 (a)、(b) 在没有标记时红，(c) 绿；标签的新格、`parse_test` 与 `links_test` 里标题 id 的新格在没改的代码上红，`render_test`、`view_test` 两格守锚点一侧，没改时也绿。
- **反向对照**（7.3）：S1-1 到 S1-34 的 45 条（其中 11 个编号另有整个程序的一格）与实施者加的 x1–x11，共 56 条（轻的 40、重的 16），在头 `2200b5d6` 的干净临时 worktree 上、每张表先在没改的树上把每个命令跑一遍。轻的全抓到；重的 13 条抓到，S1-24、S1-29、S1-30 的整个程序那一格照绿，同 3.8.2 的预期，单元的那一格抓到。S1-16 红在"全关之后两张表空"（这个写法账户照减）；S1-27 原来只由 `TestListIndexJudgesInTurn` 的调用次序抓到（替身的 `Check` 收 `"/"`），第一次修复让替身照真的拒尾随的 `/` 之后，"the root"、"type and parent"、"a notebook hidden" 三格都红。x1–x11 全抓到；x1（`Blank` 不减 U+FFF9–FFFB、U+13430–13440）只有逐码位的比对抓得到。写代码时的一条发现：处理器的 429 一格答 200 时读到流的末尾，S1-12、x7 下挂到超时，提交之前改了（3.21 第 10 条）。
- **实测**（本机 8 核 arm64，经 `heavy.sh`，头 `2200b5d6`）：`TestTheLogReadsOneMomentAcrossARevert` 1.46 秒（`-race` 2.39 秒；设计推 1–2 秒），`TestASignInsFifthStreamSupersedesItsOldest` 1.53 秒（`-race` 2.19 秒；设计推约 1 秒）。第七份竞态下约多 4.6 秒，在 7.5 的"约 4–6 秒"之内。合并 `main` 之后（`d1e90929`）竞态下各一次：2.51、1.23 秒。
- **门禁**（本机）：头 `2200b5d6` 与修复第 1 轮的头 `1f1d3d11` 各一轮：`make lint gen-check knip build` 绿；`make test` 绿（组合根八份、竞态、成本、守卫，web 4537 个；3538、3536 秒，含等锁）；`make test-go-short` 绿；`make e2e` 577 个（576 个通过、1 个跳过：WebKit 的 A15 第 3 格）；分份脚本 556 个，145/50/119/120/60/15/32/15，两个新测试在第七份。修复第 2 轮的头 `a7723228`：`make lint gen-check knip build`、`make test-go-short` 绿，本地不跑 `make test` 与 `make e2e`（决定 486）。合并 `main` 之后的 `d1e90929`：`make lint gen-check knip build`、`make test-go-short` 绿，分份 556 个不变。
- **分支的 CI**（`ci.yml` 每次 19 项第一次 attempt 全绿，没有重跑，没有碰到已知的偶发；Test 与 E2E 是那一步的用时）：

  | 头 | run | 第七份 | 竞态一份 | E2E 一步 |
  |---|---|---|---|---|
  | `2200b5d6`（实施） | 43778049280 | 4:03（EPYC 9V45） | 8:50 | 8:52（Playwright 8.6 分钟） |
  | `1f1d3d11`（修复第 1 轮） | 43793058599 | 4:29（EPYC 9V74） | 10:57 | 6:50 |
  | `a7723228`（修复第 2 轮） | 43801996056 | 4:00（Xeon 6975P-C） | 8:23 | 7:34 |
  | `d1e90929`（合并 `main`） | 43814516991 | 3:54（Xeon 6975P-C） | 12:33 | 6:17 |

  八份都在 450 秒之下。第一次的 e2e 比 `main`（`99def961` 的 6.1 分钟）整体慢，最慢的故事都慢 1.4–2 倍，作业没有 CPU 一步；之后三次 6.1–7.4 分钟，没有再现。第七份这几次都没落在 EPYC 7763 上；合并之后两次 `main` 照 06-environment 第 4 节记进 00 号文档第 13 节。文档提交的分支 CI：`79e3b884` 的 run 43819949305，第 1 次 attempt 全绿；第七份 4:26、竞态一份 11:12，E2E 一步 7:34。合并 `f3e5a6de` 之后第一次 `main`：run 43823936850，第 1 次 attempt 全绿；第七份 4:09、竞态一份 9:10，E2E 一步 6:56。
- **审查**：三份（A 标题键、标题 id、标签、名字、`log` 的 `path` 0I 2M 2N；B MCP 超时的文字、提交错误的标记与快照 0I 1M 1N；C 事件流、契约与网页、分份、提交与文档 0I 2M 3N）。见 [S1 的审查记录](reviews/P3-S1-small-correctness-review.md)。
- **修复第 1 轮** 6 个提交（`ac30ec3d`–`1f1d3d11`）：MCP 的 `FaultUnknown` 一支也认提交的标记，带标记的答 `the server failed as its changes were being saved, so they may or may not have been; read the notebook to see whether they were before retrying it`（B-M1，决定 483），`tx.go` 的注释（B-N1）；`TestEachTokenIsACredentialOfItsOwn` 与 `TestASupersededStreamLeavesItsOwnAccount`，顶掉时减被顶掉那一条自己的账户（C-M1，决定 484）；标题 id 的 `www.` 自动链接四格（A-M1）；linking 的替身照真的拒尾随的 `/`（A-M2）；`make help` 的第七份（C-N1）；网页三处注释（C-N3，决定 485）。修复的变异 23 条：审查要的 8 条里 6 条抓到，`committing` 不看方法的值（b-m1-mark）与去掉 `job` 一支（c-m1-job）两条等价（证据在记录的"反向对照"）；重跑的 S1 原有的 15 条全抓到，S1-27 现在由 "the root" 等三格抓到。逐个提交单独绿；门禁全绿。
- **核对第 1 轮**：三位的旧发现都解决，新的 0 Important、0 Minor、4 条 Nit。
- **修复第 2 轮** 4 个提交（`1812134f`–`a7723228`），只补那 4 条 Nit：linking 的替身对 `"/"` 答 `required`（A-C1-N1）；`internalFailedSaving` 的注释（B-C1-N1）；hub 的 `uncount`，被顶到 0 的账户也删项，新断言先红后绿（C-C1-N2）；网页注释写 429 的两种起因（C-C1-N1）。跑了事件流三个包竞态 `-count=3`、`linking/app` 与 `mcpserver`，变异 16 条（hub 的、S1-27 与新的 c-c1-n2）全抓到，逐个提交单独绿。由主持者自己核对（决定 486）：读了 diff，与说明的处置一致；新断言在没改的代码上红；变异全红；分支 CI 跑全套、全绿。
- **合并 `main`**：`d1e90929` 合进 P2、P4、P5 的设计与 P3/S2，冲突只在 `Makefile`（`SEVENTH` 留 S1 的两个名字、取 S2 的改名；计数取 `main` 的说明、接 S1 的括注），之后分份 556 个、145/50/119/120/60/15/32/15，S1 的两个在第七份、S2 改名的只在第八份；`892dd827` 合进 `41c5ac25`（S2 的合并号），没有冲突。

### 10.2 S2：第 302 条（2026-11-16，合并见 00 号文档第 12 节）

**提交**（分支 `m12-p3-s2`，从 `48bbfc99`；6.2 的第 1–5 个，修复第 1 轮四个，文档是第 10 个，即这一个）：

| # | sha | 说明 |
|---|---|---|
| 1 | `de2de0ae` | domain 测试的 `world` 照根的时刻读组（P3-3） |
| 2 | `ba65144f` | 规则：`topOf` 停在现在本身是回收站根的节点，五处构造计划传 `Nodes`；domain 的表（十二格）、`TestRevertGroupsAndRefs` 的一段；整个程序的 nested 拆成请求与后台两格翻过来、改名 `TestRevertingARollbackPutsBackWhatTheAgentLeft`；`Makefile` 两个模式换名字；两格组合根的测试照决定 304 翻过来（3.22 第 1 条） |
| 3 | `e3725b07` | `needed` 也看段内后面的放回；分时刻的装载与 `delApart`；`TestARestoreUnderAMemberCutShortInASegmentBringsItBack` |
| 4 | `f6e54ead` | `TestTheJobsStepsReplayToStar` 扩开：`delApart`、`folder` 的 z、四个计数与下限 |
| 5 | `a714787f` | `TestTheRollbacksStepsReplayToTheBaseline` 扩开：撤销回滚的变更集，请求与后台轮流；agent 的删除先读整棵；三个计数与下限 |
| 6 | `9991aee1` | 修复第 1 轮：确定的一格加成员 Q（3.22 第 11 条） |
| 7 | `99abd703` | 修复第 1 轮：改名的那一个的两格 nested 先核对回收站里的两项（3.22 第 13 条） |
| 8 | `46de0b3e` | 修复第 1 轮：`folder` 的 w、`nestedDeep` 与下限；全部下限照约定重算（3.22 第 10、12 条） |
| 9 | `63cfcd9b` | 修复第 1 轮：回滚的性质测试的说明写上两条路的下限（3.22 第 8 条） |

第 2–5 个推送之前重做过一次：第一次的头 `737b15e7` 的 `make test` 红在 3.22 第 1 条的两格，翻转放进第 2 个提交，再 `cherry-pick` 第 3–5 个（与旧头只差这两个文件）；旧头留在本地分支 `m12-p3-s2-before-flip`，没有推。

**每个提交单独为绿**：在一个临时 worktree 里逐个 `git checkout --detach`，build、vet、`make lint`、`make test-go-short` 都是 0；第 1、2 个另跑 page 的 domain 全量与整个程序的那一个（第 1 个是旧名字、答 409，第 2 个是新名字、200 与后台成功），第 2 个另跑 3.19 的分份脚本，并在第 2 个提交的树上另跑翻过来的两格连同改名的那一个（三个测试、十八个子测试都过；实施者非竞态，审查 C 竞态，审查 C 的 N4）。审查 C 另在第 2–4 个上跑 page 的非短测试，都过。修复的第 6–9 个同样逐个跑：build、vet、`make lint`、`make test-go-short` 都是 0，另在每一个上跑改名的那一个（竞态，三格都过）与后台撤销、回滚的两个性质测试和确定的一格的全量（都过）。

**`needed` 先红后绿**（6.2 第 3 条）：第 4 个提交的生成器放在 `needed` 改之前，`-short` 400 次里 17 次失败，都是模型的 "a restore under a parent gone"，第一个是种子 34（审查 A 的原型也是 17 次）；改后 0 次。确定的一格在改之前三处都红：整串步里 P1 在段后、模型第 4 步失败、批切在段尾时核对的替身记下 "z under P1, not alive"。加成员 Q 之前，S2-3b、S2-3c 在这一格上活下来（审查 A），加了之后红。

**下限**（3.10.7；本机，经 `heavy.sh`；后台撤销的那个是修复第 1 轮加了 w 之后的生成器，`46de0b3e`）：

| 计数 | 40 个基 × 400 次的最少 | 20 个基 × 2,000 次的最少 | `-short` 下限（之前） | 全量下限（之前） |
|---|---|---|---|---|
| `aside` | 206 | 1,141 | 137（138） | 760（741） |
| `restoreWrite` | 217 | 1,257 | 144（157） | 838（836） |
| `writeDelete` | 129 | 774 | 86 | 516（512） |
| `cut` | 39 | 270 | 26 | 180（176） |
| `across` | 149 | 825 | 99（100） | 550（548） |
| `goneLater` | 14 | 106 | 9（11） | 70（73） |
| `goneSame` | 30 | 189 | 20（22） | 126 |
| 注入的改名 | 120 | 651 | 80（78） | 434（427） |
| 注入的占名 | 86 | 580 | 57（66） | 386（364） |
| 注入的子节点 | 63 | 355 | 42（40） | 236（235） |
| 注入的放置的父节点删除 | 14 | 93 | 9（8） | 62（58） |
| 注入的环 | 6 | 46 | 4（3） | 30（28） |
| 注入的放置太深 | 15 | 125 | 10（11） | 83（78） |
| 注入的恢复太深 | 9 | 67 | 6（5） | 44（41） |
| 注入的恢复的父节点删除 | 13 | 100 | 8（9） | 66 |
| `nested` | 25 | 172 | 16 | 114 |
| `nestedMid` | 22 | 162 | 14 | 108 |
| `nestedGroup` | 11 | 73 | 7 | 48 |
| `nestedCut` | 11 | 88 | 7（6） | 58 |
| `nestedDeep`（修复第 1 轮新加） | 9 | 72 | 6 | 48 |
| 回滚：`nested` | 13 | 92 | 8 | 61 |
| 回滚：请求的撤销 | 198 | 998 | 132 | 665 |
| 回滚：后台的撤销 | 199 | 998 | 132 | 665 |

60 组都没有失败（回滚的那个在 agent 改了之后）。嵌套的前三个的最少次数与加 w 之前相同：它们在 w 那一次抽签与 w 的 id 之前就定了（有没有 z、z 在哪、有没有 zz）；`nestedCut` 由 9、87 变为 11、88。审查 B 在 `a714787f` 上把加 w 之前的数逐字重量过，核对第 1 轮又从两份日志逐个重算了 40 个下限，0 处不符。下限都是最少次数的三分之二、向下取整（决定 479）；`-short` 的最小是 4（`cycle`）。测试自己的基 0：`-short` 下五个嵌套的计数 33/31/18/21/15，全量 194/176/92/105/83。

**量下限时找到的**：回滚的性质测试的 agent 删新子树时边走边删，往上的路经过已删的节点就读不到，留下"活着、父节点不在"的节点（56,000 次里 15 次）。在 `48bbfc99` 上原样复现（种子 10334、37050、37135、201121、600695、801293、1601237 在回滚的第一遍失败；另 8 个回滚照过，撤销答 `page.revert_conflict`，诊断过的 34196 是 `parent_gone`，审查 B 核对 8 个都是）。测试自己的 2,000 个种子没有一个留下孤儿。改为先读整棵再删（3.22 第 7 条）。

**变异**（7.3；`mut.py`，每条先在没改的树上跑同一命令为绿）：S2-1 到 S2-9、X1–X8 与分份的对照，第一次在没推的头 `737b15e7` 上做（与 `a714787f` 的非测试代码与这些命令用到的测试逐字相同，只差两个组合根的测试文件；审查 C 的 N3）；那时的命令里没有翻过来的两格，审查 C 补跑了 S2-1 对它们（四格红）。修复第 1 轮在 `63cfcd9b` 上整个重跑，结果与第一次逐条相同（S2-3 在回滚的测试与整个程序上照旧活下来，S2-7、S2-8 等价，X8 只在别的种子基上抓到）；另加的：S2-3b、S2-3c 由确定的一格抓到，S2-3b 在后台撤销的性质测试上活下来（它只核对结果）；B1（单独的项同一时刻）在 `-short` 上红；X10（`folder` 不造 w）、X11（`nestedDeep` 恒假）以 `nestedDeep` 的 idle 红（X10 照样抽签，但少了 w 的 id 的几次随机数，别的计数小有变动，只有 `nestedDeep` 报 idle；核对第 1 轮 B 的 C1-N1）；X12（改名的那一个核对两项的先后反过来）红在两格 nested；X9 见 3.22 第 13 条。逐条的表在 [S2 的审查记录](reviews/P3-S2-nested-trash-roots-review.md)"反向对照"。

**门禁**（7.4，经 `heavy.sh`）：头 `a714787f`：`make lint gen-check knip build`、`make test`（含八份与竞态一份）、`make test-go-short`、`make e2e`（576 个通过、1 个跳过）都是 0；第一次的头 `737b15e7` 的 `make test` 红在 3.22 第 1 条的两格，翻过来之后重做第 2–5 个提交、整个重跑。修复第 1 轮的头 `63cfcd9b`：同样四步都是 0（`make test` 185 个包 ok、没有 `FAIL`；e2e 576 个通过、1 个跳过）。

**分份**（3.19）：第 2 个提交与头上都是 554 个顶层，145/50/119/120/60/15/30/15，每个恰在一份；改名的只在第八份，旧名字不在列表里；`TestTheRevertOfEachBatchShape` 在第三份，`TestABackgroundRevertStoppedInAGroup` 在第七份。审查 C 以自己读 `Makefile` 配方的脚本另核，结果相同；两个模式都留旧名字时新名字落到第四份，只有第七份留旧名字时它在第四、第八两份，脚本都报错。

**实测**（7.5；本机，经 `heavy.sh`，同一台机器上与 `48bbfc99` 交替；负载在括号里）：整个程序的那一格竞态 2.90 → 3.00 秒（非竞态 1.79 → 2.16；负载 7–8）。两个性质测试照 `make test-go-race` 的跑法，在修复第 1 轮的头 `63cfcd9b` 上与 `48bbfc99` 交替三轮（负载 1.5–3.9）：`TestTheJobsStepsReplayToStar` 13.56–13.60 → 13.94–13.98 秒，多 0.34–0.40 秒（w 与 `nestedDeep` 在内；审查 A 的原型多约 0.2 秒）；`TestTheRollbacksStepsReplayToTheBaseline` 1.80–1.82 → 3.27–3.31 秒，多约 1.5 秒（审查 A 推约 1.3 秒）。（初稿的"S2 的生成器扩开了、多 1.0–2.0 秒"不对：审查 A 的原型已有扩开的生成器；那两轮在负载 3.3–6.5 时量的，偏大。审查 B 在负载低的三轮量的是 13.61–14.39 → 13.95–14.70 秒，多 0.27–0.36 秒，回滚的多约 1.5 秒；审查 B 的 M2。核对第 1 轮 B 在负载 1.1–1.5 时另量两轮，整台机器偏慢、基线 15.4 秒，多 0.20 与 0.42 秒、回滚多 1.65 与 1.67 秒，量级相符。）

**分支的 CI**：`a714787f` 的 run 43772091351，第 1 次 attempt 全绿，没有重跑。Test 一步：第八份（改名的那一格）4:03（EPYC 9V45），第七份（交错 20）5:07（EPYC 7763），第三份（种下的嵌套一格）4:31（Xeon 6975P-C），竞态一份 13:47（Xeon 8370C）；E2E 一步 7:31（审查 C 的 N2：照 Test 一步写，06-environment 第 4 节按它判份的余量）。修复第 1 轮的头 `63cfcd9b`：run 43800625515，第 1 次 attempt 全绿，没有重跑；Test 一步第八份 3:59（EPYC 9V45），第七份 5:07（EPYC 7763），第三份 6:05（EPYC 7763），竞态一份 8:59（EPYC 9V45）；E2E 一步 5:52。各份都远在 450 秒之内；包的用时随 runner 的 CPU 差得多，不拿来与 `main` 比，合并之后两次 `main` 照 06-environment 第 4 节记进 00 号文档第 13 节。文档提交 `97693f6f` 的分支 CI：run 43805853985，第 1 次 attempt 全绿；Test 一步第三份 5:34、第七份 3:48、第八份 5:31、竞态一份 9:04，E2E 一步 7:14。合并 `main` 之后的 `d2c86359`：run 43808862555，第 1 次 attempt 全绿；第三份 4:36、第七份 4:00、第八份 5:29、竞态一份 10:56。合并之后第一次 `main`（`46b6940f`）：run 43812781960，第 1 次 attempt 全绿；第三份 4:29、第七份 3:57、第八份 5:08、竞态一份 8:52，E2E 一步 6:18；整个程序的 nested 两格在 `main` 上绿。记合并号的 `41c5ac25`：run 43816198500，第 1 次 attempt 全绿；第三份 4:09、第七份 3:53、第八份 4:09、竞态一份 9:04，E2E 一步 6:06；两次的平均记在 00 号文档第 13 节（S1 的那一行）。

**3.10.5 的现有格**：`TestPlanRevertConflicts` 与 page 模块的全部测试在规则之后照过，没有一格翻过来；`bootstrap` 里翻过来两格（3.22 第 1 条）：种下的上下相套的两项与交错 20 的"往回"，都由 409 变成撤销得了、笔记本等于之前。

**审查**：三份（A 规则 0I 1M 2N，B 性质测试 0I 2M 2N 1Q，C 组合根、提交与文档 0I 2M 6N），修复第 1 轮四个只改测试的提交与草稿的改动；核对第 1 轮三位的旧发现都解决，新的只有 4 条 Nit（只在文档），在本提交改了文字。主持者的决定 479–482。见 [S2 的审查记录](reviews/P3-S2-nested-trash-roots-review.md)。

**第 8 节"S2 另"**：3.10.7 的每个计数的下限在测试里（上表的 23 个），实测的最少次数在上表；整个程序的 nested 两格在分支的 CI 与合并之后的 `main`（run 43812781960）上都绿。

### 10.3 S3：导入导出的任务（2026-11-17，合并见 00 号文档第 12 节）

- **提交**（分支 `m12-p3-s3`，从 `main` 的 `2d32d346`；实施 4 个：6.3 的第 1–3 个照次序，另加决定 482 的一个（决定 530）；修复第 1 轮 7 个；合并 `main` 一次；文档是本提交）：
  - 启动时的收拾（T4）：五种任务设 `DiscardRunning`；`AtStart` 照 `domain.Kinds()` 逐种 `lost`；River 适配器、模块与 `TestTheRescue` 的测试；`riverHoldsSQL`、`riverTakenSQL` 与 3.14.3 的六处种子；`TestServeFailsTheJobsLeftRunning` 的三种排队行（`62e1416e`）。
  - 按人的上限（T2）：`MaxPersonJobs`、`ErrTooManyJobs`、`CountActive` 的 `mine`、`Uploads` 记人、`room` 先按人再全局、回滚以零 UUID；契约四处、`make gen`；网页的码、两种语言的文字（含 `importCut`）、`offeredOf`、`jobs-server.ts`；`TestAPersonsSixthJobIsRefusedAndAnothersStarts`（`9aea99ec`）。
  - 名称的编码（T3）：`names.go`（`unicodePath`、`readNames`）、`Entries` 以读出的名字判文件夹；`NameSource`、`RawEntry.From`、`Sorted.Read`、两种原因、`importRun` 在 `finish` 之前加进报告；契约的枚举与说明、`make gen`；网页的报告文字；适配器的表（13 格）、差分（13 个计数各带下限）、模糊；`TestAnImportReadsNamesNotInUTF8`（`292b238f`）。
  - 决定 482：`transfer.revertLeftGroup` 去掉"不要先撤销这次撤销"，两种语言，`transfer-background.test.tsx` 的两处断言（`a48ace20`）。
  - 每个提交在干净的 worktree 里 build、vet、`make lint`、`make test-go-short` 都是 0；第 1 个另跑 transfer 模块的库测试与 3.14.3 的四个 `-race -count=20`（80 次全过，包 100 秒）；第 2、3 个另跑 `gen-check`、各自的整个程序的测试（竞态）与分份（557、558 个）；第 4 个另跑 knip 与 `test-web`。第 2、3 个推送之前各改过一次（`oxfmt` 的格式、`gochecknoglobals`）：`9aea99ec` 的 `test-web`、`292b238f` 的 knip 与 `test-web` 是在改前的 `03b29219`、`fae5c81e` 上跑的（改的只是格式与 Go，都是 0），审查 C 在最终的 sha 上补跑，都是 0（4,608 与 4,614 个）。
- **与设计不同之处**（3.23 列出）：文档不随代码的提交（决定 531）；决定 482 是第 4 个代码提交（决定 530）；app 的替身认得来源；zip 的帮手；差分在内部测试；第一个 0x7075 说了算，模糊的种子多三个钉边界的；`Read` 收库的顶层文件夹；来源到码是方法；T2 的测试分布与锁下的一格；T4 的测试帮手；契约说明的措辞；"不是 UTF-8"的文字不改（决定 532）；三处种子写成一条语句；启动时 `lost` 失败就让启动失败。
- **先红后绿**：3.14.3 的六处种子各去掉 River 行都红（第 4 处 `rollingBackIn` 不确定：`TestUpgradePermissionMatrix -count=5` 里 4 次红、1 次绿；第 3 处读与取消两格这一次都红）；第 6 处带着 River 行、插一次收拾照绿。差分先在"没有标志的名字都按 GBK 读"上红。
- **变异**（7.3；`mut.py`，每张表先在没改的树上把每个命令跑一遍为绿；头 `a48ace20`）：S3-1 到 S3-18（S3-9 分 domain 与 app 两条，S3-17 五种各一条）与实施者加的 O-1–O-12、网页的 W-1–W-3，服务端 50 条红（轻的 34、重的 16，含六处种子的对照）、2 条正向对照照预期绿，网页 3 条红；没有抓不到的。S3-15 在 http 的四格上也红（替身 `maxQueued` 3，五个任务让两样都满）。分份脚本的对照同 S1：去掉 `$$` 照绿，从 `SEVENTH` 去掉、加进 `EIGHTH`、只在 `EIGHTH` 都 rc=1。
- **下限**（3.12.3；差分 `TestReadNamesReadsAsGBKOnlyAZipOfGBKNames`，种子 1–3，各 1,000 个包；下限是另 20 组 `3b+1`–`3b+3` 的最少次数的三分之二、向下取整）：

  | 计数 | 测试自己的 | 20 组的最少 | 下限 |
  |---|---|---|---|
  | GBK 的包 | 991 | 906 | 604 |
  | 没有候选的包 | 1,798 | 1,744 | 1,162 |
  | 有候选解不开的包 | 211 | 200 | 133 |
  | 其中旁边有解得开的候选（S3-6） | 110 | 82 | 54 |
  | GBK 的包里有标志、解得开的名字（S3-1） | 455 | 399 | 266 |
  | 其中不是 UTF-8 的（S3-2） | 253 | 214 | 142 |
  | 没有候选的包里无标志、非 ASCII 的合法 UTF-8（S3-3） | 2,270 | 2,146 | 1,430 |
  | GBK 的包里无标志、恰好合法 UTF-8、读成 GBK 的（S3-4） | 865 | 698 | 465 |
  | CRC 错或版本 2 的字段（S3-7） | 2,095 | 2,002 | 1,334 |
  | 从字段读的名字 | 1,570 | 1,598 | 1,065 |
  | 其中原名是 ASCII 的 | 269 | 246 | 164 |
  | 字段与原名相同的 | 46 | 32 | 21 |
  | 读成 GBK 的名字 | 2,279 | 2,008 | 1,338 |

- **实测**（7.5；本机 8 核 arm64，经 `heavy.sh`，头 `a48ace20`）：`TestAnImportReadsNamesNotInUTF8` 1.36 秒（`-race` 2.23 秒；设计推约 1–2 秒），`TestAPersonsSixthJobIsRefusedAndAnothersStarts` 1.31 秒（`-race` 2.17 秒）：第七份竞态下约多 4.4 秒，加上 S1 的 4.6 秒共约 9 秒，超出 7.5 的"约 4–6 秒"（那是 S1、S3 四个测试合起来的推断），第七份在 7763 上约 324 秒、余量约 126 秒（推断：7.5 的 315 秒加本机竞态下多的约 9 秒；这次 CI 上第七份落在 Xeon 6975P-C 上，Test 一步 4:10）。审查 B 另量：前者 1.42 秒、竞态 2.31 与 2.33 秒。审查 A 的探针（竞态下 20 次）：启动时的 `lost` 都比 `newAcmeTeam` 返回早 0.70–0.73 秒，所以 3.14.3 的 `-race -count=20` 证明的是"不偶发"，没碰上赛跑（3.23 第 13 条）。差分在竞态下 0.14 秒（非竞态 0.02 秒），模糊的 9 个种子 0.00 秒，表 0.04 秒；archive 包竞态下 1.32 秒（负载 2.4–4.5）。
- **门禁**（本机，头 `a48ace20`）：`make lint gen-check knip build` 绿；`make test-go`（组合根八份、竞态、成本、守卫；185 个包 ok）与 `make test-web`（4,614 个）绿；`make test-go-short` 绿；`make e2e` 577 个（576 个通过、1 个跳过：WebKit 的 A15 第 3 格），TR4 的 `f\xff.md` 照旧 `name_not_utf8`；分份脚本 558 个，145/50/119/120/60/15/34/15，两个新测试在第七份。
- **分支的 CI**：`a48ace20` 的 run 43919318998（`ci.yml`），第 1 次 attempt 19 项全绿，没有重跑，没有碰到已知的偶发。Test 一步（E2E 是那一步）与 runner 的 CPU（作业的 "Runner CPU" 一步）：

  | 作业 | Test | CPU |
  |---|---|---|
  | bootstrap-1 | 6:29 | EPYC 9V45 |
  | bootstrap-2 | 4:17 | Xeon 6975P-C |
  | bootstrap-3 | 4:38 | Xeon 6975P-C |
  | bootstrap-4 | 4:49 | Xeon 6975P-C |
  | bootstrap-5 | 6:25 | EPYC 7763 |
  | bootstrap-6 | 5:19 | Xeon Platinum 8573C |
  | bootstrap-7（两个新测试） | 4:10 | Xeon 6975P-C |
  | bootstrap-8 | 4:17 | Xeon 6975P-C |
  | race（差分与模糊的种子） | 9:08 | Xeon 6975P-C |
  | E2E 一步 | 6:15 | |

  八份都在 450 秒之下（最慢的第一份 389 秒、第五份 385 秒，与 S3 无关的两份，分别落在 9V45 与 7763 上）。第七份这次没落在 EPYC 7763 上。别的作业：web 4:43，short 2:57，costs-1/2/3 11:06/24:48/14:17，costs-lint 8:03，guards-core 12:56，guards-sources 10:36。
- **第 8 节"S3 另"**：3.14.3 的六处种子都改了，每处的反向对照跑过（第 4 处 5 次 4 红）；四个测试的 `-count=20` 在第 1 个提交上 80 次全过。
- **审查**：三份（A T4 与决定 482 0I 0M 4N；B T3 0I 1M 3N；C T2、契约与网页、提交与文档草稿 0I 1M 3N）。主持者的决定 530–532。见 [S3 的审查记录](reviews/P3-S3-transfer-jobs-review.md)。
- **修复第 1 轮** 7 个提交（`65315a27`–`cbda8435`，只改测试）：模糊的种子加三个钉边界的字段（1 字节的、尺寸多 1 字节的、截在 CRC 与 `测` 之后的），`unicodePath` 的三处边界（审查 B 的 M1）；表第 11 格加一个 GBK 名字以 0x5C 结尾的符号链接，`Special` 按读出的名字判（B-N1）；失败的导入照样列出读过的名字（B-N2）；后台开始在锁下再数一次的一格（C-M1）；启动时 `lost` 失败就让启动失败的一格（A-N4）；三处种子的行与 River 行写成一条语句（A-N3，3.23 第 13 条）；中文的 `revertLeftGroup` 整句比对（A-N2）。修复的变异 8 条（B5、B6、B7、B11、B12、MC-1、A-N4 的"失败照样往下"、A-N2 的"中文的后半句放回去"），修之前都绿（审查 B、C、A），修之后都红；种子的对照：照原样绿，每个种子之后插一次收拾、River 行照写的绿，不写 River 行的三个都红。逐个提交 build、vet、`make lint`、`make test-go-short` 都是 0，网页的那一个另跑 knip、`test-web`（4,616 个）；推送之前组合根的第三、四、七份（`make test-go-bootstrap-3/4/7`）与 transfer 模块的全部包 `-race` 都绿。分支的 CI：`cbda8435` 的 run 43948816167，第 1 次 attempt 19 项全绿，没有重跑（`ci-wait.sh` 轮询时一次答复不是 JSON，打出一段 traceback，接着照常等到结束）。Test 一步：第三份 4:37、第四份 4:53、第七份 4:05（都在 Xeon 6975P-C 上），竞态一份 9:04（Xeon 6975P-C）；最慢的第五份 6:26；E2E 一步 6:16。
- **核对第 1 轮**：三位的旧发现都解决，A、B 各一条新的 Nit、C 没有新的，都只在文档，在本提交照审查者改：
  - A 的 C1-N1：3.23 第 13 条说矩阵的种子"在起应用之前写"不对，它在 prepare 的应用起来之后写；改为照实写它不怕赛跑的原因。
  - B 的 C1-N1：`full[:12]` 截在 CRC 与 `测` 之后，不是"截在 `测` 的中间"；文档照实写，测试里的注释照旧，不改（3.23 第 6 条）。
- **合并 `main`**：`7dfb248a` 合进 P4/S2 的合并（`e2e/perf/`、e2e 的依赖与锁文件、knip 的配置）与 P4/S5 的设计（决定 519–529），没有冲突：S3 的提交不碰这些文件，也不碰 `docs/`。之后 `make lint gen-check knip build`、`make test-go-short`、`make test-web`（4,616 个）经 `heavy.sh` 都是 0；分支的 CI run 43969116486 第 1 次 attempt 19 项全绿，Test 一步第七份 4:05、竞态一份 9:11（都在 Xeon 6975P-C 上），最慢的第四份 6:25（Xeon Platinum 8370C），E2E 一步 6:18；各份与 CPU 在 S3 的审查记录。
- **本提交自己的分支 CI**：（待填：本提交的分支 CI）。

---

## 附录 A 读过的代码

都在 `194ca350` 上读过；本文正文引的行号以这里为准。

- **page**：`domain/revert.go:196-214`、`:275-353`、`:388-…`、`:470-533`、`:677-686`；`domain/revert_part.go:25-30`、`:42-51`、`:79-84`；`domain/revert_facts.go:23-25`；`domain/revert_schedule.go:86`、`:262-282`、`:328`、`:431-436`；`domain/rollback.go:58-69`；`domain/search.go:238-255`、`:272`、`:282`、`:493`；`domain/path.go:37-53`；`domain/revert_test.go:84-142`、`:454-695`、`:955`；`domain/client.go:50`；`app/jobs.go:413-602`；`app/unit_restore.go:115-118`；`app/unit_revert.go:470-515`；`app/history.go:18-24`、`:49-94`、`:146-193`；`app/list_changesets.go:49-208`；`app/get_changeset.go:34-122`；`app/read_path.go:153-167`；`app/fakes_test.go:28-33`、`:811-839`；`app/revert_job_property_test.go:64-127`、`:426-474`、`:476-644`、`:699-787`、`:903-965`；`app/rollback_test.go:415`、`:560-621`；`app/revert_test.go:20-28`；`app/path_history_test.go:333`、`:405`；`app/search_test.go:122`；`adapter/postgres/queries/nodes.sql:258-261`、`trash.sql:64-90`、`history.sql:63-71`、`:172-197`；`adapter/mcp/tools.go:238-250`、`log.go:11-18`；`adapter/river/search_test.go:64`；`module.go:239-242`、`:296-297`、`:318`、`:329`、`:367`；`virtual_files.go:104-197`。
- **shared**：`title_key.go:18-28`、`title_key_context.go:15-38`、`title_key_test.go:9`、`:43-44`、`name.go:16-38`、`client_name.go:28-47`、`actor.go:15-23`、`error.go:186-189`。
- **linking**：`domain/index.go:87-109`；`adapter/postgres/queries/reads.sql:87-94`、`reads_test.go:417`；`app/list_index.go:192-219`、`list_index_test.go:157`。
- **platform**：`markdown/parse.go:52-103`、`:168-177`、`parse_test.go:94`、`links_test.go:165`、`render_test.go:127`；`postgres/tx.go:131-144`；`httpserver/mcpserver/errors.go:77`、`:99-101`、`:290-299`、`:358-360`、`call.go:46-60`、`answer_test.go:413-415`、`prompt_bounds_test.go:498`；`httpserver/apierrors.go:231-233`、`limit.go:38-47`；`jobs/jobs.go:33-39`、`:101-106`、`:134-148`、`:169-186`、`inserter.go:50-78`、`jobs_test.go:305-317`。
- **events**：`app/hub.go:26-31`、`:89-116`、`app/stream.go:13-25`、`app/open_stream.go:30-46`、`app/hub_test.go:61`；`domain/frame.go:15-30`；`adapter/http/handler.go:66-74`、`:96-100`、`handler_test.go:505`。
- **transfer**：`adapter/archive/archive.go:296-309`；`domain/entry.go:88-201`、`report.go:100-141`、`:213-227`、`errors.go:24-32`、`:49-55`；`domain/entry_test.go:55`；`app/import.go:93`、`:151-154`、`import_write.go:350`；`app/start.go:59-82`、`:186-245`、`start_import.go:68-115`、`:243-272`、`start_background.go:73`、`:114`、`:138`、`:182-200`、`rollbacks.go:31-38`、`uploads.go:20-103`、`maintain.go:87-154`、`ports.go:275-286`；`app/fakes_test.go:486-497`、`import_fakes_test.go:28-70`、`requests_test.go:339-371`；`adapter/postgres/queries/jobs.sql:16-18`；`adapter/river/jobs.go:47`、`:135-171`、`:294-305`；`module.go:255-264`；`rescue_test.go`、`contributors_test.go:76-103`、`:155-169`；`server/migrations/00029_transfer_transfer_jobs.sql:22`、`:61`。
- **identity、workspace**：`identity/domain/user.go:49`、`api_token.go:77`；`workspace/domain/workspace.go:27`、`:45`。
- **bootstrap**：`upgrade_rollback_revert_test.go:30-118`（nested `:87-114`、`:99-110`）；`transfer_jobs_test.go:40-110`（`seededJob`、`seedJobs`）、`:225-249`；`permission_matrix_transfer_test.go:30-50`、`permission_matrix_seeded_test.go:716-727`；`upgrade_fixtures_test.go:75-99`；`upgrade_permission_matrix_test.go:410-425`；`transfer_import_test.go:372-386`；`interleavings_transfer_test.go:116-147`、`interleavings_import_test.go:174-190`；`upgrade_rollback_test.go:709`；`assets_jobs_test.go:124-132`（`awaitJob`）；`app_test.go:128-150`（`waitHealthy`）；`jobs_test.go:55-85`；`interleavings_virtual_test.go:128-164`；`events_stream_test.go:44`；`events_lab_test.go:38-90`；`mcp_reads_test.go:557`、`mcp_history_test.go:269`、`mcp_tools_test.go:156`、`mcp_test.go:232-237`、`mcp.go:92`；`search_test.go:485-527`；`transfer_jobs_test.go:165-194`；`transfer_test.go:453-465`；`app.go:94-107`。
- **archtest**：`rules_test.go:127-133`。
- **契约**：`api/openapi.yaml:17`；`api/modules/events.yaml:54-72`、`:235-243`；`linking.yaml:125-133`；`transfer.yaml:28-33`、`:88-106`、`:166-188`、`:256-302`、`:1055-1095`；`api/mcp/tools.json:303-342`、`:938-…`。
- **网页**：`W/events/hub.ts:6-10`、`:198-273`；`W/app/problem-messages.ts:11-33`、`:60-90`、`problem-messages.test.ts:12-60`；`W/pages/notebook/transfer-report.tsx:92-125`、`:244-262`；`trash-row.tsx:125-177`；`revert-dialog.tsx:186-200`；`export-dialog.tsx:76-80`；`import-dialog.tsx:217-223`；`W/test/jobs-server.ts:152-214`、`:320-372`、`history-server.ts:41`、`:472`；`W/services/api.ts:61`、`:70`；`W/i18n/messages/en.ts:1056-1057`、`zh-CN.ts:1009-1010`（`transfer.importCut`）；`W/i18n/messages/en.ts:905`、`:968-983`、`:1023-1075`、`:1249-1251`，`zh-CN.ts` 对应的各行。
- **其余**：`Makefile:272-402`；`README.md:192-223`；`e2e/stories/transfer/tr4-malicious.spec.ts:55-105`；`e2e/fixtures/events.ts:55-57`；Go 1.27.4 的 `archive/zip/reader.go:391-406`、`struct.go:75-79`；`golang.org/x/text` v0.42.0（`server/go.mod:33`）。

## 附录 B 实验

在 scratchpad 的临时 worktree（`194ca350`）里跑，原样留在 `m12/p3-design/`：

| 文件 | 内容 | 结论 |
|---|---|---|
| `domain_zz_p3design_test.go` | 3.10.2 的六种形状，三种代码（改前、`split-tops.diff`、"`topOf` 只看起点"） | 改前都冲突；原型照常撤销 1–4、反例 5–6 照旧不撤销；"只看起点"只在第 4 格（嵌套的组两个成员）与原型不同 |
| `app_zz_p3design_test.go` | 3.10.6 的形状，`needed` 不改与改后 | 不改时模型第 4 步失败、批切在段尾时替身记下"活着的页在不活着的父节点下"；改后都对 |
| `gbk_zz_test.go`、`gbk_zz2_test.go` | GBK 解码器对 0x5C 尾字节、Shift-JIS、GB18030 四字节、0x80、0xFF | 3.12.1 的表 |
| `gbk_utf8_chance.py` | GBK 名字恰好是合法 UTF-8 的比例 | 一级汉字单字 18.2%，两字 3.3%；`目录`、`图片` 是 |
| 用完即删的 `postgres:18.7-trixie` 容器里的三个 psql 会话 | REPEATABLE READ 的快照冻结的时刻 | 冻结在第一条读等锁之前（简单与扩展协议相同）；READ COMMITTED 的对照看得到等锁期间的提交（3.8.2） |
| `evidence.txt` | 以上的输出 | — |
| `m12/p3-rev1/shards.py`（第一次修订） | 审查 C 的 `shards.py` 照修订的名字（四个新的、一个改名）、读 `86565dd6` 的 `Makefile` | 新的四个都只在第七份，改名的只在第八份，全部顶层每个恰在一份；`TestServeFailsTheJobsLeftRunning`、`TestOneImportAtOnceThroughServe`、`TestPermissionMatrix` 在第四份，`TestTheSweepDeletesTheArchivesNoJobKeeps`、`TestTheQueuesLimitIsTheConfigured` 在第三份，`TestUpgradePermissionMatrix` 在第七份 |
| `m12/p3-rev2/time4.sh`、`time4.log`（第二次修订） | 3.14.3 点名的四个测试在 `86565dd6` 的临时 worktree 里竞态下各跑一次（经 `heavy.sh`，负载 0.6，编译已缓存） | 3.31（用 `holdExportWorker` 的子测试 1.16）、0.99、0.78、0.79 秒；20 次约 2 分钟 |
| `m12/p3-rev2/trial/00-M12-design.md`、`trial.diff`（第二次修订） | 00 号文档的脚本在 `86565dd6` 的副本上 `--check` 与写 | 45 处全部匹配；改坏一处旧文字之后报出那个标签、一个字也不写 |

审查者的原型、探针与量得的数在 `m12/p3-review-a/`（302 的原型、性质测试的下限、竞态的用时、变异）、`m12/p3-review-b/`（pgx 的快照探针、标题键与 `Blank` 的逐码位探针）、`m12/p3-review-c/`（Writer 的标志、GBK 的往返、T4 的探针、分份），修订核对的在 `m12/p3-check1-c/`（T4 的种子）、`m12/p3-check2-b/`（提交跨过时限的探针），本文引的地方注明了。

整个程序的测试只为 3.14.3 的估时各跑了一次（上表，第二次修订）；别的没有跑，`make test`、`make e2e` 都没有跑（设计不跑重的东西）。
