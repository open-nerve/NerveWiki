# M8/P3 日志与整组撤销设计：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | [03-P3-log-revert.md](../03-P3-log-revert.md)（初稿），对照 `main` `613adfc` 的代码、P2B 的工作区（审查时 `f22a9dc`，修订时 `4daf892`）、00 号文档、总体设计与 P1、P2 的审查记录 |
| 审查方式 | 两位审查者（Opus）并行，只读：撤销的语义、数据与并发（A；另以仓库的 sqlc 离线生成了初稿的两条迁移与 3.3、3.4 的全部查询）、契约、权限、前端、测试与 e2e、文档的一致（B；另读了生成的 strict server 与 oapi-codegen v2.8.0 的模板，定下可选的请求体怎么解码） |
| 日期 | 2026-10-14 |
| 结论 | 没有 Important。A：Minor 7、Nit 9；B：Minor 11、Nit 22（B-M11 与 A-M3 是同一处）。全部处理，修订直接写进设计（与这份记录一起提交）。A 的"过度设计"第 1 条采纳：P3 只做单轮应用，通用的多轮与临时名称移交 M9；B 的"过度设计"里与它相反的一句因此没有采纳（见末节）。没有要负责人确认的新事项（设计 2.2 仍为"无"） |

## 发现与处置

节号是修订后的。

### A：撤销的语义、数据与并发

| # | 发现 | 处置 |
|---|---|---|
| A-M1 | `Run` 答的 `ErrChangesetReverted` 不经 `treeError`（它只套在单元里的 `CreatePage`、`CreateAsset` 上），任务以 `internal` 失败；`unit` 把它当作结果未知，这一批的文件不删 | 同 B-M1：`transferTree.Import` 的结果经 `treeError`；`unit` 把它当作确定的拒绝、删文件，`unitFailure` 报 `changeset_reverted`；`transfer/app` 的单元测试，交错 5 的第三批带附件、`checkAssets` 钉住（2.1 第 7 条、3.7、3.10、3.12） |
| A-M2 | `RecordItems` 的 `DO UPDATE` 保留第一次的 `revision`：先改名再删除同一页，删除的条目没有版本号，之后撤销永远 `changed` | `DO UPDATE` 加 `revision = CASE WHEN excluded.after_name IS NULL THEN excluded.revision ELSE changeset_items.revision END`（合并成删除时取删除的）；适配器测试"改名之后删除，条目带删除时的版本号"（3.4、3.12）。P3 不再走临时名称（A 过度设计 1），这一处仍改：M9 的 `batch` 会这样合并，改动只是一列 |
| A-M3 | L 里"回收站里的正文的哈希"没有来源：`ContentsNow` 只读活着的，`TrashGroup` 没有哈希 | 经 `VersionHashes` 读 `(页, 组里它现在的版本号)` 与 `(页, item.Revision)` 两个哈希相比（每个正文的版本号都有一行版本）；读不到的记进 `RevertPlan.Missing`，答 409 `page.changeset_trimmed`（3.5 的输入、L 表，3.6 第 8 步） |
| A-M4 | 先读组再判断它是不是 C 的那组：可能读几万个节点，并以 422 盖住真正的 `changed` | `NodesNow` 带根上记下的 `trash_pages`、`trash_assets`；第 6 步先核对每个最上层（在回收站、是根、两个数等于 C 删除的它与它之下的页面、附件），通过的才读组，不通过的交给 `PlanRevert` 记 `changed`；用例测试"不读组、答 409 而不是 422"（2.1 第 3 条、3.4、3.6、3.12） |
| A-M5 | 通用的"放回"先移动后改名，中间态撞上兄弟的唯一索引；临时名称超出 255 字节、丢掉附件的扩展名（M9 的潜在缺陷） | 照 A 过度设计 1：P3 只做单轮（放回 → 删除 → 恢复 → 正文），模拟按移动、改名两步的中间状态判，做不完答 `ErrRevertUnordered`（缺陷，500）；多轮、临时名称与这两处问题连同数据库上的测试写进 M9 的移交（2.1 第 11 条、3.5、第 7 节） |
| A-M6 | `RenameNode`、`MoveNode` 答 `ErrTitleTaken`，`WriteContent` 答 `ErrRevisionMismatch`，都是已分类的 409，原样传出是契约之外的码 | `Unit.Revert` 以 `%v` 包成未分类的错误（同 P2A 审查 B-4），答 500；用例测试各一例（2.1 第 3 条、3.6 第 4 步、3.12） |
| A-M7 | 交错少两例：撤销编辑会话自己的变更集；撤销里的恢复撞上清理的半途 | 加交错 9（会话活着 → `page.locked`；过期之后撤销成功、旧标签页的保存 `page.edit_session_ended`；净改动为零 → `changed: false`、会话照旧并入 C，3.6 第 5 步写明是预期的）与交错 10（只跑版本的清理器 → 读不到哈希 → `page.changeset_trimmed`；只跑附件的清理器 → `ErrTrashChanged` → `page.changeset_trimmed`）（3.10） |
| A-N1 | `IS NOT NULL AS has_item` 在 sqlc 里生成 `interface{}` | 写成 `(i.node_id IS NOT NULL)::boolean`（3.4） |
| A-N2 | "没有 `revision` 的旧条目"只对页面 | L 表按类型分开，附件不比；domain 测试加"删除的附件不因没有版本号而冲突"（3.5、3.12） |
| A-N3 | 补值的测试经程序写不进 `Down` 之后的旧约束 | 以 SQL 写删除的节点、正文与条目，一例在回收站、一例已恢复（3.2） |
| A-N4 | `Parsed` 按版本号配对，两遍之间字节相同、版本号变了会答一次假的 `changed` | 按正文的哈希配对，写下的版本号取锁下读到的；`VersionContents` 同时读哈希（3.4、3.6 的 `Unit.Revert` 第 3 步、3.12） |
| A-N5 | `ChangesetVersion` 少了 `BaseBytes` | 加上（3.1、3.5） |
| A-N6 | "最上层的那个"不对：撤销一次导入的撤销以同一时刻删除几个最上层 | 改为"每个最上层（'前'的父节点不是 C 删除的节点）各是一项的根，它的组恰是 C 删除的、它之下的节点"（2.1 第 1 条、3.5） |
| A-N7 | 读整本树与笔记本成正比，降界线压不住它 | 基准把读树与 `PlanRevert` 分开记；与 C 成正比的部分超过才另取界线，读树超过写进 M12 的移交、改法是只读附近的节点（2.1 第 9 条、3.11） |
| A-N8 | 组内两个节点在 Unicode 升级之后键相同，撤销永远 `title_taken`、两边都是 C 的节点 | 写成已知的局限（3.5），冲突的提示在节点在回收站里时给"从回收站恢复（会加序号）"（4.4） |
| A-N9 | 移回一个节点的代价与子树成正比、不受 `MaxUnitNodes` 管 | 2.1 第 9 条写明不计入的理由：原来的移动也是这样，撤销不比它多 |
| A 过度设计 1 | 通用的多轮应用与临时名称 M8 用不到，只在纯函数上测，A-M2、A-M5 正是它在数据库上的缺陷 | 采纳，见 A-M5 |

### B：契约、权限、前端、测试与文档

| # | 发现 | 处置 |
|---|---|---|
| B-M1 | 同 A-M1 | 同 A-M1；另加网页的 `transfer-report.test.ts`、`transfer-import.test.tsx` 各一行（3.9） |
| B-M2 | 第 5 节的两步绿不了：契约的四个操作在第 5 步、矩阵行在第 6 步；平台的扩展成员在第 1 步、`common.yaml` 在第 5 步 | 矩阵的四行、`matrixRevert` 与 `seeded.workspaceOfRow` 的登记移到第 5 步（同 P2A）；`common.yaml` 的 `Problem.conflicts`/`conflict_count` 与 `httpserver.Problem`、平台的契约测试同在第 1 步（3.10、第 5 节） |
| B-M3 | `expectSubtreeRestored` 写死 `trash_restore`；HI4 只有一个版本；HI3 的页面版本没有数据断言 | `expectSubtreeRestored` 加参数 `kind`；HI4 加 PAT 的接口版本，两个版本调用同样的断言；HI3 的页面版本撤销那次改名并调用接口版本的断言（`expectVersions` 的 `reverts_id`、`links.ts`、冲突之后没有新的 `revert` 行）；接口版本另经登录会话写一次（`web`），客户端的筛选才有意义（4.8） |
| B-M4 | 日志行的名称只到分钟，同类型同执行者的两条同名（P2B 刚修的同一个问题） | 以 `twinsOf` 按（类型、执行者、分钟）找出同名的行，加 `shortId`；"撤销了「…」"的链接同样；vitest 两条同一分钟的行（4.2、4.7） |
| B-M5 | 422 的文字只说节点数，五种界线里只有一种是它 | 照 P2B 的 `trash.tooLarge` 说"节点、页面、正文、链接或这些页面的历史"，`node_count` 作为事实另写（4.4） |
| B-M6 | 网页从答复里看不出 C 改名或移动：前三项按节点 id，改名那一项可能不在里面 | 只有 `getChangeset` 答 `renames`、`moves`（新的 `ChangesetMoves`，在前 1,000 项条目上算；契约里是 `ChangesetDetail`，照 `TransferJobDetail` 不用 `allOf`）；单个变更集页的确认框读 `renames || moves`，冲突里的"改回原名"读 `renames`（2.1 第 5 条、3.4、3.9、4.4）。日志的每条不带：一页 100 个大的变更集要读约 10 万行条目，日志的行以前三项为摘要，撤销只从单个变更集页发起；成本测试量那个 5 万个条目的变更集的 `ChangesetMoves`（3.11）。没有改 `changes` 的次序 |
| B-M7 | "结果未知"漏了请求期限的 500；按 `reverted_by` 的个数判断会在 100 处失灵、会把别人的撤销说成自己的 | 500、网络错误、超时都说"结果未知"；比较 `reverted_by[0]` 与请求之前知道的最新那个，新的就读它，执行者是自己才说"已撤销"，否则"已被 X 撤销"；对话框测试加 500（4.4、4.7） |
| B-M8 | 手改的地址里不是 UUID 的值得到 400，历史页不处理 | `:changesetId` 不是 UUID 时 `NotFoundPage`、不读（同 P2B 的 `nodeIdPattern`）；`actor`、`page` 不是 UUID 时不读，同 422 说明并给"清除筛选"（4.2、4.3） |
| B-M9 | 右栏的版本列表加变更集链接，每行同名 | 只在版本页加（P1 的移交只要这一处）（4.5） |
| B-M10 | 2.1 与第 7 节漏了 00-M8 与 v0.1 的几处 | 2.1 第 6 条改 :166 与 :51；加第 12 条（:75 的用例名 `RevertChangeset`）、第 13 条（:243 加 `reverted_by` 100）；第 7 节加 v0.1 6.1（日志的内嵌显示名）、9.2（左栏的历史与回收站）、13.1 第 5 条（并入的单元锁变更集的次序）、第 19、21 条、13.2 第 25 条、13.4 第 8 条（两个新的成本测试与 Makefile）；M12 的移交第 15 项与新的 M9 移交 |
| B-M11 | 同 A-M3 | 同 A-M3 |
| B-N1 | `bootstrap/deps.go` 一行不对：`History` 在 `page/module.go` 里接 | 去掉，`page/module.go` 一行写明（3.1） |
| B-N2 | "409 四种"：只有三个 409 的码 | 改为"409 三种，冲突另有超过 100 项的一例"（4.7） |
| B-N3 | 内联的可空对象不合契约的规则；计数没有上下限；数组没有 `maxItems` | 命名 `ChangeNode`、`ChangeContent`；计数 `minimum 0`、`maximum 1000`；`reverted_by`、`conflicts` `maxItems 100`，`changes` `maxItems 3`（3.9） |
| B-N4 | `RevertedChangeset` 与 `ChangesetReverted` 容易混 | 前者改名 `ChangesetBrief`（3.9） |
| B-N5 | 请求体是不是 `required` | `required: false`：空的 POST 即没有说明；网页不带空的说明，`""` 答 422（3.6、3.9、4.4） |
| B-N6 | `ParseClientFilter` 要拒绝坏的 UTF-8 | 写明并测（3.4、3.12） |
| B-N7 | `page_id` 答空列表与 4.12 的约定相反 | 照约定：不是这本笔记本的节点（含已被清理的）答 404 `page.not_found`，码的次序同 `listAssets`；网页同 422 给"清除筛选"（2.1 第 5 条、3.4、3.9、4.2） |
| B-N8 | 冲突带的是哪个名称 | S\* 里的名称（`title_taken` 即被争的），S\* 里不在的是现在的名称（2.1 第 6 条、3.5、3.9） |
| B-N9 | `title_taken` 的回收站提示只适合在回收站里的节点；`parent_gone` 的"在回收站里"不含已清理 | 提示只在节点不在树里时给；`parent_gone` 写"不在了（在回收站里或已被清理）"（4.4） |
| B-N10 | 明细的节点没有 `trash_root`；正文的链接对不活着的页无用 | 回收站里的都链到 `/trash/:id`（预览把非根换成它的根）；正文的链接只在活着的页上（4.3） |
| B-N11 | 403、404 之后重读什么 | 这一页与 `["notebooks", ws]`，同 P2B 回收站的拒绝（4.4） |
| B-N12 | 焦点回到 `h1` 要经 `watchReader` | 写明（4.4） |
| B-N13 | 撤销可能带回这个标签页删掉的页，答复不列出它们 | `removed` 不空时再读一次树，同 P2B 的 `restore`；`removed` 也收进别人删掉的页，无害（4.6） |
| B-N14 | "历史"要不要 `end`；单个变更集页的面包屑；`nav` 的名称键还是 `trash.places` | `NavItem end`；`BreadcrumbTrail`；名称键改为 `notebook.places`（4.1、4.3、4.5） |
| B-N15 | 带说明的确认框是表单 | 经 `useForm`，服务端的 422 落在字段下（4.4） |
| B-N16 | 已被撤销过的再撤销，除非那次撤销又被撤销，否则每个节点都冲突 | 按钮旁写明（4.3） |
| B-N17 | 日志没有总数，"播报结果数"说什么 | 定下文字（"列出 50 条，还有更多"等）（4.2） |
| B-N18 | 重读之后行会掉出去 | 行的控件用 `useFocusLeaving`（4.2） |
| B-N19 | 只测了观察者，守卫也要看到 `Kind`、`RevertsID` | `TestTheObserversFollowARevert` 加一个守卫的断言（3.8） |
| B-N20 | `reverted` 是"被撤销过" | 3.7 写明：撤销又被撤销的同样不再被并入 |
| B-N21 | page 的适配器没有 `planOf`；外键的核对可以加计划断言 | `history_plans_test.go` 复制一份；加外键核对走日志的索引（3.11） |
| B-N22 | P2B 的行号要换到修复之后；A 与 P2B 谁先合并；B 第 1 步 knip 不绿 | 引用换到 `4daf892`；P2B 先合并，A 的网页改动变基到它之上（第 0 节）；服务函数随第一次用到的步骤加入，B 的步骤按页重排（第 5 节） |
| B 过度设计 1 | 右栏的变更集链接 | 采纳，见 B-M9 |
| B 过度设计 2 | 通用的依赖次序与临时名称是 00-M8 4.8 要的，保留，测试不再增加 | 没有采纳（见末节） |

### 作者的更正

| # | 发现 | 处置 |
|---|---|---|
| 作者-1 | 按 A-M3 经版本读回收站里的哈希之后，多了一种情形：清理以偏差的时钟先拿走了版本（条目还在、C 没被标记），哈希读不到 | 不算冲突，记进 `RevertPlan.Missing`，与读不到的正文在同一位置答 409 `page.changeset_trimmed`（冲突之后、503 之前，2.1 第 3 条不变）；稳定时 C 的条目也已过期、被标记，答的码一致。交错 10 走到它 |
| 作者-2 | 单轮之后 `PlanRevert` 的签名与 00-M8 的"按依赖逐个应用"、第 9 节的三种单元测试、:296、:302 的落实与移交都对不上 | 2.1 加第 11 条一并改这五处；`PlanRevert(RevertState) (RevertPlan, error)`，`error` 只有 `ErrRevertUnordered`（3.5） |

## 未采纳

- **B 过度设计 2（保留通用的依赖次序与临时名称）**：没有采纳，按 A 过度设计 1 改为单轮。理由：M8 的每个变更集只有一种操作，P3 的整个程序与性质测试都走不到这条路，它只在纯函数上测；而 A-M2、A-M5 表明它在数据库上另有问题（移动带着现在的名称撞上唯一索引、临时名称超出名称界线与附件的扩展名），要在 M9 的 `batch` 里连同真实的测试一起做。00-M8 4.8 的这一段随之修订（2.1 第 11 条），不在第 11 节负责人确认的 12 条里。
