# M8/P5 搜索：架构设计 & 实施规划

## 0. 分两部分

照 P1–P4 的做法分成 A 与 B，各自一个分支、一次审查与修复核对、一次合并。B 以 A 合并之后的契约为准。

- **A（服务端）**：
  - 先在 spike 的 30 万页语料上实测本文的方案（3.9 第 1 步），据此定下字母文字的写法（三元组 T，或 spike 量过的词的前缀 P）与几个实测的界线，写回本文之后再写代码。
  - 迁移；分词；索引的队列与后台的建索引；搜索的用例与契约；权限；`nervewiki reindex` 加上搜索；整个程序的测试、交错与守卫；在产品的代码上复现实测。
- **B（前端）**：工作区左栏的搜索框与 `Mod+Shift+F`；结果页；e2e SE1、SE2；输入法清单加一步。

P5 不依赖 P4：A 从 `main` 开始，不碰 transfer 与 P4 的任务代码。但 P4A（`m8-p4a`）已改了 P5A 也要改的这些文件，都是文本冲突，后合并的一方变基时解决：

- **迁移**：编号（P4A 用 00039，本文写 00040；P5A 若先合并就用 00039，P4A 变基时改号）；`server/migrations/schema_test.go`；`server/sqlc.yaml`。
- **page**：
  - `page/app/unit.go`：P4A 在 `Run`、`lockInto` 加 `Create`；P5A 在 `Run` 加入队。
  - `page/app/ports.go`、`page/module.go`：两边都加 `Deps` 的字段与模块的任务。
  - `page/adapter/postgres/store.go`、`page/app/fakes_test.go`。
  - `page/adapter/http/trash.go` 与 `api/modules/page.yaml`：P4A 把 `TrashPlacement` 移进 `common.yaml`，P5A 在旁边把 `TrashAttachment` 改名。
- **组合根**：`bootstrap/deps.go`（P4A 改 `transferDeps`，P5A 改 `Queues` 与 page 的 `Deps`）。
- **Makefile**：`test-go-costs` 的列表。
- **生成物**：`api/dist/openapi.yaml`、`web/packages/api-client/src/schema.gen.ts`（P4B 先合并时）。
- **文档**：00 号文档的进度表与变更记录；M12 的性能移交的编号（P4 占第 16 项，P5 写在它之后）。

语义上不冲突：后台的恢复与撤销经 `Unit.Run`，同样入队。后合并的一方在 `bootstrap/search_test.go` 的步骤里加上"后台的恢复与撤销"。

A 没有新的问题码（`workspace.not_found`、`notebook.not_found`、`validation_failed`、`bad_request` 网页已有文字），网页的门禁只有生成的类型与 `TrashAttachment` 改名（3.7）。

## 1. 基线

- A 从 `main` 的 `422abd35` 开始：P1–P3 已合并；P4 的设计已提交，P4A 在 `m8-p4a` 上实施。`1f732c96` 之后只改了文档、CI 与守卫的时限，本文引用的代码两者相同。
- **没有搜索的代码**：没有 `page_search`、`searchWorkspace`、`page.search`。`pg_trgm` 扩展由 00001 建，但没有任何三元组索引（M7 移交第 6 项的前提不成立，00 号文档 4.12 已记），也没有 `tsvector`。
- **表**：
  - `nodes`：`name` 1–255 字节，NFC；`name_key` 是完全折叠（`shared.TitleKey`）；`kind` 为 `page` 或 `asset`；新建、改名、移动、删除、恢复写 `updated_at`。
  - `page_contents`：`content` 至多 5 MiB、`revision`、`updated_at`。正文的写只改它，不改 `nodes.updated_at`（`WriteContent`）。
- **写入单元**：观察者在单元的事务里、全部操作之后、提交之前一次拿到合并的改动（`tell`）。
  - `Change` 没有类型、没有正文：新建与恢复没有"前"，删除没有"后"，改名的前后名称不同，写正文带 `Revision`，移动的子孙前后相同。
  - 改动按"第一次改到"的次序，不按 id。
  - linking 的观察者在同一事务里维护它的索引，取按笔记本的 advisory lock：同一本笔记本的保存在它上面串行到提交。
  - 导入的单元是 `Kind` 为 `import` 的单元（`ImportWrites.Import`）。
- **派生的索引表**（linking 的、`edit_sessions`）不带外键、没有 `deleted_at`，在删除的事务里删掉被删页的行。建指向 `nodes` 的外键就要 `deleted_at` 与自己的清理器：`TestEverySoftDeletedTableHasAPurger`、`TestPurgersComeBeforeTheTablesTheyReference` 都会失败。
  - linking 的行记下提取规则的版本（`indexed_pages.extractor`）：改了它的版本要求执行 `nervewiki reindex`。
- **保存的频度**：网页在内容停 2 秒之后自动保存（防抖，不是定时），每次有改动的保存是一个单元。导入每批至多 100 页、8 MiB、20,000 条链接，一个单元，持笔记本行 `FOR NO KEY UPDATE`；每 10,000 个节点各模块对自己的表 `ANALYZE`（要 `MAINTAIN` 的授权）。
- **reindex**：`nervewiki reindex` 写死只重建 linking，一本笔记本一个事务；没有登记重建者的扩展点。它同时按当前的 Unicode 数据重算标题键；README 建议在没人写的时候执行。
- **计划**：`postgres.Planned` 每次按参数规划。缓存语句的通用计划把每行与数组参数逐个比较，P2A 实测过数秒对几十毫秒。
- **可见性**：
  - `notebook.NewVisibleNotebooks(pool).VisibleIn(ws, user, role)` 答笔记本的 id，角色由调用者给；今天只有事件流用它。
  - 一个工作区的笔记本没有数量上限。
  - slug 经 `workspace.NewWorkspaces(pool).FindBySlug` 找。notebook 的 `ListNotebooks` 不另读角色，取判定答的 `Grant.WorkspaceRole`。
  - page 的 `Workspaces` 端口只有 `ShareByID`，还没有 `VisibleNotebooks`。
- **契约**：
  - 通用的 `Limit` 是 1–100、默认 50，越界是 `out_of_range`（`shared.PageSize`）；`Cursor` 是不透明的字符串。
  - 生成的绑定在用例之前答 400 `bad_request`：`required` 的参数缺了、`format: uuid` 的不是 UUID、整数的不是整数。自由文字的参数照 linking 的 `target` 不标 `required`，缺了由用例答 422。
  - `TestFreeTextParametersDoNotAnswer5xx` 给每个字符串参数送 NUL 与 `\xff`。
- **日志**：`loggedPath` 只写路由，从不读查询串。`APIErrors.Write` 把错误整个写进日志：包着用户文字的数据库错误会把它带进去。
- **后台任务**：
  - 模块的定时任务是 River 的 periodic job（page 的会话清理，`RunOnStart`）。默认队列只有 2 个 worker，由清理、各种收拾共用；长任务有自己的队列（`transfer_import`、`transfer_export`）。
  - River 默认的任务时限是 1 分钟（asset 的清扫另设 `Timeout`）。River 0.47 的 `UniqueOpts.ByState` 默认含 `completed`：完成的唯一任务在 River 的清理删掉它之前（默认 24 小时）挡住同种的新任务。
  - 测试的配置不经校验（`testConfig`）：定时任务的间隔要显式设（会话清理设 1 小时），为 0 时 River 会不停地排队。
- **CI**（`422abd35`）：服务端三份：
  - 竞态检测下组合根之外的包 4–6 分钟 / 15。
  - 竞态检测下的组合根：测试一步 8:50–10:31（作业 9:29–11:07），包的时限 15 分钟（作业 25）；M12 的移交第 15 项约定过 13 分钟时按测试名再分。
  - 不带竞态检测的成本：作业 9:26–13:22 / 20。新的 `!race` 成本测试要加进 Makefile 的 `-run` 列表才在 CI 上跑。
  - `8c83ccb7` 起守卫的时限是本机中位数的三倍、在请求的 15 秒期限之下：CI 约慢一倍。
- **网页**：
  - 顶栏在工作区之外（`Layout` 在 `:slug` 之上，没有工作区的上下文）。工作区的左栏在 `WorkspaceLayout` 里。
  - `isMod` 拒绝 Shift。除了未解析链接的回车，没有别的输入框防输入法组合中的回车。
  - SWR 用默认值（获得焦点时重读），缓存在一次登录里一直在。
  - `Located` 与 `openOf` 已有（注释点名 P5 的搜索结果，`openOf` 只有回收站用）。没有 `<mark>` 组件；`--nw-mark-background` 只在 `.nw-reading` 里定义。
  - `useLoadMore` 只在读完最后一页、按钮消失时移动焦点；快速切换以 `arrived` 到达，目的地的 `h1` 取得焦点。
  - e2e 共 241 个。
- **spike**（`scratchpad/m8/spike-search-report.md`）：
  - pg_trgm + ILIKE 不达标：1–2 个字 2.5–3 秒，常见 3–4 字 0.8–1.5 秒，`latency` 836 ms。
  - 应用算的 CJK 二元组在 GIN 里：2 字 3–40 ms；惰性复查时常见 3–4 字与 `C语言` 28–37 ms；标题任何长度 0.5–2.2 ms。
  - 最常见的单字（"的"）在正文里 1.2–1.8 秒。
  - 写的代价只量了 pg_trgm：`fastupdate = off` 每次保存 10 KB 107 ms、200 KB 1.3 秒；`fastupdate = on` 平时便宜，但碰上合并待处理列表的那次保存停 5–9 秒。二元组每页的键是它的 0.64–0.89 倍，没有量。
  - 字母文字 spike 量的是词的前缀（B 的写法，"api" 找不到 "rapid"），把语义留作问题 5；三元组的子串是向负责人提问时推荐的，没有量。
  - `tsvector` 的限制是词的字节之和至多 1,048,575，超过时报错、不截断（审查 A 在 PG 18.6 上核对）：约 17 万个不同的二元组，约 2 MB 以上的中文才到（spike 2.1：2 MB 有 15.7 万个）。

## 2. 目标与范围

做的事对应 M8 设计 4.1、4.12–4.14 的搜索部分与第 3 节第 6 条：

- **实测（先做）**：在 spike 的语料上量本文的方案与两种字母文字的写法，定下写法与界线（3.9）。
- **迁移**：`page_search`（每个活着的节点一行：标题与正文的词）与 `page_search_queue`（要重建的节点），已有的节点以"批量"入队。
- **分词**（page 的 domain）：页的词、查询的词、复查与定位的模式（大小写在模式里）、高亮的位置。
- **索引的维护**：
  - 单元在事务里把碰过的节点放进队列：用户的写是"准确"的（读时当作候选），导入的批是"批量"的（不当候选，结果标明可能不全）。笔记本删除、`nervewiki reindex` 同样入队。
  - 一个 River 定时任务在请求之外重建它们的行，同时只有一个在跑。
- **搜索**：
  - 用例 `Search`：标题与正文两层（名称先查、正文有界地走查）、对当前标题与正文的复查、按层的约数、摘要与高亮（UTF-16）、路径与附件的信息、"索引正在建立"的标记。
  - 动作 `page.search`；接口 `searchWorkspace`；page 的端口 `VisibleNotebooks`，`Workspaces` 加 `FindBySlug`。
- **测试**：可见性的集成测试、增量等于重建与"复查蕴含键"的性质测试、交错、矩阵、计划与成本测试；产品的基准在 30 万页上复现实测。
- **前端**：
  - 左栏的搜索框（带范围）、`Mod+Shift+F`、输入法组合中的回车与快捷键不生效。
  - 结果页：范围、约数、高亮、摘要、"更多"与"继续查找"、"索引正在建立"，附件打开到它所在的位置；再次提交重新读。
  - e2e SE1、SE2；输入法清单第 21 步。

不做：

- 相关度排序、词典分词、拼音、模糊匹配、繁简互通、NFKC（全角与半角互通）、不区分重音（cafe 找不到 café）、附件的内容、正则（M9 的 `grep`，第 8 节）、跨工作区。
- 路径（祖先的名称）参与匹配：只匹配节点自己的名称与页的正文。
- 回收站里与历史版本里的搜索。
- 搜索的实时推送与重读：结果页不随事件、焦点、重连重读，用户再提交时才读（4.3）。
- 搜索的并发上限：M12 压测之后再看。

### 2.1 对 00 号文档的修订

P5 的文档提交随这份文档一起改 [`00-M8-design.md`](00-M8-design.md)。行号指 `422abd35` 的（与 `1f732c96` 相同），理由在本文各节。

1. **负责人的三个决定**（第 11 节第 10 条 :392；4.12 :211–223）：

   负责人 2026-10-14 答复 spike 的三个问题："搜索三个问题都按你的建议来"。

   - 单个汉字只搜标题，不搜正文。
   - 字母文字 3 个及以上按子串匹配（三元组），1–2 个按整词。
     - 子串是向负责人提问时推荐的，spike 没有量过（spike 量的是词的前缀，"api" 找不到 "rapid"）。P5A 先按 3.9 实测它是否达到目标，不达到时退回词的前缀。
     - 由 P5A 按数字定下并写回 4.12 与本文；两种都不达标才再问负责人。
   - 结果的数目是约数（"约 N 条"、"1000+"）。

   第 10 条改为："搜索列出附件（只按名称），不列回收站里的。单个汉字只搜标题；汉字 2 个起、字母与数字 3 个起按子串，字母与数字 1–2 个按整词（负责人 2026-10-14；字母的写法按 P5A 的实测；数字与字母同样处理是设计的扩展）。不做相关度排序：名称里有全部的词的排在前面，其余的在后，各按最后改动的时刻从新到旧。数目是约数。用户的写立即反映在结果里；导入、升级之后与 `nervewiki reindex` 之后由后台建索引，期间结果页写明'索引正在建立，结果可能不全'。"
2. **4.12 整节改写**（:211–223）：改为方案的摘要，DDL、SQL 与数字指向本文。
   - 应用算的词（CJK 的单字只在标题里、二元组、字母文字的三元组或词、1–2 个的整词）存在 `page_search` 的两个 `tsvector` 列里，各有 GIN 索引，每行记下分词的版本（3.2、3.3）。
   - 单元在事务里只把碰过的节点放进 `page_search_queue`，后台的任务在请求之外重建行，同时只有一个（3.4）。
   - 读：
     - 候选是索引的命中加上队列里"准确"的节点（用户写的），连活着的节点、限看得到的笔记本。"批量"的节点（导入、升级的补入、reindex）不当候选，答复带 `building`。
     - 名称层先查全部候选的名称；正文层按 `(最后改动, id)` 倒序，每次请求至多走查 `MaxSearchWalk` 个页的正文，对当前的名称与正文复查（3.5）。
     - 所以用户写过的按已提交的状态是准确的：删除之后立即消失，恢复、新写的立即找到。例外（3.3）：超过 64 个字母的连续串与长的 `data:` 串里的子串、一页超出约 1 MB 词的部分，只在排队时找得到。
   - 两层：每个词都在名称里的在前，其余在后。
   - 按层的约数（`exact`、`about`、`at_least`）、摘要与高亮的位置（UTF-16），`q` 的上限与转义（3.3、3.5）。
   - 目标与实测的数（3.9，P5A 写回）。
   - 已定的部分（接口、权限、结果的形状、附件只按名称、`q` 进访问日志、`Ctrl+O` 不变）照留，结果多出 `count` 的种类、`building` 与摘要的截断标记（3.7）。
3. **端口**：
   - 第 1 节 :30 的"给 page 的'看得到的笔记本'与'工作区与调用者的角色'"改为"给 page 的'看得到的笔记本'与'按 slug 找工作区'"。
   - 4.1（:66、:77）的"从 slug 得到工作区与调用者的角色"改为：slug 经 page 的 `Workspaces` 端口加的 `FindBySlug`（`workspace.NewWorkspaces` 已有）；角色取判定的 `Grant.WorkspaceRole`，同 notebook 的 `ListNotebooks`。`Deps` 去掉 `WorkspaceRoles`，加 `VisibleNotebooks`（交空时 `page.New` panic）与 `SearchInterval`。
4. **第 2 节"不做"**：
   - :42 加"繁简互通、不区分重音、搜索的并发上限"。
   - :43 的"列表随 `pages` 事件按间隔重读"加"搜索的结果页除外：不随事件、焦点、重连重读，再提交时读（4.13）"。
5. **第 3 节第 6 条**（:54）：
   - 目标写成 3.9 的表：各类查询第一页与"更多"的 p99、建索引时与积压时的同上、索引的体积、任务的吞吐与从空建完的时间、保存时多出的开销、计数的误差。
   - "删除之后消失、恢复之后回来"加"立即（用户的写）"。
   - 加"增量维护的索引等于重建"（13.4 第 10 条）与"复查蕴含索引的键（3.3 的例外之外）"。
6. **4.13**：
   - 搜索一条（:233）改为 B 的设计（4.2–4.4）：
     - 搜索框在工作区左栏的顶部，带范围的提示：顶栏在工作区之外。
     - 组合中的回车与 `Mod+Shift+F` 不生效。
     - 结果页的范围选择、约数、"继续查找"、"索引正在建立"、高亮是 `<bdi>` 里的 `<mark>`。
     - 结果页不随事件、焦点、重连重读；再提交同样的搜索时重读，后退显示缓存。
   - 列表的模式（:234）的"四个列表共用"改为"五个"（加搜索的结果）。
7. **4.14**：
   - 码的次序加 `searchWorkspace`（3.6）。
   - 上限（:249）加 `limit` 1–50、每次走查 `MaxSearchWalk`、计数到 1,000、查询里连续的字母至多 64 个。
   - 日志（:251）加"搜索语句的错误只记 SQLSTATE"。
   - 加锁加队列的行（3.8）；交错加 3.8 的九种。
8. **第 5 节**（:269）：`searchWorkspace` 一行写明参数（`q`、`notebook_id`、`cursor`、`limit`）与答复（`data`、`next_cursor`、`count`、`building`）。
9. **第 7 节 P5 一行**（:289）：交付加队列与后台任务、reindex；验证改为"3.9 的目标在 30 万页上复现（产品的基准，手动），守卫在不带竞态检测的一步；可见性的集成测试；增量等于重建、复查蕴含键；转义与上限；e2e SE1、SE2"。
10. **"移交的落实" M0/P1 一行**（:297）：落实改为"spike（2026-10-13）实测，P5A 在产品的方案上再实测（P5 3.9）"。
11. **"写给后面的 M"**：
    - M9（:308）的"`grep` 用 4.12 的索引"加：从正则的字面片段取键、候选并上队列里准确的节点、SQL 里以正则复查；词的前缀时只能有界地扫（第 8 节，新的移交）。
    - M12（:311）加"搜索在更大的语料与并发下的压测、任务的吞吐与积压、计数的误差、繁简映射（P5，第 17 项）"：编号接在 P4 的第 16 项之后，P5 先合并时由后合并的一方改号。
12. **第 8 节**：没有新的扩展点（队列是 page 自己的表）。
    - :337 的"`VisibleNotebooks`、成员资格接给 page"改为"`VisibleNotebooks`、`FindBySlug` 接给 page"。
    - "改前面 M 的扩展点与代码"加：page 的 `Workspaces.FindBySlug`、`Deps.VisibleNotebooks`；单元入队（导入的单元以批量）；笔记本删除入队；M6 的 `nervewiki reindex` 加上搜索。
13. **第 9 节**（:347、:359–360、:362）：
    - 搜索的测试一条加：队列与任务的不变式 `checkSearch`、增量等于重建、复查蕴含键、交错、单一的任务。
    - SE1、SE2 照 4.6 写明：单个汉字只找到标题、两个字母按整词、约数、附件、同一个标签页里再搜一次。
    - 人工一条写明第 21 步。
14. **第 10 节**（:368）：中文搜索一行改为实测之后剩下的局限：
    - 一页不同的词的字节超过约 1 MB（约 2 MB 以上的中文）时超出的部分不进索引；超过 64 个字母的连续串、长的 `data:` 串不进索引。这些只在排队时找得到。
    - 繁简、重音、全角与半角不互通；正文里不是 NFC 的组合字符与 ß/ss 这类完全折叠的差别。
    - 导入、升级、reindex 之后建索引期间结果可能不全（页上写明）。
    - 字母文字的写法按实测定。
15. **第 12 节**：P5 一行改为"进行中（设计已审查）"，链到本文与设计审查。
16. **第 13 节**：加一行变更记录（P5 设计与它的审查）。

### 2.2 需要负责人确认

无。

- 三个决定照做。子串（三元组）是向负责人提问时推荐的、spike 没有量过的一种；P5A 先按 3.9 量它，不达标时退回 spike 量过的词的前缀，按数字定下、写回，不另问。两种都不达标才问负责人。
- 下面三条是设计里的取舍，写进第 11 节第 10 条，负责人可以改判：
  - 两层的次序（名称里有全部的词的在前）是固定的规则，不打分，属于"不做相关度排序"之内。
  - 数字与字母同样处理（1–2 位按整词：`20` 找不到 `2020年`）：决定里只说了字母。
  - 用户的写立即反映；导入、升级的补入、reindex 由后台建索引，期间不当候选，结果页写明"索引正在建立，结果可能不全"。否则一次大的导入会让每次搜索走查它的几万页（3.4）。
- 剩下的局限（1 MB、64 个字母、`data:`、繁简、重音、NFC 与 ß、建索引期间不全）写进 4.12 与第 10 节。

**结果**（P5A）：三元组（T）在平时每个字母的类都达到 p99，P 在常见的词上 2.5–3.7 秒，照规则用 T，没有再问负责人（3.9）。300 ms 的 p99 在重的类上没有全部达到：主持者接受、移交 M12，负责人可以改判（3.9）。剩下的局限按实施改：64 的界线只对 ASCII 的字母与数字，泰文等的长段照常进索引；不是 NFC 的正文按它的 NFC 读（3.3）。另由主持者定下（负责人可以改判）：后台的恢复与撤销同导入以批量入队，期间不当候选、答复带 `building`（3.4，P5A 审查 C-I1；写进 00 号文档第 11 节第 10 条）。

## 3. A：服务端

### 3.1 文件

| 文件 | 内容 |
|---|---|
| `server/migrations/sql/00040_page_search.sql` | `page_search`、`page_search_queue`、序列与索引；函数 `page_search_hits`（正文层的候选只经两个 GIN 索引读，偏离 1，3.2、3.5）；已有节点以批量入队（3.2） |
| `deploy/runtime-grants.sql`、`server/sqlc.yaml` | 两张表的 DML、序列的 `USAGE`，`page_search` 的 `MAINTAIN`（任务的 `ANALYZE`）；函数经 PUBLIC 默认的 `EXECUTE`；page 的迁移列表 |
| `page/domain/search.go` | 分词 `SearchTokens(name, content)`（标题、正文两份）与版本 `SearchTokenizer`（3.3）；`ParseSearchQuery`（`SearchQuery`、`SearchTerm`：键、只搜标题、ARE 与 RE2 两种写法的模式）；界线：`MaxSearchQueryBytes` = 256、`MaxSearchTerms` = 8、`DefaultSearchLimit` = 20、`MaxSearchLimit` = 50、`MaxSearchWalk` = 500、`MaxSearchCount` = 1,000、`MaxSearchCountScan` = 20,000、`SearchCountSample` = 100、`SearchCountPasses` = 2、`MaxSearchLexemeBytes` = 1,000,000、`MaxSearchWordRunes` = 64、`MinSearchRebuild` = 30 秒，任务的一批 `SearchBatchNodes` = 100、`SearchBatchBytes` = 8 MiB、`SearchAnalyzeRows` = 10,000、`SearchRunBudget` = 1 分钟（3.3–3.5、3.9）；`SearchLimit(limit)` |
| `page/domain/search_hits.go` | 高亮：在名称与摘要里按词的规则找命中，答排好、不相交的 UTF-16 区间；摘要的上下文字符、空白合并与截断；`SearchCursor{Tier, At, ID}`；计数的估计 `EstimateCount`（`SearchTally`） |
| `page/domain/errors.go`、`actions.go` | `ErrWorkspaceNotFound`（码 `workspace.not_found`）；`ActionSearch = "page.search"` |
| `page/app/ports.go` | `Workspaces.FindBySlug`；`VisibleNotebooks`；`Snapshots`（搜索的只读快照，偏离 6）；`SearchQueue`（单元的入队）；`SearchIndex`（取锁、队列的一遍、来源、正文、写行、删行、出队、`ANALYZE`）；`SearchReads`（名称层、命中数、正文层、两层的计数、`building`）；`AttachmentInfos` 已有 |
| `page/app/unit.go`；`import_writes.go`、`jobs.go` | `Run` 在 `do` 之后、`tell` 之前 `queueSearch`（3.4）；`UnitSpec.Background` 标出导入与后台的恢复（`restoreUnit`）、撤销（`revertUnit`）的单元，它们以批量入队 |
| `page/app/extension.go`（`NotebookDeletion.NotebookDeleted`） | 这次删除的节点入队 |
| `page/app/index_search.go` | `IndexSearch.Drain`：任务的一次运行（3.4） |
| `page/app/search.go` | 用例 `Search`（3.5、3.6） |
| `page/adapter/postgres/queries/search.sql`、`search.go` | 队列、任务、两层、命中数、两层的计数与 `building` 的语句；搜索的语句经 `planned(ctx)`；任何错误只留 SQLSTATE（`statementError`）；锁在自己取的连接上，一次运行的语句都在它上面（3.4） |
| `page/adapter/http/search.go`、`handler.go`；`trash.go` | `searchWorkspace` 的处理器；`gen.TrashAttachment` 改名 |
| `page/adapter/river/search.go` | `SearchIndexJob`：kind `page.index_search`，队列 `page_search`（1 个 worker），唯一（不含 `completed`），一次运行只试一次，时限 2 分钟，每隔 `page.search_index_interval`，启动时一次；启动时丢掉停下的服务留在运行中的行（3.4） |
| `page/module.go`、`page/search_index.go` | `Deps.VisibleNotebooks`、`Deps.SearchInterval`（0 时不注册任务，只在测试里）、`Deps.Snapshots`；`Jobs()` 加任务；`SearchQueue` 常量；`page.NewSearchIndex(pool, commitTimeout)`（偏离 7；reindex 的 `Requeue`，测试的 `Drain`） |
| `access/domain/rules.go` | `page.search`：工作区级，每种角色 |
| `platform/config`、`configs/config.yaml` | `page.search_index_interval`（默认 5 秒，1 秒到 10 分钟）；连接池的规则算上搜索的任务的一个连接（`SearchJobConns`，3.4） |
| `platform/jobs/jobs.go` | `Job.DiscardRunning`：runner 启动时丢掉这种任务留在 `running` 的行（3.4） |
| `platform/postgres/tx.go`、`planned.go` | `OnConn(ctx, conn)`：`DB(ctx, pool)` 与 `TxManager` 开的事务都在这个连接上（3.4）；`Querier.SendBatch`（任务的一批 upsert，偏离 5） |
| `bootstrap/deps.go`、`reindex.go`；`cmd/nervewiki/reindex.go` | 接线：`VisibleNotebooks`、`Snapshots`、`Queues` 加 `page_search: 1`；reindex 在 linking 之后让这本笔记本里没有行或行的分词版本不同的节点以批量入队，每本一行写明入队的数目 |
| `api/modules/page.yaml`、`api/openapi.yaml` | `searchWorkspace` 与结构；`TrashAttachment` 改名 `AttachmentInfo`（同形，搜索也用） |
| `bootstrap/search_test.go`、`search_rebuild_test.go`、`interleavings_search_test.go`、`page_visibility_test.go`、`permission_matrix_page_test.go`、`permission_matrix_seeded_test.go`（`queueSeeded`）、`search_bench_test.go`、`save_bench_test.go`（偏离 13）；`page/domain/search_test.go`；`page/app/search_test.go`、`search_queue_test.go`、`index_search_test.go`；`page/adapter/postgres/search_test.go`、`search_plans_test.go`、`search_cost_test.go`、`search_guard_test.go`（守卫的时限表，没有构建标签）；`page/adapter/http/search_test.go`；`page/adapter/river/search_test.go`；`platform/jobs`、`platform/postgres`、`platform/config` 的测试；`migrations/search_test.go`；`Makefile` | 3.10、3.9 |

### 3.2 迁移

**`00040_page_search.sql`**（归 page，13.1 第 7 条）：

（实施的样子，注释从简；以 `00040_page_search.sql` 为准。）

```sql
-- +goose Up
-- A node's search lexemes (M8/P5 design 3.2): computed by the app (domain.SearchTokens) from the node's name and a
-- page's content as the search's job last read them. A derived table as linking's are: no foreign key, no
-- deleted_at, no purger (v0.1 design 13.1, item 6). The job keeps a row for each live node and removes a node's
-- row once the node is deleted or gone; reads join the live nodes, so a stale row is never an answer.
CREATE TABLE page_search (
    node_id   uuid PRIMARY KEY,
    -- The page's content revision the body was built from; null for an attachment, which has no body.
    revision  integer CONSTRAINT page_search_revision_check CHECK (revision >= 1),
    -- The name the lexemes were built from.
    name      text NOT NULL,
    -- The tokenizer the lexemes are of (domain.SearchTokenizer: its own version, Go's Unicode version and x/text's):
    -- a row of another is rebuilt once nervewiki reindex queues it, as linking's extractor.
    tokenizer text NOT NULL,
    -- When the job last built the row: it builds it again no sooner than domain.MinSearchRebuild after.
    built_at  timestamptz NOT NULL,
    -- The name's: CJK unigrams and bigrams, the word runs' trigrams, the short words.
    title     tsvector NOT NULL,
    -- A page's name and content's: CJK bigrams, the word runs' trigrams, the short words.
    body      tsvector,
    CONSTRAINT page_search_body_check CHECK ((revision IS NULL) = (body IS NULL))
);
CREATE INDEX page_search_title_idx ON page_search USING gin (title);
CREATE INDEX page_search_body_idx ON page_search USING gin (body);

-- The body tier's candidates by their lexemes, read through the GIN indexes alone (M8/P5 design 3.5, 3.9).
-- +goose StatementBegin
CREATE FUNCTION page_search_hits(body_keys tsquery, title_keys tsquery) RETURNS SETOF uuid
    LANGUAGE sql STABLE PARALLEL RESTRICTED ROWS 1000000
    SET enable_seqscan = off SET enable_indexscan = off SET enable_indexonlyscan = off SET work_mem = '64MB'
BEGIN ATOMIC
    SELECT node_id FROM page_search WHERE title_keys IS NULL AND body @@ body_keys
    UNION ALL
    SELECT node_id FROM page_search WHERE title_keys IS NOT NULL AND body @@ body_keys AND title @@ title_keys
    UNION ALL
    SELECT node_id FROM page_search WHERE body_keys IS NULL AND title @@ title_keys;
END;
-- +goose StatementEnd

-- The nodes whose row may be stale (M8/P5 design 3.4): a unit queues each node it creates, restores, renames,
-- deletes or writes the content of, in its transaction; the job takes them off. Each queueing takes a new version
-- from the sequence, so the job takes a node off only as it read it. A bulk node (an import's, a background
-- restore's or revert's, the backfill's, nervewiki reindex's) is no candidate of a search, which says the index is
-- being built; a user's write makes it exact.
CREATE SEQUENCE page_search_queue_version_seq;
CREATE TABLE page_search_queue (
    node_id     uuid PRIMARY KEY,
    -- The node's notebook, which never changes: a search reads its notebooks' queue only.
    notebook_id uuid NOT NULL,
    version     bigint NOT NULL DEFAULT nextval('page_search_queue_version_seq'),
    bulk        boolean NOT NULL
);
CREATE INDEX page_search_queue_notebook_id_bulk_idx ON page_search_queue (notebook_id, bulk);
-- The live nodes so far, in bulk: the job builds their rows once the server runs.
INSERT INTO page_search_queue (node_id, notebook_id, bulk)
SELECT id, notebook_id, true FROM nodes WHERE deleted_at IS NULL ORDER BY id;

-- +goose Down
DROP FUNCTION page_search_hits(tsquery, tsquery);
DROP TABLE page_search_queue, page_search;
DROP SEQUENCE page_search_queue_version_seq;
```

- **侧表，不是 `page_contents` 的列**：
  - spike 的 B 把词放在 `page_contents` 里，正文与词的 TOAST 混在一起，6.2 GB 对 1.6 GB，正文的读都跟着变慢（spike 3 第 7 条 (a)）。
  - 分词只有一份，在 Go 里；表达式索引要另写一份 SQL 的分词，不走索引的计划会逐行分词。
- **`page_search` 不建 `notebook_id`、`kind`**：读时连 `nodes`，活着、可见、类型、名称都以它为准。队列的行带 `notebook_id`：搜索只读它的笔记本的队列，一个工作区的积压不拖慢别的工作区。
  - 所以命中不按范围：限定一本笔记本的搜索也读整个服务器的命中，连 `nodes` 之后才留下范围里的（P5A 的未决项 9：语料上一本约 1,500 页的笔记本里最重的第一页中位数 110–150 ms，大半是别的笔记本的命中；跨工作区的计时，P5A 审查 C-S1）。节点不跨笔记本移动，这一列不会变旧；要改这张表、重量语料，移交 M12。
- **版本号取自序列**：重新入队也取新的，同一个 `(节点, 版本)` 不会再出现（没有 ABA）。版本号的更新不改索引的列，是 HOT 的；`bulk` 至多由真变假一次。
- **`tsvector` 不带位置**，由 `array_to_tsvector(@lexemes::text[])` 写。一行的词的字节之和至多 `MaxSearchLexemeBytes`（3.3）。
- **GIN 的 `fastupdate` 用默认（开）**：
  - 只有任务写这两个索引，待处理列表的合并由任务付，请求从不碰它。
  - 读多扫至多 4 MB 的待处理列表（`gin_pending_list_limit` 的默认）。
  - 开与关由 3.9 实测定：留着开。关掉时 10 KB 的页的新行 37 ms（开时 1.8 ms），满的索引上任务只有 23 行/秒。
  - 任务的写在自己的事务里以 `work_mem` 64 MB、`gin_pending_list_limit` 16 MB（3.4，偏离 3）：读至多多扫 16 MB 的待处理列表。
- **授权**：两张表的 DML、序列的 `USAGE`；`page_search` 加 `MAINTAIN`，任务大量写之后 `ANALYZE`，同导入。函数经 PUBLIC 默认的 `EXECUTE`，授权文件不列。
- **`page_search_hits`**（实施时加，偏离 1，3.9 第 1 步）：正文层的候选经这个函数读。它的 `SET` 关掉表的整表扫描与按键的扫描，只剩两个 GIN 索引的位图：照 3.5 原样的语句在规划器的两种读法几乎持平时（建索引中、待处理列表满）整表扫描 `page_search`、逐行解压每一行的正文的词（3.7 GB 的 TOAST），5–40 秒。
  - `work_mem` 64 MB：位图过了 `work_mem` 就把页变成有损的，复查有损的页的每一行要从 TOAST 读正文；默认的 4 MB 时几乎每页都有的键约在 200 万行有损，64 MB 时约 3,000 万行（P5A 审查 C-M3）。位图随它增长才占内存，30 万页约 0.6 MB。
  - `ROWS 1000000`：语句以同一快照里数出的命中数作它的 `LIMIT`，即规划器的估计，`LIMIT` 从不截断（3.5）。`PARALLEL RESTRICTED`：worker 拿不到它的设置。设置随函数结束，外面的语句照常规划。
- **实施时**：队列的索引名 `page_search_queue_notebook_id_bulk_idx`（仓库的命名规则，`TestConstraintAndIndexNames`，偏离 4）；`revision` 的约束命名 `page_search_revision_check`。迁移测试另核对函数在 Up 之后在、Down 之后不在，反例加版本号 0。
- **迁移测试**：
  - `schema_test.go` 核对表、约束、序列与索引的名字。
  - `TestChecksRejectCounterexamples` 加反例：附件带 `body`、页没有 `body`。
  - Up 带数据跑：活着的节点以批量入队、版本各不相同，已删的不入。
  - Down 带数据跑一次。

### 3.3 分词：页的词与查询的词

**规整与字符的类**（`domain/search.go`，照 spike 的 `tok.go`）：

- 词用 `searchFold`：`NFC(Fold(NFC(s)))`（`shared.TitleKey`），之后把切罗基文的小写换成大写（U+AB70–ABBF → U+13A0–13EF，U+13F8–13FD → U+13F0–13F5）。x/text 的完全折叠把切罗基文的大小写对调，Unicode 的折叠两者都到大写：不换时一种大小写的查询的键是另一种的词，排队时找到、建好之后找不到（P5A 审查 A-M1）。复查与高亮看正文的 NFC（下）。
- **CJK**：`unicode.Han`、`Hiragana`、`Katakana`、`Hangul`、`Bopomofo`，以及 `ー`、`ｰ`、`〆`。
- **字母文字**：字母（L）、数字（N）与组合记号（Mn、Mc、Me），不是 CJK 的。拉丁、西里尔、希腊、泰文、天城文等都算（天城文的元音符号是 Mc，算作分隔会把"हिन्दी"切开）。Mn、Mc、Me 都连成一段（`TestSearchTokens`：`हिन्दी` 是一段）。
- 其余（空白、标点、符号、emoji）是分隔。
- 一段连续的 CJK 是一个 CJK 段，一段连续的字母文字是一个字母段。段的长短（1–2 个、3 个起、64 的界线）按完全折叠之后的字符数算，模式的边界用同一个分类。
- **不进索引**：
  - 只由 ASCII 的字母与数字组成、超过 `MaxSearchWordRunes` = 64 个的字母段（base64、十六进制一类的长串：每个字符一个字节）。别的文字的长段照常以三元组进索引：泰文、老挝文、高棉文、缅甸文词之间不留空格，过 64 个码位的分句就是一个字母段；初稿的界线对任何字母段，这些文字的大半正文建好之后谁也找不到（P5A 审查 A-I1）。
  - 以 `data:` 开头、长过 64 个字符的串（Markdown 里内嵌的图片），到第一个 data 地址不会有的字符为止：空白、非 ASCII 的字符，或 `)`、`"`、`'`、`<`、`>`、`]`、反引号。它的 base64 切成很多短段，会把一页的词的预算用在没有意义的串上。初稿到空白为止，把图片的 `)` 之后紧接的整段中文一起丢掉（P5A 审查 A-M3；反引号由修复二轮加上，修复核对第一轮 N-7）。

**页的词**（`SearchTokens`），同一个函数给任务用，也给测试的重建用：

| | 标题（`title`：名称） | 正文（`body`：名称 + 换行 + 正文，只有页） |
|---|---|---|
| CJK 段 | 每个字（单字）与相邻两字（二元组） | 二元组 |
| 字母段，3 个及以上 | 每三个相邻的（三元组；3.9 实测定为 T） | 同标题 |
| 字母段，1–2 个 | 整个字母段 | 同标题 |

- 正文里不收单字：负责人的第一个决定。spike 量过，"的"在正文里 1.2–1.8 秒，去掉它还省下 0.93 + 0.24 GB。
- 正文里也收名称的词，所以"每个词在名称或正文里"一次 `@@` 就能找出候选（3.5）。
- 词按第一次出现的次序收，字节之和到 `MaxSearchLexemeBytes` = 1,000,000（PostgreSQL 的界线是 1,048,575，超过时报错）为止，其余丢掉。
  - 约 16 万个不同的二元组，约 2 MB 以上的中文才到；正文至多 5 MiB，所以这是已知的局限，写进 4.12 与第 10 节。30 万页的语料里没有到界线的页（3.9）。
- `SearchTokenizer` 是分词的版本、Go 的 Unicode 版本（字符的类）与 x/text 的 Unicode 版本（折叠与 NFC），现在是 `2/17.0.0/17.0.0`，写进每一行。版本 2 是 P5A 审查的修复改的（64 的界线、`data:` 的末尾、切罗基文），x/text 的版本由审查 A-N5 加上：同一个 Go 下升级 x/text 也会换表。分词、Go 或 x/text 的 Unicode 数据变了时，发布说明要求执行 `nervewiki reindex`，同标题键与 linking 的提取规则。
- 代价（3.9，任务在满的索引上，p50）：分词 10 KB 0.2–0.3 ms、200 KB 7–9 ms、1 MB 22–26 ms、5 MiB 76–86 ms。

**查询**（`ParseSearchQuery(q)`，与资源无关，在找工作区之前，3.6）：

- **取值**：去掉首尾空白之后 1–256 字节；是合法的 UTF-8、没有 NUL；按 Unicode 空白（含全角空格 U+3000）切成词，按规整之后的形式去重（折叠相同的词是一个，取第一个的写法：`ß ss` 是 `ß`），至多 8 个；词里连续的 ASCII 字母与数字至多 64 个（更长的串不进索引，永远找不到；别的文字的长段照常）。
- **错误**：不合时答 422 `validation_failed`，字段 `q`：缺了或为空是 `required`，太长、太多词或连续的 ASCII 字母与数字超过 64 个是 `too_long`，坏的 UTF-8 或 NUL 是 `invalid_format`，全部的词都没有键是 `not_allowed`。
- **每个词分成段**，得到它的键与模式：
  - **键**：
    - 2 个及以上字的 CJK 段给二元组。
    - 3 个及以上字母的段，T 给三元组；P 给整段，词末的一段作前缀（`'linux':*`）。
    - 1–2 个字母的段给整段。
    - 只有一个字的 CJK 段给单字，只作标题的键。
  - **只搜标题的词**：有键的段只有一个、是一个汉字（"锅"、"锅！"）。别的词里的单字段只给标题的键：`A股` 在正文层以 `a` 找候选，在标题层以 `a` 与 `股`。
    - 几个被标点隔开的单字的词（`中·国`、`タ・ネ`）只给标题的键，对别的词的正文键找到的候选在名称或正文里复查：`中·国 数据库` 以 `数据库` 的键找候选，`中·国` 在名称或正文里。实施的第一版把它们也当作只搜标题（偏离 8），正文里有它的页找不到（P5A 审查 A-M2），修复改回这条规则。
    - **单独的情形**：整个查询都没有正文的键时（没有字母、数字、组合记号，也没有两个相连的汉字），带单字的词都只搜标题（正文层的 `title_keys`，只在名称里找）：单独的 `中·国` 只找名称。由 `ParseSearchQuery` 定，契约的 `q` 写明。
  - **没有键的词**（只有标点或符号，如 "#"）照样复查，只是不进索引。全部的词都没有键时答 422。
  - **模式**：词的 NFC 原文，外面一个捕获组。
    - **大小写在模式里**：每个有大小写的字符写成它的 `unicode.SimpleFold` 轨道的方括号（`ß` → `[ßẞ]`，`σ` → `[σςΣ]`，`k` → `[kKK]`），别的字符转义。PostgreSQL 用区分大小写的 `~` 与不带 `i` 的 `regexp_instr`，RE2 不带 `(?i)`：两边拿到同一个不用再折叠的模式，按构造一致，也不依赖 PostgreSQL 的 Unicode 版本。
    - T：开头是 1–2 个字母的段时，左边要有边界；结尾是这样的段时，右边要有边界。
    - P：开头的字母段都要左边界，结尾 1–2 个字母的段要右边界。
    - 边界是文字的开头、结尾，或一个不是字母文字的字符（CJK 算边界）。
  - **两种写法由同一个构造器给出**：PostgreSQL 的 ARE，给 `~` 与 `regexp_instr`；Go 的 RE2，给高亮。CJK 与字母文字的范围由 Go 的 `unicode` 表生成成方括号里的区间（审查 A 核对过 863 个区间的类在 ARE 里能编译，在 110 万字符上多约 1 ms）。
- **`tsquery`**（文字由 Go 写，词里只有字母、数字与组合记号，没有要引的字符；以 `::text::tsquery` 传）：
  - 标题层：全部词的标题键相与。
  - 正文层：不是只搜标题的词的正文键相与（`body_keys`），加上只搜标题的词的单字在标题里（`title_keys`，实施的参数名 `titled_keys`）。两者至少有一个（单独的情形保证这一点）；每个词都只搜标题时这一层不跑。
  - 所以 "锅 #" 的正文层候选是标题有"锅"的页，再复查 "#" 在名称或正文里。

**语义**（写进 4.12 与契约的说明）：

| 词 | 例 | 在哪里找 | 怎样算命中 |
|---|---|---|---|
| 单个汉字 | 锅 | 只在名称 | 子串 |
| 2 个及以上汉字 | 中国、数据库 | 名称或正文 | 子串 |
| 3 个及以上字母或数字 | api、latency、2020 | 名称或正文 | 子串（api 找到 rapid；3.9 实测定为 T） |
| 1–2 个字母或数字 | ai、go、C、20 | 名称或正文 | 整词（`Go语言` 的 Go 是整词：汉字算边界；`20` 找不到 `2020年`） |
| 混合 | Go语言、C++、node.js、A股 | 名称或正文 | 整个词作子串，两端 1–2 个字母的段另要边界 |
| 被隔开的单字 | 中·国 | 名称或正文；查询里没有正文的键时只在名称 | 整个词作子串 |
| 几个词 | 中国 历史 | 每个词各在名称或正文里 | 全部命中 |

- `%`、`_`、`\` 与正则的元字符照字面匹配（转义）。
- 大小写不敏感是对称的：同一个轨道的字符互相找得到（"ΛΟΓΟΣ" 找到 "λογος"，"straße" 找到 "STRAẞE"）。完全折叠的展开不互通："strasse" 找不到 "straße"。全角与半角、繁简、重音不互通。
- **不是 NFC 的正文**（组合的声调、谚文的字母分开写，少见；编辑器写 NFC）按它的 NFC 读：复查、第一处命中的位置、摘要的切与正文的字符数都对 `normalize(content, NFC)`，存的正文不动（3.5）。所以 NFC 的写法与分解的写法都找得到它，只有基本字母的词找不到（"cafe" 找不到分解的 "café"），排队时与建好之后一样。初稿对原样的正文复查：查询总被规整成 NFC，哪种写法都找不到它，按基本字母的命中建好之后消失（P5A 审查 A-M4）。
- **键与复查**：命中的定义是"键在索引里，且复查通过"。同一轨道的字符键相同（切罗基文靠 `searchFold` 的换，`TestEachOrbitKeysAlike` 走遍每个简单折叠的轨道），所以复查通过时查询的键都在这一页的词里。例外：
  - 不进索引的部分：超过 64 个的 ASCII 字母与数字的段、长的 `data:` 串、超出 `MaxSearchLexemeBytes` 的词。
  - 大小写的折叠改变了之后一个组合记号怎样组合的字符（偏离 11；P5A 审查 A-N1 穷举 NFC 稳定的"字符 + 组合记号"，956 对、96 个字符）：`İ` 加不与 `I` 组合、组合类小于 230 的记号；`H` 一类加 U+0331、`J` 加 U+030C、`T` 加 U+0308、`W`、`Y` 加 U+030A（小写有预组合的形式、大写没有）；`ß`、`ẞ`、`ſ` 加与 `s` 组合的记号；`ŉ` 加记号；希腊大写加 perispomeni 或 psili；带 iota 下标的字母加第二个记号。止于这个字符或起于这个记号的词漏找；真实的文字里没有。
  - 这些在节点排队时由复查找到，建好之后找不到；写进 4.12 与第 10 节，性质测试在例外之外核对（生成器去掉 `İ` 加低组合类的记号并说明，3.10）。
- **按设计的召回的缺口**，不破坏上一条（模式是字面的）：软连字符（U+00AD）、ZWJ 与 ZWNJ、汉字的异体字选择符把一个词切开（`葛城` 找不到 `葛` + 选择符 + `城`）；带变体选择符的 emoji（`❤️`）以选择符为键，单独的 `❤` 答 422 `not_allowed`（P5A 审查 A-N8，两边一致）。

### 3.4 索引的维护：队列与后台的任务

**为什么不在单元里写 `page_search`**（spike 2.4 的数，B 的写法没量过、只能按每页的键比）：

- **每次自动保存**（停 2 秒就是一次）：GIN 每次插入一页的全部键。pg_trgm 的数是 `fastupdate = off` 时 10 KB 107 ms、200 KB 1.3 秒；`on` 时有 5–9 秒的停顿。这段时间持着正文行。而且观察者在 linking 的按笔记本的锁之后，这本笔记本的保存都会排在它后面。
- **导入的一批**（100 页、8 MiB）：照上面的数一批要十几到几十秒，其间持着笔记本行 `FOR NO KEY UPDATE`，挡住这本笔记本的保存。单元"满负载 5 秒"的规则（[02-P2](02-P2-trash.md) 3.11）。
- **界线处的恢复与撤销**同样。

所以单元只把节点放进队列，一条语句，约是写一行；行由后台的任务重建。

**准确与批量**：

- **准确**（`bulk = false`）：用户在请求里的写，即导入与后台任务（恢复、撤销）以外的单元，与笔记本删除。搜索把它们当作候选、对当前的状态复查，所以写了立即反映。
  - 用户的写受请求的速率所限，任务的吞吐（3.9：从空建 285–329 页/秒）高于它；同步的恢复与撤销一次至多 100 页（P2 的页数界线）。所以准确的积压只有几十到几百页。
  - 初稿让后台的恢复与撤销也准确。P4 的任务一次至多 `MaxJobItems` = 60,000 个节点、每秒 360–470 个，快过搜索的任务；恢复的节点多半没有行，又成了最新的候选：之后几分钟里罕见词的搜索答一串空的"继续查找"、不带 `building`（P5A 审查 C-I1、B-M3：2,000 页时第 1–4 页空，8,000 页时第 1–16 页空）。
- **批量**（`bulk = true`）：导入的单元、后台的恢复与撤销的单元（主持者的决定，负责人可以改判）、迁移的补入、`nervewiki reindex`。它们一次可以有几万到几十万个节点：当作候选时，正文层的走查全花在它们上面，每次搜索答一串空的"继续查找"（审查 A-I2）。所以它们不当候选，搜索的答复带 `building`，结果页写明"索引正在建立，结果可能不全"，任务建好之后消失。
- 用户的写让批量的行变成准确的（`bulk` 取两者的与）。
- 任务先做准确的，再做批量的（下）：用户的写从不排在导入与后台任务的几万个之后。

**入队**（`Unit.queueSearch`，`Run` 在 `do` 之后、`tell` 之前，同一事务）：

- 从合并的改动取节点，以下情形入队：
  - 新建：没有"前"，含恢复。
  - 删除：没有"后"。
  - 改名：前后名称不同。
  - 写了正文：`Revision` 不为零。
- 移动与重新排序不入队：路径不参与匹配。
- 后台任务的单元（`UnitSpec.Background`：导入的 `ImportWrites.Import`、后台的恢复 `restoreUnit` 与撤销 `revertUnit`）以批量入队，别的以准确入队，含同步的恢复、撤销与版本的还原。

```sql
-- name: QueueSearch :exec
-- The nodes a unit, a notebook's deletion or nervewiki reindex changed (M8/P5 design 3.4), in ascending id order
-- (v0.1 design 13.1, item 5): queued for the search's job, a node already queued queued again with a new version,
-- so the job's take of an older read misses it; exact unless every queueing of it is bulk.
INSERT INTO page_search_queue (node_id, notebook_id, bulk)
SELECT unnest(sqlc.arg(ids)::uuid[]), sqlc.arg(notebook_id)::uuid, sqlc.arg(bulk)::boolean
ORDER BY 1
ON CONFLICT (node_id) DO UPDATE SET version = excluded.version, bulk = page_search_queue.bulk AND excluded.bulk;
```

- `excluded.version` 是默认值算出的新的 `nextval`。单元传入的 id 也已排序（假的存储核对）。
- **笔记本删除**（page 的 `NotebookDeletion`，`DeleteNotebooksPages` 的事务里）：这次删除的节点入队（`QueueSearchOfNotebooks`：`INSERT … SELECT id, notebook_id, false FROM nodes WHERE notebook_id = ANY(…) AND deleted_at = @at ORDER BY id`，冲突时同上），任务随后删掉它们的行。早就在回收站里的节点不再入队。不走单元，所以另写。
- **`nervewiki reindex`**：每本笔记本在 linking 重建之后（`page.NewSearchIndex(pool, commitTimeout).Requeue`）：
  - 按 id 每批读 1 万个"活着、没有行或行的 `tokenizer` 不是当前的"节点，各一个事务以批量入队（同一条 `QueueSearch`）。
  - 旧的行在重建之前照常回答；当前的行不入队，所以只为 linking 执行的 reindex 不让搜索重建。
  - 命令只入队，由服务的任务重建。每本笔记本一行，以"; N nodes queued for the search"结尾；linking 撞键的那本也照样入队（偏离 10）。
  - 一本的入队失败时，它那一行写明没有入队（"its nodes not queued for the search: …"），命令接着做别的笔记本、最后失败；`--notebook` 点名的那本重建失败时立即失败，带上入队的错误（P5A 审查 B-N2）。
- **清理不碰这两张表**：节点被删时任务已删掉它的行；节点在队列里时被清理了，任务读不到它，删掉行。

**任务**（`app.IndexSearch.Drain`；River 的 `page.index_search`，`adapter/river/search.go`）：

- **调度**：
  - 每隔 `page.search_index_interval`（默认 5 秒）一次，服务启动时一次。
  - `InsertOpts{Queue: "page_search", UniqueOpts: {ByState: available、pending、running、retryable、scheduled}}`：排队与运行中至多一个；不含 `completed`，否则完成的那个挡住之后 24 小时的定时插入（River 0.47 的默认）。
  - 在自己的队列 `page_search` 上，一个 worker：不占默认队列的两个 worker。
  - 一次运行只试一次（`MaxAttempts: 1`，同 transfer 的任务）：失败的运行丢掉，下一次定时的照常排队。River 默认的 25 次时 `retryable` 在唯一的状态里，一次失败按退避挡住之后的定时运行（P5A 审查 B-I1）。
  - **启动时丢掉留在运行中的行**（P5A 审查 B-I1）：唯一的状态必须含 `running`，River 的救援只看 `RescueStuckJobsAfter`（`transfer.job_timeout` 加 1 小时，默认 7 小时）。不干净地停下的进程（OOM、`docker kill`、停机的宽限期短于关闭的次序）留下的 `running` 的行因此挡住之后的每一次运行，其间导入与补入的节点找不到、用户的写积在准确的队列里。平台的 `jobs.Job.DiscardRunning`：runner 启动时、River 取任务或排队之前、各任务的 `Start` 之前，把这种任务留在 `running` 的行丢掉（`discarded`，原因记进它的错误，同 River 的救援；另记一条 WARN 与行数）。只有 `serve` 启动 runner；`nervewiki reindex`、`migrate` 不建任务的客户端，可以在活着的服务旁执行。v0.1 只跑一个服务（注释写明），这样的行不属于活着的运行；同时只有一个运行仍由 advisory lock 保证。
  - 时限 2 分钟（River 默认的 1 分钟会截断最后一批），远小于 `RescueAfter`。
  - `Deps.SearchInterval` 为 0 时不注册任务：只在测试里（`testConfig` 不经校验），测试经 `page.NewSearchIndex(pool, commitTimeout).Drain` 自己跑（3.10）。它取同一把锁，但不等 `MinSearchRebuild`：测试要做空队列；最短间隔由 `page/app` 的测试以假的时钟核对。
- **同时只有一个**：`Drain` 在一个专用的连接上 `pg_try_advisory_lock`（固定的键，同 transfer），拿不到就返回，做完放开；进程退出时连接断开，锁随之放开。两个同时跑会丢更新（审查 A-M4：较早读的那个在较新的之后写回旧的行，节点却已出队），River 的救援、测试与命令里的 `Drain`、第二个进程都可能让它发生。
  - **实施**（P5A 审查 B-M4、B-S3、B-N3）：一次运行的每条语句（读、写与它们的事务、`ANALYZE`、出队）都在取锁的这一个连接上（`postgres.OnConn`：`DB(ctx, pool)` 与 `TxManager` 开的事务都用它）。锁就是这次运行自己的，运行只占池里的一个连接。放开在不带运行的取消的上下文里、5 秒之内（`searchReleaseTimeout`），放不开就关掉这个连接、不还给池。
  - 连接池的规则算上它（`SearchJobConns` = 1，`validate.go`）：导出的 worker n 要 2·(n + 1) ≤ `max_conns`，导入的 E 与导出的 n 要 2·(E + n + 1) ≤ `max_conns`；默认（各一个）时 `max_conns` 至少 6（原来 4）。
- **一次运行**先按 `node_id` 走一遍准确的队列、再走一遍批量的（同一个 1 分钟的预算，P5A 审查 C-I1），每个节点至多读一次，做到队列的末尾或 1 分钟之后不再开始新的一批：
  1. `QueuedSearch`：`bulk = @bulk AND node_id > @after ORDER BY node_id LIMIT 100` 的 `(node_id, version)`，不加锁。
  2. `SearchSources`：按键读这些节点的类型、名称、是否已删、正文的版本号与字节数，以及它们现在的行的 `revision`、`name`、`tokenizer`、`built_at`。要重建的页的正文另一条语句读，一批至多 8 MiB（同导入的批）；超出的留到下一批，`@after` 只前进到做完的最后一个。
  3. 在 Go 里逐个判：
     - 节点不在或已删：删它的行。
     - 行的 `revision`、`name`、`tokenizer` 都是现在的：不动。
     - 行在 `MinSearchRebuild` = 30 秒之内建过：跳过，留在队列里（仍是候选，结果照样准确）。一页编辑中每 2 秒保存一次时，至多每 30 秒重建一次（审查 A-M9）；没有行时立即建。
     - 否则分词、写行。
  4. **事务一**：这一批的写与删，按 `node_id` 的次序（每行一条 `INSERT … ON CONFLICT (node_id) DO UPDATE`，`array_to_tsvector`，经 pgx 的 batch），提交。
     - 事务里先 `SetSearchWriting`：`set_config('work_mem', '64MB', true)`、`set_config('gin_pending_list_limit', '16MB', true)`，只到这个事务的末尾（偏离 3：满的索引上默认的设置重建 38 行/秒，两者都设 136.5 行/秒，3.9）。
     - 失败时这一批逐个节点各一个事务再写一次。还失败的节点记下 id 与 SQLSTATE、这次运行跳过它：它留在队列里（仍是候选），下一次运行再单独试一次。一个写不进的节点不挡住整个队列。
     - 一个节点的事务开始不了（运行的连接断了：FATAL、超过 `database.commit_timeout` 的 COMMIT、`pg_terminate_backend`）时，错误属于这次运行而不属于节点：运行以这一个错误结束，不逐个记"没写"，节点留在队列里，下一次运行在新的连接上接着做（P5A 修复核对第一轮 M-1，修复二轮）。平台的 `TxManager` 给开始不了的事务的错误打标记（`shared.TxNotBegun` 认得出），语句的失败与提交的超时不打；这一批的事务失败之后第一个节点的事务开始不了时，运行的错误两个都带（"the batch's write: …; a node's alone: …"，修复核对第二轮 N2-1，修复三轮）。
  5. **事务二**（单独、很短）：

     ```sql
     -- name: DequeueSearch :exec
     -- The queued nodes the job read at these versions, taken off unless a unit holds or requeued one since: a
     -- skipped row is read again by the next run. The job never waits.
     DELETE FROM page_search_queue q
     USING (SELECT unnest(sqlc.arg(ids)::uuid[]) AS node_id, unnest(sqlc.arg(versions)::bigint[]) AS version) r
     WHERE q.node_id = r.node_id AND q.version = r.version
         AND q.node_id IN (SELECT x.node_id FROM page_search_queue x WHERE x.node_id = ANY(sqlc.arg(ids)::uuid[])
                           ORDER BY x.node_id FOR UPDATE SKIP LOCKED);
     ```

     跳过的与失败的节点不在 `ids` 里。
  6. 自上次 `ANALYZE` 以来写满 10,000 行就 `ANALYZE page_search`，同导入的节拍：`@@` 的选择度靠它的统计。这个计数跨运行累计：满的索引上一次运行常写不到 1 万行（P5A 审查 B-N4）。
- **为什么对**（只有一个任务时）：
  - 任务写的总是它读队列之后读到的节点的状态。之后的写让版本号变大，出队的条件不成立，节点留在队列里。
  - 等锁之后 PostgreSQL 按新的行版本重判 `version` 的条件，同理。
  - 第 4、5 步之间中断：行写了、节点还在队列里，下次读到它与行相符，直接出队。
  - 所以任何时候：每个活着的节点有 `revision`、`name` 是现在的行，或在队列里（`checkSearch` 不做空队列也核对它，3.10）。
- **不等别人**：
  - 事务一只锁 `page_search` 的行，只有这一个任务写这张表。
  - 事务二 `SKIP LOCKED`。
  - 单元至多等事务二删它那个节点的一行。
- **导入与后台的恢复、撤销**：每批让它的至多 100 个节点以批量入队，任务随后跟上；期间这本笔记本的搜索带 `building`。导入 1 万、5 万页之后 66.6、324.3 秒清空（3.9）。
- **升级**：00040 让全部活着的节点以批量入队；任务建完之前（30 万页实测 15–17 分钟，3.9）每次搜索都带 `building`，只找得到之后写过的。v0.1 还没有发布过，只影响开发的库。大的补入与 reindex 写大量的 WAL、让 `page_search` 在磁盘上翻倍（3.9）：README 的部署一节写明。

### 3.5 查询：两层、复查、游标、计数与摘要

**候选**：索引的命中并上队列里准确的节点（新写的、恢复的、改过名的，它们的行可能还没重建），连活着的节点、限看得到的笔记本：

```sql
SELECT s.node_id FROM page_search s WHERE /* this tier's keys */
UNION
SELECT q.node_id FROM page_search_queue q WHERE NOT q.bulk AND q.notebook_id = ANY(sqlc.arg(notebooks)::uuid[])
```

- 队列里行已是现在的节点（只在任务的两个事务之间一瞬）当候选也只是多复查一次，不另比行。
- **实施**（偏离 1，3.9 第 1、7 步）：名称层的候选照这样读。正文层的候选是 `(SELECT node_id FROM page_search_hits(@body_keys, @titled_keys) LIMIT @hits)` 并上队列：函数只经 GIN 的位图读（3.2），`@hits` 是同一快照里 `SearchHitCount` 先数出的命中数，规划器以它为候选的估计，`LIMIT` 从不截断。一次请求在第一条正文层的语句之前数一次。
- **连接**（第 7 步在四种里取的）：候选连 `nodes`（与 `page_contents`）只带笔记本的条件，在 `OFFSET 0` 之后；已删、类型、游标与名称的模式在它之上对连好的行检查，从不在 `nodes` 的扫描里。照下面原样的连接条件在没有统计时把一本笔记本的活着的节点估作一行、逐个比较每个候选：守卫的规模上 `电 #` 97 ms、`the` 143 ms，对 4–6 ms。
- **排序**：按 `sort_at = greatest(n.updated_at, c.updated_at)`（最后一次改动：正文、名称或位置）与 id 倒序；附件没有正文，`greatest` 略过空值。

**两层**：

- **名称层**（第 0 层）：每个词都匹配名称的活着的节点，页与附件都在。
  - 名称在排序与 `LIMIT` 之前检查（`n.name ~ ALL(@terms)`）：名称至多 255 字节、在 `nodes` 里、不用解压，审查 A 估 1 万个约 5–10 ms。这一层不走查，只取 `limit + 1` 个；页的正文只为答出的行按键读，给摘要。
- **正文层**（第 1 层）：活着的页，满足三条：
  - 不是每个词都匹配名称（那些已在第 0 层）：`NOT n.name ~ ALL(@terms)`，在 `LIMIT` 之前。
  - 只搜标题的词匹配名称：`n.name ~ ALL(@titled)`，在 `LIMIT` 之前。
  - 别的词各匹配名称或正文：走查时对正文复查。
- **走查**：正文层至多走查 `MaxSearchWalk` = 500 个页的正文，直到 `limit + 1` 个通过。走查不排序正文；正文按键只为走查的行读一次，拼接与复查各算一次（审查 A 在 PG 18.6 上核对过这个写法：top-N heapsort、按 `page_contents_pkey` 读走查的行、顶上没有排序）。

实施的 `SearchBodies`（`480c031f`；正文按它的 NFC 读：`CASE WHEN raw IS NFC NORMALIZED THEN raw ELSE normalize(raw, NFC) END`，P5A 审查 A-M4，是 NFC 的只多一次检查，按字符算，一个 4 KB 的正文中文约 19 µs、英文约 34 µs）：

```sql
-- name: SearchBodies :many
-- The body tier of a search (M8/P5 design 3.5): the live pages of the notebooks given whose lexemes hold the keys,
-- hits of them (SearchHitCount's, all of them), or that wait in the queue as a user wrote them, below the cursor,
-- newest first, whose name misses some term and holds each titles-only one; walk of them at most have their content
-- read, by key, in NFC as SearchTitles reads it, and checked as they are now, until max_rows pass. The candidates are
-- joined to their nodes and contents with the notebooks' condition alone, behind OFFSET 0, as in SearchTitles; the
-- other checks, the patterns among them, run above it on the joined rows, never in a scan of nodes.
-- The sort carries no content; OFFSET 0 keeps each lateral that reads or checks it one subquery too, so the
-- concatenation and the check run once a row. The last row walked comes back passing or not, with walked: more than
-- walk is more to walk. A row that passes has its first hit's place and excerpt as SearchTitles'.
SELECT w.id, w.notebook_id, w.name, w.sort_at, w.seq::integer AS seq, w.walked::integer AS walked,
    coalesce(x.passed, false)::boolean AS passed, coalesce(e.at, 0)::integer AS at,
    coalesce(CASE WHEN e.at > 0 THEN substr(b.doc, greatest(e.at - 41, 1), 162) END, '')::text AS excerpt,
    coalesce(length(b.doc), 0)::integer AS chars
FROM (
    SELECT v.*, row_number() OVER (ORDER BY v.sort_at DESC, v.id DESC) AS seq, count(*) OVER () AS walked
    FROM (
        SELECT j.id, j.notebook_id, j.name, j.sort_at
        FROM (SELECT n.id, n.notebook_id, n.name, n.deleted_at, c.deleted_at AS content_deleted_at,
                  greatest(n.updated_at, c.updated_at)::timestamptz AS sort_at
              FROM ((SELECT h.node_id
                     FROM page_search_hits(sqlc.narg(body_keys)::text::tsquery, sqlc.narg(titled_keys)::text::tsquery) h (node_id)
                     LIMIT sqlc.arg(hits)::bigint)
                    UNION
                    SELECT q.node_id FROM page_search_queue q
                    WHERE NOT q.bulk AND q.notebook_id = ANY(sqlc.arg(notebooks)::uuid[])) k
              JOIN nodes n ON n.id = k.node_id
              JOIN page_contents c ON c.node_id = n.id
              WHERE n.notebook_id = ANY(sqlc.arg(notebooks)::uuid[])
              OFFSET 0) j
        WHERE j.deleted_at IS NULL AND j.content_deleted_at IS NULL
            AND (j.sort_at, j.id) < (sqlc.arg(below_at)::timestamptz, sqlc.arg(below_id)::uuid)
            AND NOT j.name ~ ALL(sqlc.arg(terms)::text[]) AND j.name ~ ALL(sqlc.arg(titled)::text[])
        ORDER BY j.sort_at DESC, j.id DESC
        LIMIT sqlc.arg(walk)::integer + 1
    ) v
) w
LEFT JOIN LATERAL (SELECT CASE WHEN r.raw IS NFC NORMALIZED THEN r.raw ELSE normalize(r.raw, NFC) END AS doc
                   FROM (SELECT c2.content || '' AS raw FROM page_contents c2
                         WHERE c2.node_id = w.id AND w.seq <= sqlc.arg(walk)::integer OFFSET 0) r OFFSET 0) b ON true
LEFT JOIN LATERAL (SELECT NOT EXISTS (SELECT 1 FROM unnest(sqlc.arg(bodied)::text[]) p
                                      WHERE NOT (w.name ~ p OR b.doc ~ p)) AS passed
                   WHERE b.doc IS NOT NULL OFFSET 0) x ON true
LEFT JOIN LATERAL (SELECT min(nullif(regexp_instr(b.doc, p, 1, 1, 0, 'c', 1), 0)) AS at
                   FROM unnest(sqlc.arg(terms)::text[]) p WHERE x.passed) e ON true
WHERE x.passed OR w.seq = least(w.walked, sqlc.arg(walk)::integer)
ORDER BY w.sort_at DESC, w.id DESC
LIMIT sqlc.arg(max_rows);
```

- **`SearchTitles`**：候选是 `s.title @@ @keys` 并上队列，`LEFT JOIN page_contents`，页与附件都在，同样只带笔记本的条件连接、在 `OFFSET 0` 之后；`WHERE` 里 `n.name ~ ALL(@terms)`，`ORDER BY … LIMIT @max_rows`；之后为这些行按键读正文（`LEFT JOIN LATERAL … OFFSET 0`）、取摘要的位置，没有正文的词时没有摘要。
- **外层的 `ORDER BY`** 沿用子查询的次序，不再排序，复查保持惰性（计划测试钉住，3.10）。
- **数组与参数**：`terms`（全部的词）、`titled`（只搜标题的）、`bodied`（其余的）是 3.3 的 ARE 模式（各带一个捕获组，`regexp_instr` 的第 7 个参数取它的位置）；`keys`、`body_keys`、`title_keys`（实施的名字 `titled_keys`）是 3.3 的 `tsquery`；`hits` 是数出的命中数。`~ ALL(数组)` 是一个运算，不是每行一个子查询。
- **sqlc 的类型**：时刻 `::timestamptz`、位置与字符数 `::integer`、摘要 `::text`、`tsquery` 以 `::text` 传：不这样写时生成的是 `interface{}`、`[]byte`、`float64`（审查 A 以仓库的 sqlc 核对）。
- **看得到的笔记本**：
  - `VisibleIn` 答几本就传几本，工作区没有笔记本数的上限。
  - 经 `Planned` 时数组是常量，9 个及以上元素时 PostgreSQL 用散列比较；缓存语句的通用计划则逐个比较（P2A 的教训）。
  - 3.9 量 5,000 个 id，与 `VisibleIn` 在 200、5,000 本笔记本时的时间。
- **为什么连 `nodes` 而不在 `page_search` 里存笔记本**：活着、可见、名称都要以节点现在的为准；节点不跨笔记本移动，但删除、恢复随时发生。
- **一次请求的语句**（两层、计数、`building`、路径）在一个 `REPEATABLE READ READ ONLY` 事务里：同一个快照，一次改名不会让一个节点在一页里出现两次或一次也不出现。跨页的仍可能（改名移出名称层的节点在正文层再出现），网页按 id 去重。

**游标**：`SearchCursor{Tier, At, ID}`（`shared.EncodeCursor`）。第一页从第 0 层的顶（最大的时刻与 id，同 `ChangesetFrom`）开始。

- 第 0 层取 `max_rows = limit + 1`。不足时第 1 层从它的顶接着走，补到 `limit + 1`。
- **`next_cursor`**：
  - 通过的有 `limit + 1` 个时，是第 `limit` 个的位置。
  - 否则 `walked` 大于 `MaxSearchWalk` 时（还有没走查的），是走查的最后一行。这时这一页可能少于 `limit` 个，甚至一个也没有。
  - 否则第 1 层走完，为空。走查恰好用完剩下的候选时也为空，不多一个空页。

**计数**（只在第一页；之后的页 `count` 为空）：

- **第一页没有 `next_cursor`**：数目就是这一页的个数，`exact`。
- **否则按层估**（实施时分成两条语句，偏离 2）：
  - `n0`：名称层的命中，名称检查过，数到 1,001；另数其中的页 `p0`（`SearchTitleCount`）。
  - `n0` 过了 1,000，或没有正文层（每个词都只搜标题）时到此为止。只有名称层没有定下时才数 `c1`：数正文层的候选让 `of`、`the` 的第一页 340–400 ms，名称层已定下时用不到它（3.9）。
  - `c1`：正文层的候选（`page_search_hits` 的命中并上队列，活着、看得到的页，名称不检查），数到 `MaxSearchCountScan` = 20,000（`SearchBodyCount`）；正文层的候选约是 `c1 − p0`。
  - `r1`：正文层的走查里通过的比例，走查查了 n 个、k 个通过（P5A 审查 C-M1、A-M5，修复二轮；`SearchTally.Full` 只在第一页的走查通过的到了它要的个数时设）：
    - 走查因这一页满了停在第 k 个通过（第 n 个通过）时取无偏的 (k − 1) / (n − 1)：k / n 在 k 为 1–2 时偏大、方差很大（名称层快填满第一页、正文层早早有一个通过时，初稿答"1,000+"对真的 60）。
    - 走查停在 `MaxSearchWalk`、这一页没满时取 k / n。
    - 走查走到候选的末尾时，正文层的数就是通过的个数（页满而恰好走完时同样；只查了一个候选时比例不取 0/0，修复核对第二轮 N2-3）。
    - 第一页没走到第 1 层时（名称层填满了这一页），另以 `SearchBodies` 从第 1 层的顶走查 `SearchCountSample` = 100 个取它。走查跑过但很短时不再取样：取样是又一条整个的正文层的语句，连接与排序随候选、不随取样的长短，30 万页上 `参考文献`（全部）的第一页约多 160 ms，答复照旧是 `at_least 1000`（P5A 修复核对第一轮 I-1）。
  - 估计 `N = n0 + max(c1 − p0, 0) × r1`。

- **种类与值**（`EstimateCount`，`SearchCount{value, kind}`）：
  - `n0` 过了 1,000，或 `N` ≥ 1,000：1,000，`at_least`（"1,000+"）。
  - 没有正文层（每个词都只搜标题）时 `n0` 就是准确的数，`exact`。
  - 一个都还没找到（名称层没有，走查也没有通过）时为空：网页说"还没有找到"。
  - 还有没查的候选、而比例依据的通过少于 `SearchCountPasses` = 2 个时（页满停下时第 k 个不算，依据 k − 1 个）：`at_least max(n0 + 通过的, 这一页的个数)`，向下取整（P5A 审查 A-M5：一个都没通过时初稿答"约 n0"，n0 只是下界；一个通过时比例的 95% 区间跨两个数量级，两个时约三分之二的时候在真的 2 倍之内。修复核对第二轮 F-1：页满停在第 2 个通过时比例只依据一个通过，修复三轮起也是 `at_least`）。
  - `c1` 数到了这次搜索设的上限：`N` 只是下界，`at_least`（"至少 N"，向下取整；`SearchTally.BodiesCapped`，P5A 审查 A-N6）。
  - 否则 `about`，取两位有效数字，至少是这一页的个数。两个通过的精度有尾巴：页满停在第 3 个通过、两个通过落在最前的几个候选里时比例很高，十分之一的密度、名称层 17–18 个时约百分之一答"1,000+"对真的约 200（修复核对第三轮 O-1，移交 M12）。
- **为什么按层**：名称很短，名称层的通过率接近 1；正文层的候选对 3 个字以上、对字母的三元组只是上界（spike 的 Plan 3 里 `latency` 的三元组候选 14,028 个、真的 225 个）。混在一起的比例会把"名称里 25 个、正文里约 300 个"的 `kernel` 说成"1000+"（审查 A-I3）。
- **`MaxSearchCountScan`**：20,000（3.9：减半到 10,000、5,000 时 `of`、`the` 的第一页仍过 300 ms，让它们过线的是名称层定下时不数正文层）。

**`building`**：每一页都带：`EXISTS (SELECT 1 FROM page_search_queue WHERE bulk AND notebook_id = ANY(@notebooks))`。为真时这些笔记本里还有没建好的节点，结果可能不全。

**摘要与高亮**：

- **摘要**：在 SQL 里只为答出的行算（至多 `limit + 1` 行），正文有命中时才给（偏离 9）。按词定位正文里第一处命中（`regexp_instr` 取捕获组的位置），取它之前 40、共 160 个字符，前后各多取一个字符作边界的上下文，与正文的字符数一起带回，不传整页的正文（spike 2.6：在 SQL 里切比回传整页再切便宜）。
- **在 Go 里**（`search_hits.go`）：
  - 在带上下文的摘要上按 3.3 的 RE2 模式找出每个词的全部命中（逐个找，从上一个命中的词的末尾接着找，不吞掉边界的字符）：上下文的字符让词边界看得到摘要外的那个字符。
  - 再去掉上下文的字符、把连续的空白合并成一个空格，区间随之换算，落在去掉的字符上的部分截掉；前后是否截断看位置与字符数。
  - 命中落在切的最开头或最末、这个词要那里有边界、而正文在切之外还有时，丢掉这个命中：不知道边界在不在（P5A 审查 A-N2）。
  - 在名称里找全部词的命中。
  - 重叠的区间合并，排好序。区间以 UTF-16 的码元计（`[start, end)`），网页直接切 JS 的字符串，不自己找（00 号文档 4.12：网页的标题键与数据库的折叠不同）。
- **路径**：结果的祖先经已有的 `TrashLines`（按键读，已删的也读到，P2 的语句）一条读出，根在前。
- **附件**：类型与大小经 `AttachmentInfos.Of`（4.1 的端口）。

### 3.6 用例与码的次序

```go
type SearchRequest struct {
	Q          *string
	NotebookID *uuid.UUID
	Cursor     string
	Limit      *int
}

type SearchAnswer struct {
	Hits     []SearchHit // node, ancestors, title ranges, excerpt, changed_at, attachment info
	Next     *domain.SearchCursor
	Count    *domain.SearchCount // the first page's: {Value 0–1,000, Kind exact|about|at_least}; nil later or when none has passed yet
	Building bool
}
```

`Search.Execute(ctx, actor, slug, req)`。在它之前，生成的绑定对 `notebook_id` 不是 UUID、`limit` 不是整数答 400 `bad_request`。之后：

1. **游标**读不出：400 `bad_request`（6.1）。
2. **`ParseSearchQuery`**：422 `validation_failed`（字段 `q`）。在找工作区之前：`q` 是自由文字，它的检查不说出工作区是否存在，同正文的大小与 NUL（13.1 第 4 条的例外）。
3. **`Workspaces.FindBySlug`**：没有这个工作区时 404 `workspace.not_found`。
4. **判定** `page.search`（`Target{WorkspaceID}`）：`ErrNotVisible` 也答 404 `workspace.not_found`。每种角色都允许，所以没有 403。
5. **`VisibleNotebooks.VisibleIn(ws, user, grant.WorkspaceRole)`**：
   - 带 `notebook_id` 而它不在其中：404 `notebook.not_found`。不答空列表，同别的接口。
   - 否则只搜它。
6. **`limit`**（`domain.SearchLimit`）1–50，默认 20：越界答 422（字段 `limit`，`out_of_range`）。分页的参数在资源之后，同 `listTrash`、`listChangesets`。
7. **看得到的笔记本为空时**答空的一页，`count` 为 `exact` 0，`building` 为假。
8. **一个只读事务里**（`Deps.Snapshots`，偏离 6）：两层、第一页的计数、`building`、路径；之后高亮与附件的信息。

码：`[bad_request, validation_failed, workspace.not_found, notebook.not_found]`。

- 用例不记日志：读不记，同别的读。
- 搜索语句的任何错误在适配器里换成只带 SQLSTATE 的错误（没有 SQLSTATE 的，如 pgx 在客户端编码参数的错误，只带它的种类）再往上交：PostgreSQL 的消息会带上模式或 `tsquery`，即用户的文字，`APIErrors.Write` 把错误整个写进日志。

### 3.7 契约

**`searchWorkspace`**：`GET /api/v0/workspaces/{slug}/search`，`tags: [page]`。

- **参数**：
  - `slug`：page 的新参数 `WorkspaceSlug`，同 notebook 的 `Slug`。
  - `q`：字符串，不标 `required`（同 linking 的 `target`）。说明写"缺了或为空是 `validation_failed`（`required`）"与 3.3 的取值、语义的表。
  - `notebook_id`：uuid，可选。不是 UUID 时绑定答 400。
  - `cursor`：`Cursor`。
  - `limit`：新的 `SearchLimit`，1–50、默认 20，越界 `validation_failed`（`out_of_range`），不是整数时绑定答 400。不用通用的 1–100：复查的工作随页的大小增长，spike 量的是 20。
- **答复** 200 `SearchPage {data: SearchHit[], next_cursor: NextCursor, count: SearchCount|null, building: boolean}`：
  - `SearchCount {value: integer 0–1000, kind: exact|about|at_least}`：`exact` 在第一页拿到了每一个命中、或 `q` 只在名称里找时；`about` 是约数，取两位有效数字；`at_least` 是下界：1000 总是，查过的正文里通过的太少、不足以估计时是已见的命中，候选多得数不完时是向下取整的估计（P5A 审查 C-N2、A-M5，修复核对第一轮 N-2）；只在第一页；还没有一个结果而查找没有走完时为空。
  - `building`："这些笔记本里还有节点在等后台建索引（导入、后台的恢复与撤销、升级、reindex 之后）：结果可能不全"（后台的恢复与撤销由修复二轮写进契约，修复核对第一轮 M-2）。
- **结构**：
  - `SearchHit {node: SearchNode, ancestors: Ancestor[]（根在前）, title_hits: TextRange[], excerpt: SearchExcerpt|null, changed_at, attachment: AttachmentInfo|null}`。
  - `changed_at`：最后一次改动名称、位置或正文的时刻。不叫 `updated_at`：`TreeNode.updated_at` 不含正文的写。
  - `SearchNode {id, notebook_id, kind: NodeKind, name}`。
  - `TextRange {start, end}`：UTF-16 码元，`end` 不含，整数 ≥ 0；一个列表里的区间排好序、不相交（服务端合并不同的词的重叠）。
  - `SearchExcerpt {text, hits: TextRange[], cut_start: boolean, cut_end: boolean}`：正文的写法，Markdown 不去掉，空白已合并；不是 NFC 的正文组成 NFC（编辑器写 NFC，修复核对第一轮 N-1）。
  - 可空的对象照 P2 3.9 写成 `anyOf`。
- **`TrashAttachment` 改名 `AttachmentInfo`**：说明改为附件的类型与大小，回收站的单项与搜索都引用它。一种形状一个名字，P1A 审查 C6。它只在服务端的 `trash.go` 里有引用（网页只有生成的类型），与别的模块的结构不重名。
- **说明写明**：
  - 两层与次序；`q` 的语义（3.3 的表），折叠相同的词是一个、取第一个的写法（`ß ss` 是 `ß`，P5A 审查 A-N7）；单独的情形：被隔开的单字的词，在没有一个词带正文的键（字母、数字或组合记号，或两个相连的汉字）的查询里只在名称里找（修复核对第一轮 N-3）。
  - 用户在请求里的写按已提交的状态立即找得到；导入、后台的恢复与撤销、升级、reindex 写的节点在任务建好之后，其间 `building`，可能不全。
  - 走查的界线：一页至多查 500 页的正文，可能少于 `limit` 个、甚至没有而带着 `next_cursor`（P5A 审查 A-N3、C-N1）。
  - 422 的 `too_long` 含"至多 64 个连续的 ASCII 字母与数字"；`not_allowed` 是没有键的 `q`：没有字母、数字、组合记号或汉字（与单独的情形的用词一致，修复核对第二轮 N2-4），服务端的消息同样。
  - 码的次序（3.6）。
  - `q` 在请求行里，会进反向代理的访问日志。
- `x-problem-codes: [bad_request, validation_failed, workspace.not_found, notebook.not_found]`。
- `api/openapi.yaml` 加路径的 `$ref`。
- 契约测试自动覆盖它：`TestFreeTextParametersDoNotAnswer5xx` 的 NUL 与 `\xff` 在第 2 步答 422；`ParamCases` 给 `notebook_id` 推出不是 UUID 的 400。

### 3.8 权限、日志、加锁与交错

- **动作**：`page.search`，工作区级，`every()`：管理员、成员、访客。访客只看得到被显式加入的笔记本，`VisibleIn` 已是这样。
  - `page/domain/actions.go` 加 `ActionSearch`，规则表加一行。
  - `bootstrap/actions_test.go` 的"规则 = 动作"照旧通过。
- **测试的种子**：矩阵与可见性的种子在迁移之后以 SQL 写，不经单元，00040 的补入也已跑过，所以种下的节点既不在队列里也没有行。
  - 新的帮助函数 `queueSeeded`：种子的最后让全部节点（含已删的）以准确入队：`INSERT … SELECT id, notebook_id, false FROM nodes ORDER BY id ON CONFLICT DO NOTHING`。已删的也要入队，否则"回收站里的页不在"不论读有没有过滤都成立。
  - 标记词加在每页的正文末尾（`matrixTasks` 的位置不变）；每本矩阵笔记本另建一个名称含标记词的附件，不改种下的附件的名称（`page_visibility_test.go:66` 按名称找 `team-page.png`）。
- **权限矩阵**（`permission_matrix_page_test.go`，同 `listNotebooks` 的两行）：
  - 工作区的列（`acme`）：管理员、成员与访客 200、空列表；不是成员、成员身份结束、工作区已删 404 `workspace.not_found`。
  - 笔记本的列（`lab`）：带 `check`，答复的节点等于这一列读得到的、带标记的页与附件（经队列）。
- **"列表的结果"等于"逐项判定读取"**（`page_visibility_test.go` 的 `TestTheSearchIsWhatEachReadAllows`，13.1 第 3 条点名 M8 的搜索）：
  - 对每一列，以标记词搜 `lab` 与这一列所在的别的工作区，答复的节点 id 等于逐个 `GET /pages/{id}`、`GET /assets/{id}` 读得到、带标记的节点。
  - 回收站里的页、已删笔记本的页不在。
  - 每本看得到的笔记本以 `notebook_id` 限定时是它的子集，看不到的答 404。
  - 跑两遍，先断言每遍依靠的状态：
    - 任务之前：队列里有全部种下的节点、`page_search` 为空，全部靠队列，已删的节点在候选里，读的过滤才被核对。
    - `Drain` 之后：队列为空，全部靠索引。已删的节点这时没有行；索引的命中里已删的节点由交错 2 与适配器的测试核对。
- **日志**：用例不记；搜索语句的错误只留 SQLSTATE（3.6）。`TestSearchStatementErrorsCarryNoQuery` 以一个会失败的语句核对。
- **加锁**（13.1 第 5 条加一段）：
  - 读不加锁（只读事务）。
  - 队列的行总由一条多行的语句按 `node_id` 升序取（`QueueSearch` 的 `ORDER BY 1`、笔记本删除与 reindex 的 `ORDER BY id`），13.1 第 5 条"同一张表的多行按 id 升序"。
  - 单元的入队在它全部的写之后、观察者之前：`… → nodes → page_contents → page_search_queue`，之后才是 linking 的 advisory lock。笔记本删除的入队在笔记本行之后。reindex 的入队每批一个事务，不持别的锁。
  - 任务的事务一只锁 `page_search` 的行（只有它写这张表），事务二 `SKIP LOCKED`，从不等。
  - 所以不成环：等队列行的只有单元、笔记本删除与 reindex，它们各以一条语句按同一个次序取，一方等的行总在它已持的行之后；任务只持一瞬、从不等。初稿的"不成环"漏了 reindex 与单元按不同的次序取（审查 A-M6、B-I3：PostgreSQL 在 `deadlock_timeout` 之后中止其中一个，可能是用户的删除）。
- **交错**（`interleavings_search_test.go`）：bootstrap 的测试不注册任务（`SearchInterval` 为 0），每个结束时以 `Drain` 做空队列，再核对 `checkSearch`（3.10）、`checkPages`、`checkLinks`。
  1. 任务读了一页（版本 R）之后，一次保存提交 R+1：任务写 R，出队落空；下一次写 R+1。
  2. 任务正在写一页的行时这一页被删除：搜索立即不列它（它的行还在、是索引的命中）；下一次删掉行。
  3. 删除 → 任务删掉行 → 从回收站恢复：任务再跑之前搜索就找得到（队列）；之后重建。撤销一次删除同样。实施先让任务建好这一页的行，删除之后的 `Drain` 删掉它（`Deleted == 1`、`page_search` 为空），之后页才回来（P5A 审查 B-N1：第一版里页在第一次运行之前就建了又删了，没有行可删）。
  4. 单元持着队列的一行时任务出队：跳过、不等，下一次运行再读。持行用新的 `holdQueueRow`（`holdRow` 持的是 `asset_blobs` 的行）。
  5. 删除笔记本，任务同时在建它的页：搜索从不列它们，下一次删掉行。
  6. reindex 的入队停在一批的中间（持着前一半的行）时，一次删除子树的单元入队同样的节点：都按 id 的次序，单元等到 reindex 提交，不死锁，两边都完成。
  7. 导入的两批之间（`betweenUnits`）搜索：第一批的页不在结果里、答复带 `building`；`Drain` 之后找得到、`building` 为假。
  8. 任务停着时节点被清理（`purgeBefore`）：下一次删掉它的行。
  9. 两个 `Drain`：测试的连接持着任务的 advisory lock 时 `Drain` 立即返回、什么都不写；放开之后照常。

### 3.9 大小与实测

实施之后照实测改写。第 1 步在 spike 的语料上以工具量（照 3.5 原样的语句与几种改法），第 7 步以产品的基准在同一份语料上量。数是 P5A 的：共用的 18 核、128 GB 的 macOS 笔记本电脑，Docker Desktop 里的 PostgreSQL 18.6（`builtin` 的 `C.UTF-8`），默认设置（`shared_buffers` 128 MB、`work_mem` 4 MB、`max_wal_size` 1 GB、`gin_pending_list_limit` 4 MB、每个 Gather 2 个并行的 worker，会话里 `jit` 关）。机器同时跑着别的工作，负载写在各处；一个 10 月 9 日网页变异的运行留下的 vitest 两个 worker 一直占着约两个核，到 10 月 15 日约 17:00 才停掉：第 1 步的全部与第 7 步的成本基准在它之下，第 7 步四种形状的搜索基准在它之后，没有为它重量。

**语料**：照 spike 第 4 节重建：zhwiki 20261001 的同样几个分卷与 enwiki 20261001 第一卷的前 250 MB，共 3,101 MB（原写的"enwiki 20261008"没有这个目录；spike 经 `latest/` 读的正是 20261001 的第一卷）。`meta.tsv` 与 spike 的相同，逐格可比。30 万页（活着的 294,055）、200 本笔记本，正文 2,386,847,945 字节（平均 8.1 KB）。第 7 步从同样的原始文件重建，每个文件的 sha256 与第 1 步的清单相同；之后语料文件删掉，清单与全部的结果留在 scratchpad。

**工具与方法**（第 1 步）：`bench`（Go，pgx 的 `DescribeExec`，每条语句按参数规划，`SET jit = off`）用 `domain/search.go` 的草稿，两种写法；3.5 的语句原样与几种改法，计数的估计；任务的一次运行照 3.4。查询：spike 的 28 个，加 1–2 个字母的整词（`ai`、`go`、`c`）、词中间的子串（`api`）、走查最先用完的罕见词（`veress`、`ranters`、`latere`）、名称多而通过率低的词、单字的混合（`A股`、`锅 中国`）与正文层没有键的（`锅 #`）、8 个词的、最常见的键（`the`、`ing`、`a`、`of`、`中国`）。形状：第一页（含计数）与第三页；可见 200/200、10/200、一本（约 1,500 页）、5,000 个 id 的数组；建索引时、导入 5 万页的清空期间与之后、准确的积压、待处理列表满。每格热身一次之后 20 次，取 p99。模式在两边：语料的文字上每种写法 3,776 个生成的词，`~` 与 `regexp_instr` 的位置与 RE2 的第一个捕获组处处相等。

**目标与结果**（第 3 节第 6 条的"目标"，300k 页、热、默认设置）：

| 项 | 目标 | 结果 |
|---|---|---|
| 每类查询第一页（两层、计数、摘要、路径）与"更多"的 p99，可见 200/200、10/200 本与限定一本 | ≤ 300 ms | 产品（第 7 步，负载 3–7）：无统计、ANALYZE 之后、为任意的参数规划三个阶段各 204 格里 3、9、14 格过 300 ms，至多 348、586、719 ms，没有过 1 秒的；都是重的类，大多在"全部"的可见上（下）。第 1 步（负载 6–13，过滤的连接）473 格里 20 格过，至多 771 ms。**没有全部达到**：主持者接受、移交 M12，负责人可以改判 |
| 建索引时（30 万个节点以批量排队、任务在跑）的同上 | ≤ 300 ms，答复带 `building` | 第 1 步：150 格里 98 格之内，至多 908 ms（`中国` 第一页、10 本）；照写的语句 40 秒 |
| 待处理列表满（4 MB / 16 MB）的同上 | ≤ 300 ms | 92 格里各 82 格之内，至多 486、687 ms；照写的语句 17.6、8.8 秒 |
| 准确的积压 1,000 页、1 万个附件时的同上 | ≤ 300 ms；记下第一页为空的查询的比例 | 156 格里 126 格之内，至多 1,191 ms（`C语言` 第一页、全部，负载到 26）；92 个第一页里 17 个为空而带游标（平时 7 个）。后台的恢复与撤销以批量入队之后，这样的积压只来自同步的单元（3.4） |
| 导入 5 万页：清空期间 / 之后 | — | 150 格里 98 格之内，至多 1,149 ms / 80 格里 67 格，至多 547 ms |
| 体积 | `page_search` 的表、TOAST 与两个索引合计 ≤ 正文的 3 倍 | 第 1 步 5,474 MB，2.45 倍（TOAST 3,721 MB、正文的 GIN 1,599、标题的 25、堆 76）；表填满之后 `CREATE INDEX` 2.49 倍（正文的 127 秒）。第 7 步 5,762–5,963 MB，正文 2,339 MB 的 2.46–2.55 倍 |
| 任务的吞吐 | ≥ 100 页/秒；200 KB 的页 ≥ 1 MB/秒；30 万页从空建完 ≤ 1 小时，导入 1 万、5 万页之后约 2、10 分钟清空 | 产品的任务（写的设置，3.4）从空建 285–329 页/秒、2.3–2.6 MB/秒，294,055 行 14 分 53 秒到 17 分 12 秒；第 1 步的工具在默认设置下 100.0 页/秒、0.80 MB/秒。导入 1 万、5 万页之后 66.6、324.3 秒清空，读的负载下（负载 10–17）5 万页 510 秒。200 KB 的页没有单独量吞吐（每行写 10 ms，下） |
| 保存时多出的开销 | 一页的保存 p99 多 ≤ 2 ms；1 万个节点的单元多 ≤ 0.2 秒 | 一页的入队 p50 0.49、p99 0.69 ms；1 万个 id p50 20.1、p99 29.1 ms。产品上保存 10 KiB、200 KiB 的 p50 多 0.3、0.5 ms，p99 在噪声之内（下） |
| 计数 | 每个查询的估计与真实的数相差在 2 倍之内，或都 ≥ 1,000 | 第一页 92 格里 84 格合规则（准确、2 倍之内的约数，或不超过真的数的"至少"）：`latency` 全部约 170 对真的 225，`Go语言` 约 16 对 12，`Windows系统` 约 150 对 201。8 格是走查用完的类：`A股` 全部"至少 2"对真的 406（真的下界，几乎什么也没说），`A股` 10 本与罕见的 `veress`、`ranters`、`latere`（两种可见）为空，真的 0–13：照 3.5 的规则。修复改了估计（3.5），30 万页上没有重量 |

只记录的（第 1 步）：冷读（重启数据库，Docker 的页缓存仍热）第一页 `中国` 191.5、`数据库` 208.9、`latency` 316.9、`ai` 97.3 ms；不开并行的 92 格里 9 格过 300 ms（至多 513）；语料里到词的字节界线的页 0；名称层对最常见的键 `of` 66.6/70.4 ms（全部；5,000 个 id 74.6/77.7），`the` 24.7/25.7，`的` 31.1/32.7。没有量的：每写一行之后索引的增长（pgstattuple 的大小按整页变，读数都是 0）；`VisibleIn` 在 5,000 本笔记本时。

**第 1 步的决定**（照下面的取舍的规则）：

| 项 | 决定 | 依据 |
|---|---|---|
| 字母文字 | **T**（三元组，3 个字母起；1–2 个按整词） | T 在平时每个字母的类都达到 p99（规划器自己的计划、名称层定下时不数正文层）；P 在常见的词上差得多（`the` 每种可见 2.5–3.7 秒，`of` 545–589 ms，`a` 435–468 ms）。计数两者一样（估计不看写法）；体积 2.45 倍对 2.35 倍，建索引 100 对 99.9 页/秒 |
| `MaxSearchWalk` | **500** | {500, 1,000, 2,000, 5,000} 里走查用完的类 p99 ≤ 300 ms 的最大一档：500 时最坏的 `A股` 294 ms，1,000 时 364，2,000 时 544，5,000 时 741 |
| `MaxSearchCountScan` | **20,000**（不变） | 减半没有让 `of`、`the` 的第一页过线（1 万：355、386 ms；5,000：348、377）；过线的是名称层定下时不数正文层（3.5，偏离 2） |
| 名称层的界线 | **不设** | 最常见的键的名称层单独：`of` 66.6/70.4 ms，`the` 24.7/25.7，`的` 31.1/32.7 |
| `fastupdate` | **开**（默认） | 关掉时 10 KB 的页的新行 37 ms（开 1.8），200 KB 788 ms（开 10），1 MB 1.6 秒；满的索引上任务 23 页/秒。待处理列表满时的读开关都有几格不达标，关掉也过不了吞吐 |
| 任务的写的设置 | **`work_mem` 64 MB、`gin_pending_list_limit` 16 MB，只在它的事务里**（偏离 3：3.2 原是默认） | 满的索引上重建 2 万行：默认 38 行/秒，只设 `work_mem` 66，只设待处理列表 38，两者 136.5（待处理 64 MB 加 `work_mem` 187，不取：读要扫待处理列表） |
| `MinSearchRebuild` | **30 秒**（不变） | 5 MiB 的页在任务的设置下分词 76 ms、写 62 ms（p50），吞吐没有问题 |
| 正文层的语句 | **`page_search_hits` 加数出的 `LIMIT`**（偏离 1）；与 `nodes` 的连接第 7 步定为**只带笔记本的条件** | 下 |

**规划器与 `page_search`**（改了语句的发现）：照 3.5 原样的语句平时达标；全部节点以批量重排、任务在重建时（"建索引时"），与 GIN 的待处理列表满时，读要 5–40 秒（`A股` p50 17.6 秒，`参考文献` 14 秒，`中国`（10 本）20 秒；建索引时 110 格里 27 格过 300 ms，至多 40.3 秒；待处理列表 4 MB 时至多 17.6 秒，16 MB 时 8.8 秒）。EXPLAIN：规划器选了 `Seq Scan on page_search`、以 `body @@` 作过滤，逐行解压每一行的正文的词（3.7 GB 的 TOAST，约 37 µs 一行）；整表扫描的代价不算 TOAST，GIN 的代价含待处理的页与死的项；平时两者已在一成之内（位图约 22k、整表约 24k 个代价单位），待处理列表或重建的变动就让它翻过去。

- 读的事务里 `enable_seqscan = off`：不再有灾难（建索引时至多 710 ms），别的表的路变差（平时 +10–25%，导入 5 万页时到约 1 秒），计划测试又找到下一个退路：走一遍 `page_search_pkey`、同样的过滤与解压。放下。
- 迁移里定义的函数，它的 `SET` 只留下 GIN 的位图：不再有灾难；但 SETOF 函数的估计是它固定的 `ROWS`，取 1 万时每个查询都以整表扫描散列连接 `nodes` 与 `page_contents`（罕见的 8 → 100 ms，`锅 中国` 12 → 620 ms）。
- 给外层的计划命中数的三种做法，同一负载下并排量（19 个查询 × 8 格，p50 之和）：规划器自己的语句 10,606 ms（但脆弱，上）；`enable_seqscan = off` 11,817（+11%），14 格过 300 ms；**数出的 `LIMIT`** 12,134（+14%），6 格；命中作数组参数回传 13,033（+23%），13 格。取数出的 `LIMIT`（3.5）。估计准了之后，规划器把几千个命中散列连接每个活着的节点、对 29.4 万个名称跑整词的正则（`c`：那一次扫描 204 ms）：名称的检查放在连接之后（`OFFSET 0`）。
- 代价：平时中等的类（几千个命中）是规划器自己的计划的 2–3 倍，仍远在目标之内（80–150 ms）；重的类在 ±15% 之内。建索引时每多读一次 GIN 就再扫一遍待处理列表（第一页至多三次），比 `enable_seqscan = off` 慢，但有界。

第 1 步量过的每种形状（每格 20 次读的 p99，ms；"产品"是 `page_search_hits` 加数出的 `LIMIT`、走查的名称在围栏之后，第 7 步定下连接之前；格写作类、页、可见）：

| 形状 | 格 | p99 > 300 ms | p99 的中位数 | 最大的 p99（格） | 负载（1 分钟） |
|---|---|---|---|---|---|
| 平时，照写的语句（T） | 473 | 6 | 13 | 400（`key_of` 第一页、5,000 个 id） | 3.7–6.1 |
| 平时，照写的语句（P） | 462 | 17 | 12 | 3,729（`en_hi` 第一页、10 本） | 3.8–6.4 |
| 平时，照写、名称层定下时不数正文层（T，`the`、`of`、`a`、`ing` 的格） | 52 | 0 | 125 | 287（`mx_ashare` 第一页、5,000 个 id） | 4.8–6.2 |
| 平时，产品（走查 500） | 473 | 20 | 16 | 771（`rare_ranters` 第三页、5,000 个 id） | 5.9–13.1 |
| 平时，产品，不开并行（只记录） | 92 | 9 | 96 | 513（`mx_ashare` 第一页、全部） | 6.4–9.4 |
| 建索引时，照写 | 110 | 27 | 74 | 40,333（`mx_ashare` 第三页、10 本） | 4.3–10.7 |
| 建索引时，`enable_seqscan` 关 | 150 | 33 | 95 | 710（`rare_latere` 第一页、全部） | 4.5–7.3 |
| 建索引时，产品 | 150 | 52 | 153 | 908（`c2_hi` 第一页、10 本） | 8.3–12.9 |
| 待处理列表 16 MB 满，照写 | 92 | 18 | 53 | 8,780（`c4_hi` 第一页、10 本） | 5.0–8.8 |
| 待处理列表 16 MB 满，产品 | 92 | 10 | 107 | 687（`key_a` 第一页、全部） | 6.7–9.6 |
| 待处理列表 4 MB 满，照写 | 92 | 9 | 55 | 17,641（`mx_ashare` 第一页、全部） | 4.7–8.0 |
| 待处理列表 4 MB 满，产品 | 92 | 10 | 81 | 486（`rare_latere` 第一页、全部） | 6.7–8.8 |
| 准确的积压，`enable_seqscan` 关 | 152 | 10 | 78 | 489（`key_a` 第一页、全部） | 5.6–6.9 |
| 准确的积压，产品 | 156 | 30 | 129 | 1,191（`mx_c` 第一页、全部） | 6.8–26.0 |
| 导入 5 万页清空期间，`enable_seqscan` 关 | 150 | 48 | 89 | 990（`c2_hi` 第一页、10 本） | 5.8–9.0 |
| 导入 5 万页清空期间，产品 | 150 | 52 | 124 | 1,149（`mx_ashare` 第一页、导入的笔记本） | 8.9–17.1 |
| 导入 5 万页之后，产品 | 80 | 13 | 79 | 547（`key_a` 第一页、导入的笔记本） | 6.6–8.5 |

"平时，产品"在负载 5.9–13.1 下量，照写的在 3.7–6.1；同一负载的并排量（上）里产品约比规划器自己的计划慢 14%，不是 2 倍。

**第 7 步：产品的基准**（`BenchmarkSearch`，`NERVE_SEARCH_CORPUS` 指向重建的语料，经 `heavy.sh`，testcontainers 的 PostgreSQL 18.6、默认设置；10 月 15 日 17:16–19:49，负载 3–7）：COPY 写进语料，全部以批量入队、以生产的 `Drain` 建完，量体积；再读 45 个查询 × 2 页（第一、第三）× 3 种可见（200 本的管理员、10 本的访客、一本）× 3 个阶段（无统计、ANALYZE 之后、为任意的参数规划），每格热身一次之后 20 次，经处理器、生产的中间件与 15 秒的请求期限。四种形状给候选（名称层的命中，或 `page_search_hits` 加数出的 `LIMIT`，各并上准确的队列）不同的连接，答复（命中、游标、计数）每格都相同：

| 形状 | 过 300 ms 的格：无统计 / ANALYZE 之后 / 通用 / 合计 | 过 1 秒 | ANALYZE 之后 p99 的中位数 | 三个阶段 p99 之和 |
|---|---|---|---|---|
| 按键的 lateral（`8ac758a2`） | 20 / 17 / 25 / 62 | 1（`中国` 第一页、10 本、通用，1,175 ms） | 23.5 ms | 63.5 秒 |
| 过滤的连接（`7277c41d`，3.5 照写的） | 14 / 2 / 31 / 47 | 5（`API接口` 第一页、全部、无统计 2,316 ms；`参考文献`、`history`、`的` 第一页、全部、通用，至多 1,982） | 37.0 ms | 67.8 秒 |
| **只带笔记本条件的连接（取）** | **3 / 9 / 14 / 26** | **0** | 47.3 ms | **53.3 秒** |
| 不带条件的连接 | 23 / 17 / 22 / 62 | 0 | 74.6 ms | 79.3 秒 |

- 取只带笔记本条件的连接：三个阶段过 300 ms 的格最少，任何阶段都没有过 1 秒的，之和最低。过滤的连接 ANALYZE 之后更好（2 格对 9），但脆弱：无统计时到 2.3 秒；守卫的规模上（两本笔记本）慢 15–25 倍（`电 #` 97 对 4 ms）：语料的 200 本笔记本正好合两个默认的选择度，掩盖了它，一本大的笔记本的团队会碰上。lateral 的中位数最快，但对常见的词在每种范围里都探测每一个命中，重的类里四种之中最慢（`a` 全部第一页 737 ms，`the` 10 本第一页 438 ms）。
- 取的形状过 300 ms 的格：无统计 `c` 304、`A股` 348、`a` 317 ms（都是全部的第一页）；ANALYZE 之后全部的第一页 `参考文献` 327、`中国 历史` 303、`api` 309、`A股` 586、`a` 476、`ranters` 321、`history` 347，`A股` 全部第三页 398，`the` 10 本第一页 338；通用 14 格，至多 `A股` 全部第一页 719 ms。这些是几万个命中的类：GIN 的长的倒排、每个命中的连接、走查的正文。
- 对照第 1 步（过滤的连接、ANALYZE 之后、负载 6–13）p99 过 100 ms 的 66 格：取的形状与它的比的中位数 0.88，41 格在 ±30% 之内，19 格快过三成，6 格慢过三成；全部 203 格的中位数 1.09。
- 另有两次运行停在基准的缺陷上（第二个阶段又插入一次访客的成员关系，`7f8f8514` 修好），一次被客户端的退出杀掉；它们的数不用。

**修复之后的代价**（30 万页上没有重跑）：按 NFC 读（3.5，P5A 审查 A-M4）按字符算：8 KB 的正文中文 37.5 µs、中英各半 54.5 µs、英文 67.1 µs（每 4 KB 约 19、28、34 µs，是读它的几倍）；语料的页平均约 8 KB，走查 500 页约多 20–35 ms。守卫的语料上走查正文的读至多慢到 2 倍（`中国 database` 第一页 24–30 → 37–43 ms），其余的约不变。主持者为了对留着它（负责人可以改判）；量过的更便宜的两种移交 M12：正则的预检（`raw !~ '[…]' OR raw IS NFC NORMALIZED`，类取自 x/text 的 2,177 个字符、295 个区间，7.4 对 19.8 µs，约便宜六成；印地文、泰文与分解的正文仍走完整的检查），任务在它读的版本上存下 NFC 的标记（约一次按键的读，要一列与一次连接，排队的仍要检查）。

**写**（任务在满的索引上每行的代价，第 1 步）：

| 页 | 分词 p50 | 写 p50（默认） | 写 p50（任务的设置） | 每行的 WAL（任务的设置） |
|---|---|---|---|---|
| 10 KB 新建 / 重建 | 0.2 / 0.3 ms | 1.8 / 1.8 ms | 1.7 / 1.7 ms | 52 KB |
| 200 KB | 7–9 ms | 10 ms | 10 ms | 0.7–33 MB |
| 1 MB | 22–26 ms | 33–34 ms | 28–30 ms | 2.7 MB |
| 5 MiB | 76–86 ms | 新建 60 ms，重建 3.8 秒 | 50–62 ms | 重建 4.6 MB |

- 有些行的 p99 1–4.8 秒：越过待处理列表的界线、把它合并进主索引的那一次写，在任务自己的事务里；只有任务写这张表，读从不等它。
- **WAL**：满的索引上重建 2 万行，默认设置 53 GB、任务的设置 33 GB（`max_wal_size` 1 GB）。P5A 审查 B-M2 的读法：146.5 秒 33,207 MB，每行 1.66 MB、227 MB/秒，64 次检查点（每 2.3 秒一次，各约 519 MB），650 万个整页的镜像：每一轮重新镜像正文的 GIN 约 20.5 万页里的约 10 万页，镜像又填满下一个检查点的距离；行自己的记录是小头（10 KB 的页 52 KB）。外推一次全部的 reindex（294,055 行）约 488 GB、约 36 分钟；升级的补入从空建，没有量，机制相同。没有 WAL 归档或复制槽时 `pg_wal` 停在 `max_wal_size` 附近；有归档或落后的槽时全都留着。放慢没用（量是每个检查点的镜像）；杠杆是 `max_wal_size`。
- **磁盘**：全部重建之后表是建好时的约 2 倍（B 的探针，8 KB 的页：1.98、2.10 倍；`VACUUM` 之后再重建 2.00、2.12 倍），空出的地方复用、不还给系统：30 万页多约 5.5–6 GB。README 的部署一节写明（大的工作区补入或 reindex 之前调大 `max_wal_size`，留出至少 `page_search` 那么大的磁盘，有 WAL 归档时按每行约 1.7 MB 准备）；数移交 M12。

**入队多出的开销**（第 2 步：基点 `2800db3b` 与分支交替跑、同一次持锁；分支在 `7277c41d` 的工作树上，与最后的分支只差读的语句，这些基准不跑它们；负载 5–12，vitest 在）：

| 基准 | 基点 → 分支（中位数） |
|---|---|
| `BenchmarkSave` 10 KiB 的 p50（一次运行 300 次保存） | 5.2 → 5.4 ms（+0.3） |
| `BenchmarkSave` 200 KiB 的 p50 | 13.8 → 14.4 ms（+0.5） |
| 两者的 p99 | 11.4 → 10.5、19.7 → 16.9 ms（噪声） |
| `BenchmarkTrashDelete` 5 万个节点，无统计（每次运行 3 次的平均） | 1,571 → 1,720 ms（+149，+9%） |
| 同上，ANALYZE 之后 | 1,816 → 2,242 ms（+426，+23%）；8 对里每一对都是分支慢 |
| `BenchmarkRevert`：满负载的删除与它的恢复 | 噪声之内 |
| `BenchmarkRevert`：新建 1 万个（另有 0 / 10 万个节点） | 中位数 +16 到 +29 ms；分支有一次 1,043 ms |

- 一页的保存多的 0.3–0.5 ms 合第 1 步的一页入队 0.49 ms。最大的回收站删除（导入的 5 万个节点）把 5 万个节点入队，仍在 3 秒之内，是请求的 15 秒的五分之一；数移交 M12。界线处的撤销入队至多 1 万个节点，约 20 ms，在这些运行的噪声之内。

**取舍的规则**（照做了，结果见上）：

- **字母文字**：
  - CJK 的部分两种写法相同（spike 量过二元组，这里再量一遍）。
  - T 在每个字母的类别（3 个及以上，含走查最先用完的那个）都达到 p99 与计数的目标、体积与吞吐也达标时用 T。
  - 否则量 P 的这些类别，P 达标就用 P：语义改为"词的开头"，4.12、契约与 SE1 随之。
  - 两者都不达标，停下来问负责人。
- **`MaxSearchWalk`**：{500, 1,000, 2,000, 5,000} 里，最坏的类别（走查用完而不满一页）p99 ≤ 300 ms 的最大一档。初值 1,000。
- **`MaxSearchCountScan`**：初值 20,000，第一页的 p99 超过目标时减半，直到达标。
- **名称层**：最常见的键的名称层超过目标时，名称层也按走查的界线有界地查，与正文层同样带"继续查找"。
- **`fastupdate`**：默认开。待处理列表满时查询的 p99 不达标，就关掉，再核对任务的吞吐。
- **`MinSearchRebuild`**：30 秒。编辑中的大页让任务的吞吐不达标时加长。

**守卫**（`page/adapter/postgres/search_cost_test.go` 的 `TestTheSearchReadsInTimeAtAnyStatistics`，`//go:build !race`；时限表在没有构建标签的 `search_guard_test.go`）：

- 在 `test-go-costs` 里单独一行、带 `-v`，同 P2–P4 的守卫：CI 的日志看得到每个读的耗时（P5A 审查 C-M2；第一版在共用的 `-run` 列表里、不带 `-v`）。
- **语料**：确定的合成语料，5,000 页建好（英文与中文，每页约 4 KB），其中 500 页改写、以准确入队，另 5,000 个节点以批量排队。
- **用例**：每类的第一页与第三页、走查用完的那一类（`theirs`）、8 个词的、`building` 时的，共 20 个读，无统计、ANALYZE 之后、为任意的参数规划各读 8 次，共 60 个。
- **时限**：`max(3 × 中位数, 40 ms)`，`8c83ccb7` 的规则加一个底（P5A 审查 C-M2：三倍在 7–109 ms 上没有给本机的摆动留余地，同一格两次之间差到 2 倍；每个请求 8–14 次往返，不随 CPU 缩放）。守的陷阱在这个规模上是 97–143 ms（`电 #`、`the`、`的`，照 3.5 原样的连接、无统计），正常 4–7 ms。`TestTheSearchGuardsLimitsKeepItsTrapCaught` 核对每个时限至少 40 ms、至少中位数的三倍，陷阱的那几个读在无统计时的时限低于 97 ms。
- 表在 `443712e0` 上重量（三次运行的中位数的中位数，负载 5–7；ms，无统计 / ANALYZE 之后 / 通用）。`latency`、`go` 的第一页与 8 个常见的词在 `f1b5cb55` 上再量：修复二轮撤回了短的走查之后的取样，它们不再多读一次正文层（三次运行，负载 6–8，与 `480c031f` 交替跑，那里的两次是 24–30、25–29、59–68 ms）。

  | 读 | 中位数 | 时限 |
  |---|---|---|
  | `中国 eng` 第一页 | 16 / 16.1 / 19.2 | 48 / 48.3 / 57.6 |
  | `中国 eng` 第三页 | 12.1 / 12.4 / 13.6 | 40 / 40 / 40.8 |
  | `的是` 第一页 | 20.9 / 21.1 / 23 | 62.7 / 63.3 / 69 |
  | `的是` 第三页 | 11.9 / 11.8 / 12 | 40 / 40 / 40 |
  | `latency` 第一页 | 17.1 / 16.8 / 23.4 | 51.3 / 50.4 / 70.2 |
  | `latency` 第三页 | 9.8 / 10.1 / 10.8 | 40 / 40 / 40 |
  | `the` 第一页 | 6 / 6.4 / 7.2 | 40 / 40 / 40 |
  | `the` 第三页 | 4.7 / 5.3 / 5.4 | 40 / 40 / 40 |
  | `go` 第一页 | 16 / 15.9 / 21.4 | 48 / 47.7 / 64.2 |
  | `go` 第三页 | 12.1 / 15.8 / 12.7 | 40 / 47.4 / 40 |
  | `的` 第一页 | 7.4 / 7 / 6.9 | 40 / 40 / 40 |
  | `的` 第三页 | 5.1 / 5.1 / 5.2 | 40 / 40 / 40 |
  | `电` 第一页 | 3 / 3.2 / 3.4 | 40 / 40 / 40 |
  | `电 #` 第一页 | 5 / 5.6 / 7 | 40 / 40 / 40 |
  | `中国 database` 第一页 | 36.6 / 39 / 43 | 109.8 / 117 / 129 |
  | `theirs` 第一页 | 33.8 / 39.5 / 38.7 | 101.4 / 118.5 / 116.1 |
  | `theirs` 第三页 | 28.1 / 31.4 / 30.1 | 84.3 / 94.2 / 90.3 |
  | 8 个常见的词，第一页 | 28.2 / 29.3 / 35 | 84.6 / 87.9 / 105 |
  | `中国` 全部、建索引时，第一页 | 17.2 / 17.8 / 18.9 | 51.6 / 53.4 / 56.7 |
  | `中国` 全部、建索引时，第三页 | 12.5 / 12.6 / 13.2 | 40 / 40 / 40 |

- **建与入队**：`Drain` 建 5,000 页时限 7 秒（2.2–3.9 秒），1 万个 id 的 `QueueSearch` 57 ms（18–20 ms）。`480c031f` 的门禁里 60 个读都在时限的 0.45 之内，最紧的是 `latency` 第一页通用 35.8 对 79.5 ms；建 2.29 秒，入队 19.3 ms。`f1b5cb55`、`fdc09444` 的门禁里都在 0.45、0.40 之内，建 2.46、2.24 秒，入队 19 ms。CI 约慢一倍，CI 上的中位数在作业的日志里，没有读（API 要认证）。CI 上有读失败时重量中位数，不单抬一个时限。
- 300 ms 的目标由 30 万页的基准核对，守卫只抓计划的退化（秒对毫秒）。

**CI**：成本那一份多了搜索的守卫单独的一行（多一个测试进程与它的数据库）；竞态检测下组合根的两份加了 14 个整个程序的测试（4 个落在第一份）与修复一轮的 2 个（都在第一份）。`dfafcf7a`（run 38736007129，作业 / 测试一步）：竞态 7:25 / 6:52、组合根一 5:40 / 5:12、组合根二 8:29 / 7:59、成本 4:25 / 3:20、守卫 7:02 / 6:27；审查的修复 `480c031f`（run 38762693235）：竞态 7:30 / 6:52、组合根一 6:15 / 5:44、组合根二 7:23 / 6:48、成本 6:16 / 4:57、守卫 10:32 / 9:59；修复二轮 `f1b5cb55`（run 38814229551）：竞态 4:49 / 4:15、组合根一 4:44 / 4:07、组合根二 6:04 / 5:33、成本 6:19 / 4:59、守卫 10:04 / 9:34；修复三轮 `fdc09444`（run 38823209418）：竞态 7:22 / 6:50、组合根一 5:32 / 4:56、组合根二 7:46 / 7:10、成本 6:38 / 5:06、守卫 6:37 / 6:06。两份组合根离 `go test` 的 15 分钟都远，不用再分。web、e2e 与镜像见第 9 节。

### 3.10 测试

- **domain**：
  - 分词：
    - 每类字符与段（CJK、字母、数字、`Mn`、`Mc`、日文的长音、韩文、天城文）。
    - 64 的界线只对 ASCII 的字母与数字（泰文 73 个码位、高棉文 71 个码位的分句照常进索引，`TestTheKeysOfALongRunOfAnotherScript`）、`data:` 串到它的末尾（`TestSearchTokensLeaveADataURLOutToItsEnd`）、按折叠之后的长短（ß、ﬃ）。
    - 标题与正文的差别（单字只在标题）。
    - 词的次序与 `MaxSearchLexemeBytes` 处的截断（字节之和）。
    - 泰文等带 `Mn` 的字母段（spike 的 Python 分词在这里与 Go 的不同）。
  - `ParseSearchQuery`：
    - 每种取值的错误：缺了、空、257 字节、9 个词、65 个字母的段、NUL、坏的 UTF-8、全是标点。
    - 全角空格切词、去重、每种词的键与"只搜标题"（`锅`、`锅！`、`A股`、`锅 #`）；被隔开的单字的词与单独的情形（`中·国`、`中·国 数据库`，`TestSearchQueryTermsFoundInNamesAlone`）。
    - 模式的边界：Go语言、C++、node.js、A股、单个字母、数字。
    - 大小写的类：每个多于一个成员的 `SimpleFold` 轨道，与完全折叠的特殊字符（ß、ẞ、İ、ı、ſ、K、µ、ǅ、ς、ﬃ）。
    - 两种写法的模式的转义（`%`、`_`、`\`、`[`、`(`、`.`）。
    - 每个简单折叠的轨道的每个成员同键（`TestEachOrbitKeysAlike`，切罗基文的换）。
  - **复查蕴含键**（`TestTheRecheckImpliesTheKeys`，性质测试）：生成的文字与查询（ASCII、CJK、标点、空白、上面那些特殊字符、带重音的字母），RE2 的模式通过时查询的键都在 `SearchTokens` 的词里；生成器另造 3.3 的例外，核对它们正是例外。
  - 高亮：
    - 开头、结尾、多字节、代理对（emoji 在名称里）的 UTF-16 区间。
    - 上下文的字符：摘要开头的 1–2 个字母的词前面是字母时不高亮；空白合并之后的区间。
    - 相邻的整词；不同的词重叠的区间合并。
  - 计数的估计：`exact`、`about`、`at_least`（数到这次搜索的上限、到 1,000、还有没查的而比例依据的通过少于 2 个）、按层（名称多而正文的通过率低）、样本、没有一个通过；页满时的比例与停在界线的比例（修复二轮），页满停在第 2 个通过时 `at_least`、页满而走完只查了一个候选（修复三轮）。
  - 游标：层的切换、走查恰好用完、UTC。
- **用例**（`page/app`，假的端口）：
  - 码的次序：游标 → `q` → 工作区 → 判定 → 笔记本 → `limit`。
  - `notebook_id` 看不到时 404 而不是空列表。看得到的为空时答空、`count` 为 `exact` 0。
  - 两层：名称层不足时补正文层；走查的界线；三种 `next_cursor`；只在第一页计数（审查 C 的形状：名称层快填满第一页、正文层早早有一个通过，`TestTheCountOfAShortWalkLeavesItsLastPassOut` 钉住 C 的表与正文层只读一次；走查的界线上的通过按 k / n）；`building`。
  - 附件的信息与路径的组装。
  - 入队：四种情形、移动不入、导入与后台的恢复、撤销以批量、id 已排序（`TestTheUnitQueuesWhatItChanges`）。
  - 任务（假的存储与时钟）：
    - 取不到锁时返回。
    - 一遍按 `node_id`、每个节点至多读一次；一批 8 MiB 的界线与 `@after`。
    - 删、不动、30 秒之内跳过、重建四种判断；分词的版本不同时重建。
    - 出队带着读到的版本号；第 4、5 步之间中断；写不进的节点逐个再试、记下、跳过，别的照常出队。
    - 每 10,000 行 `ANALYZE`、计数跨运行；1 分钟的预算。
    - 先做准确的、再做批量的（`TestTheSearchJobTakesTheExactNodesFirst`）；开始不了的事务让运行以一个错误结束、不逐个试、不记一行、节点都留在队列里，错误带着这一批的失败（`TestTheSearchJobEndsWhenATransactionCannotBegin`，修复二轮、三轮）。
- **适配器**（真的 PostgreSQL）：
  - `QueueSearch` 的版本号取自序列、批量的与，`ORDER BY 1`；`DequeueSearch` 不删版本号变了的、跳过被持着的行；advisory lock 一次只给一个。
  - 两层的语句：
    - 子串、整词、只搜标题、每个词在名称或正文、名称层的排除、正文层没有键时以标题的单字找候选。
    - 准确的队列里的节点不靠索引也找得到；批量的不当候选、`building` 为真；已删的（含还有行的）与看不到的不在。
    - 走查的最后一行与 `walked`；摘要的位置。
  - 计数到 `cap`、名称层到 1,001、页的数。
  - 锁的连接：一个连接的池上一次运行建好每一行，事务里 `work_mem`、`gin_pending_list_limit` 是 64MB、16MB，事务之后与运行之后连接回到默认（`TestTheSearchsJobRunsOnItsLocksConnection`，P5A 审查 B-M1）；运行的连接断了（一个结束自己的后端的触发器）时运行以一个错误结束、带这一批的 SQLSTATE，节点都留在队列里，下一次运行写好（`TestASearchRunWhoseConnectionEndsFailsOnce`，修复二轮、三轮）。
  - 不是 NFC 的正文：分解的越南文与谚文字母，NFC 的词在每一层、排队时与建好之后都找到，只有基本字母的词都找不到（`TestTheSearchReadsContentNotInNFC`）；被隔开的单字的词排队时与建好之后都找到正文（`TestTheSearchFindsATermOfSingleCJKCharactersInContents`）。
  - **模式的性质测试**（`TestThePatternsMatchInPostgreSQLAsInGo`，7,656 例）：生成器含每个多于一个成员的 `SimpleFold` 轨道与完全折叠的特殊字符，每个模式在 PostgreSQL 的 `~` 与 Go 的 RE2 里答得一样，`regexp_instr` 的位置等于 Go 的第一个捕获组。
  - 迁移测试（3.2）。
- **计划测试**（`search_plans_test.go`，`planOf` 再复制一份，同 P3）：在种下、ANALYZE 过的几千页上，关掉整表扫描（队列除外：它的整表扫描是对的）。
  - 两层分别走 `page_search_title_idx`、`page_search_body_idx`，`nodes` 按主键。
  - 正文层：走查的 `Limit` 直接在它的 `Sort` 之上，中间没有 `WindowAgg`；`page_contents` 在走查之后按主键读；`EXPLAIN VERBOSE` 里正文的拼接只出现一次；外层没有第二个排序。
  - 名称层：名称的检查在 `Limit` 之下，正文在 `Limit` 之上按主键读。
  - 计数走 GIN；`building` 走队列的索引。
  - 任务的读按主键；`TrashLines` 按键。
  - **实施**（`TestTheSearchReadsUseTheirIndexes`，4,000 页无统计与 44,000 行 ANALYZE 之后两个阶段）：每个 `nodes` 的扫描只按笔记本的条件读、不读部分索引；连接的条件里没有该在围栏之上的检查；名称从不在 `nodes` 的扫描里；`the` 没有嵌套循环为每个节点重扫候选；函数的设置含 `work_mem=64MB`；任务的准确的一遍按键或 `(notebook_id, bulk)` 的索引读，批量的一遍按键。围栏的每个变异在两个阶段的每一例都失败。
- **整个程序**（`bootstrap/search_test.go`，13.1 第 21 条）：
  - 组合根不交 `VisibleNotebooks` 时 `page.New` panic。
  - 经接口：
    - 新建页与附件、写正文、改名、移动、删除、从回收站恢复、撤销、还原版本、后台的恢复与撤销。
    - 每一步之前先让任务跑过，之后搜索立即反映它、答复不带 `building`：只有这次写自己的入队能让它找到（第一版不先跑任务，4 个变异存活）。后台的恢复与撤销同导入：`Drain` 之前找不到、带 `building`，之后找到。
    - `TestABackgroundRestoreLeavesASearchItsHits`：`MaxSearchWalk` + 101 页的后台恢复之后，五页旧页的罕见词仍在第一页（`exact 5`、`building` 为真、没有游标），`Drain` 之后相同（P5A 审查 C-I1 的形状）。
    - 导入一个小 zip：任务跑之前它的页不在结果里、带 `building`；`Drain` 之后找得到。
    - 做空队列之后 `checkSearch` 成立。
  - River：任务在 `page_search` 队列上（同 `TestTheJobsHaveTheirQueuesAndOutlastRiversRescue`）；间隔 1 秒时完成一次之后再次运行（唯一的选项不含 `completed`）；停下的服务留在 `running` 的行在下一次启动时丢掉，几秒之内新的运行完成（`TestASearchRunLeftRunningByAStoppedServerIsDiscardedAtTheStart`）。
  - `nervewiki reindex`：行是当前的时不入队；把行的 `tokenizer` 改旧之后以批量入队，`Drain` 之后是当前的；一本笔记本的入队失败时那一行写明、命令接着做别的（`TestReindexTellsANotebookWhoseNodesAreNotQueued`）。
- **不变式** `checkSearch`：
  - 不做空队列也成立：每个活着的节点有 `revision`、`name` 是现在的行，或在队列里（分词的版本旧了的行在 reindex 之前照常回答，不算）。
  - 做空之后：队列为空；`page_search` 的行恰是活着的节点的；`title`、`body` 等于 `SearchTokens` 的。
  - 加进交错与整个程序的收尾核对。
- **增量等于重建**（`search_rebuild_test.go` 的 `TestTheSearchIndexIsItsRebuild`，13.4 第 10 条）：随机的操作序列，沿用 P3 的性质测试的生成器（新建页与附件、改名、移动、删除、恢复、写正文、还原、撤销），另加一次小的导入；每隔几步做空队列，`page_search` 等于从头分词的结果。固定的几个种子。
- **矩阵与可见性**：3.8。
- **handler 与平台**：契约的结构、参数与各码；`count` 的界线与种类。`TestTheJobsLeftRunningAreDiscardedAtTheStart`（platform/jobs）、`TestOnConnRunsTheStatementsOnTheConnection`、`TestATransactionThatCannotBeginSaysSo`（platform/postgres）、`TestTxNotBegun`（shared）、`TestValidateCrossKeyRules` 的搜索的任务的连接与留不出连接时的消息。

### 3.11 审查之后的改动

三位审查者（A：分词、查询的语义与命中；B：索引的维护、队列、任务与并发；C：读、权限、契约、实测与守卫）与修复核对的处置见 `reviews/P5A-search-server-review.md`。与上文最初的设计不同之处已改在各节里：

- **实施时定下的**（实施者的 16 条偏离，三位审查者判过）：正文层的候选经 `page_search_hits` 与数出的 `LIMIT`、候选只带笔记本的条件连接（偏离 1，3.2、3.5、3.9）；计数分成名称层与正文层两条、名称层定下时不数正文层（偏离 2，3.5）；任务的写的设置（偏离 3，3.4、3.9）；队列的索引名（偏离 4，3.2）；`Querier.SendBatch`（偏离 5，3.1）；`Deps.Snapshots`（偏离 6，3.1、3.6）；`page.NewSearchIndex(pool, commitTimeout)`（偏离 7，3.4）；语句的 `coalesce`、正文有命中时才给摘要（偏离 9，3.5）；linking 的 reindex 撞键时照样入队（偏离 10，3.4）；"复查蕴含键"的已知缺口（偏离 11，3.3，审查 A-N1 写准）；`VisibleNotebooks` 交空的测试在模块的测试里（偏离 12）；`BenchmarkSave` 在自己的文件里（偏离 13，3.1）；`checkSearch` 在 `checkPages` 里（偏离 14）；访问规则 `page.search`（偏离 15）；README 留到合并（偏离 16）。偏离 8（没有正文键的词只搜标题）由审查 A-M2 改回 3.3 的规则。
- **审查与修复核对带来的**：后台的恢复与撤销以批量入队、任务先做准确的（C-I1、B-M3，3.4）；64 的界线只对 ASCII 的字母与数字，分词的版本 2（A-I1，3.3）；`data:` 串到它的末尾（A-M3，修复核对第一轮 N-7，3.3）；切罗基文的小写换成大写（A-M1，3.3）；`SearchTokenizer` 带 x/text 的版本（A-N5，3.3）；不是 NFC 的正文按它的 NFC 读（A-M4，3.3、3.5）；被隔开的单字的词与单独的情形（A-M2，修复核对第一轮 N-3，3.3、3.7）；`page_search_hits` 的 `work_mem` 64 MB（C-M3，3.2）；计数：比例依据的通过太少时 `at_least`、候选数到这次搜索的界线、页满时的无偏比例，取样只在第一页的走查没有跑时（A-M5、A-N6、C-M1、修复核对第一轮 I-1、第二轮 F-1，3.5）；启动时丢掉留在运行中的行、一次运行只试一次（B-I1，3.4）；任务在它取锁的连接上跑，连接池的规则算上它，`max_conns` 至少 6，留不出连接时消息说 `max_conns` 至少要多少（B-M4、B-S3、B-N3、修复核对第一轮 N-5、第二轮 N2-2，3.4）；开始不了的事务让运行以一个错误结束，错误带着这一批的失败（修复核对第一轮 M-1、第二轮 N2-1，3.4）；任务的设置只在它的事务里的测试（B-M1，3.10）；摘要的边上的命中（A-N2，3.5）；reindex 的入队失败（B-N2，3.4）；`ANALYZE` 跨运行计数（B-N4，3.4）；交错 3 真的删掉行（B-N1，3.8）；契约的走查界线、折叠相同的词、`exact` 含只在名称里找的、`at_least` 的三种、后台的恢复与撤销要等任务、摘要的 NFC、422 的用词含组合记号（A-N3、C-N1、A-N7、C-N2、修复核对第一轮 M-2、N-1、N-2、第二轮 N2-4，3.7）；守卫的时限加 40 ms 的底、单独一行带 `-v`（C-M2，3.9）。三轮修复核对见审查记录，第三轮没有剩下的行为上的发现。
  - README 随合并改：`reindex` 一行与"链接索引"一段、部署一节的 `q` 与大的补入或 reindex、任务一句、`max_conns` 至少 6、`page_search` 的 `MAINTAIN`。

## 4. B：前端

### 4.1 文件

| 文件 | 内容 |
|---|---|
| `web/apps/web/src/services/page.service.ts` | `search(slug, q, {notebookId, cursor})`（每页 20）与类型（`SearchPage`、`SearchHit`、`SearchCount`、`TextRange`） |
| `web/apps/web/src/app/shortcuts.ts` | `isModShift(keys, key, mac)`：Cmd（mac）或 Ctrl 加 Shift、不带 Alt，字母同 `isMod` 按 `key` 或 `code` |
| `web/apps/web/src/pages/workspace/search-box.tsx`、`workspace-layout.tsx` | 左栏顶部的搜索框、范围的提示、`Mod+Shift+F`、组合中的回车 |
| `web/apps/web/src/pages/search/search-page.tsx`、`search-hit.tsx`、`search-status.ts` | 结果页、一条结果、状态行的文字 |
| `web/apps/web/src/lib/text-ranges.ts` | 按 UTF-16 区间切出 `<mark>` 的段（契约保证排好、不相交，只截到长度） |
| `web/apps/web/src/lib/open-node.ts` | `openOf` 从 `trash-row.tsx` 移来（回收站与搜索共用） |
| `web/apps/web/src/reading/reading.css` | `--nw-mark-background` 移到 `:root` 与 `.dark`（结果页在 `.nw-reading` 之外） |
| `web/apps/web/src/app/routes.tsx` | `:slug` 下的 `search`（与 `notebooks/:id`、`settings` 同级，按需加载） |
| `web/apps/web/src/i18n/messages/{zh-CN,en}.ts` | 前缀 `search.` |
| `web/apps/web/src/test/search-server.ts`、`test/page-server.ts` | 假接口：码的次序同服务端；子串与整词的简化语义；两层；`next_cursor` 与走查的界线（`walk` 开关）；`count` 的三种；`building`；开关 `searchWaits`、`searchDown` |
| `e2e/fixtures/search.ts`、`wiki-search.ts`；`e2e/stories/search/se1-search.spec.ts`、`se2-search-visibility.spec.ts` | 4.6 |
| `docs/v0.1/M4-pages/manual/P6-ime-checklist.md` | 第 21 步与表头 |

### 4.2 搜索框与快捷键

- **位置**：`WorkspaceLayout` 的左栏最上面、工作区的 `nav` 之前。
  - 00 号文档 4.13 已定。顶栏在 `:slug` 之上，没有工作区的上下文，登录页、个人设置页也显示它。
  - 窄屏时左栏在 `main` 之上，框仍在最上面。
- **结构**：`<form role="search" aria-label="搜索「工作区」">` 里一个 `<input type="search">`，有看不见的标签（不靠 placeholder）；一个带名称的提交按钮。
  - 输入框的 `aria-describedby` 指向范围的提示（与本地检查的说明），读屏说出范围。
  - `aria-keyshortcuts` 写 `Control+Shift+F`，mac 上写 `Meta+Shift+F`。
  - 这个表单只导航、不发请求，不经 `useForm`（13.2 第 12 条的例外）。
- **范围**：
  - 在一本笔记本里（`useMatch("/:slug/notebooks/:id/*")`）时提示"在「笔记本」里搜索"，提交时带 `notebook_id`。不靠 `useParams` 在父路由看得到子路由的参数。
  - 在结果页时带它当前的 `notebook_id`。
  - 否则提示"搜索整个工作区"。
- **提交**：
  - 去掉首尾空白，为空时什么都不做。
  - 本地只查 256 字节与 NUL，不合时在框下说明（`aria-invalid`），文字同服务端 422 的字段码。词数、字母段的长短与"没有键"由服务端答，结果页说明：JS 做不到服务端的完全折叠，本地数词会与服务端不同（ß/SS）。
  - 合格时去 `/:slug/search?q=…&notebook_id=…`（压入历史）。
  - **焦点**：从别的页提交时带 `arrived`，结果页的 `h1` 取得焦点、经 `aria-describedby` 读出状态行，同快速切换（左栏的笔记本列表没有上限，焦点留在框里要 Tab 过整个导航才到结果）。在结果页上提交时焦点留在框里，状态行礼貌地说出数目。
- **组合中的回车不提交**：`onKeyDown` 遇到回车时，`nativeEvent.isComposing` 或 `keyCode === 229` 就 `preventDefault`（Safari 在 `compositionend` 之后补发确认用的回车，带 229），阻止隐式提交。
- **在结果页时**框里是地址的 `q`：地址变了才同步，正在编辑时不覆盖。
- **`Mod+Shift+F`**：
  - 框的组件在 `window` 上听 `keydown`，`isModShift(event, "f", mac)`。
  - `repeat` 或组合中（`isComposing`、`keyCode === 229`）时不管：焦点离开编辑器会让浏览器把半截拼音提交进正文（13.2 第 20 条）。
  - 确认框开着时只 `preventDefault`（挡住 macOS 上 Firefox 的全屏、Chrome 的工具栏），焦点不动，同快速切换。
  - 否则 `preventDefault`，焦点到框并选中文字。
  - CodeMirror 没有绑定 `Mod-Shift-F`，编辑时同样可用。
  - 它在微软拼音、搜狗与 Linux 的 fcitx、fcitx5 里是简繁切换（输入法可能先吃掉），所以看得见的框是主要入口；进输入法清单。
- `Ctrl+O` 的快速切换不变。

### 4.3 结果页

- **路由** `/:slug/search`：在笔记本的外壳之外，没有页面树与 `Ctrl+O`。
  - 文档标题"搜索：q · 工作区"。
  - `h1` "搜索「q」"，`q` 放在 `<bdi>` 里。
  - 地址里没有 `q` 时只说怎样搜，不读。
- **范围的选择**（`NativeSelect`"在哪里找"）：
  - 选项：全部笔记本与看得到的笔记本（`["notebooks", 工作区]` 的列表），同名的带编号（`twinsOf`、`shortId`）。
  - 改了就替换地址（不压历史），焦点留在选择上，礼貌地说出数目。
  - `notebook_id` 不是 UUID 时不读，说"找不到这本笔记本"，给"搜索整个工作区"的链接。404 `notebook.not_found` 同样。
- **读**：
  - SWR 键 `["search", 工作区 id, q, notebook, location.key]`，`revalidateOnFocus`、`revalidateOnReconnect`、`revalidateIfStale` 都关（同版本页的 `unchanging`）。
  - 每次提交压入一个新的历史条目、换一个 `location.key`，所以同样的 `q` 再提交也重新读、重新播报：删掉的页消失，新写的出现。后退回到一个条目时用它的键，显示缓存里的结果与已读的"更多"。
  - 缓存只留当前的一个搜索键：换键时去掉别的搜索键。后退回到刚离开的结果页仍是缓存，更早的重新读。
  - 不进事件的处理表与 `refreshedOnConnect`：结果页不随事件、焦点、重连重读（00 号文档 4.13）。
- **状态行**（一开始就在的 `<output>`，经 `aria-describedby` 描述 `h1`）：
  - 第一页：`exact` "23 条"、"没有找到"；`about` "约 240 条"；`at_least` "至少 800 条"，1,000 时 "1,000+ 条"；`count` 为空时"还没有找到，可以继续查找"。
  - 读了更多页之后由已读的定：读完了（没有游标）是已读的条数；`count` 为空或小于已读的时"已找到 {n} 条，可能还有更多"；否则照第一页的。
  - `building` 时另加一句"索引正在建立，结果可能不全"。
  - **播报**：提交、换范围时说状态行；每次"更多"或"继续查找"之后礼貌地说"又找到 3 条"，一条也没加时"这一段没有找到，可以继续查找"。同样的文字再说时加不断行的空格，同历史页。
- **列表**：`<ul aria-label="搜索结果">`，接上的一页按 id 去重（13.2 第 19 条：一次改名可以让节点从名称层移到正文层，又出现一次），每条：
  - 类型的图标：页 `ScrollText`，附件 `AssetIcon`，`aria-hidden`。
  - 名称的链接：高亮在 `<bdi>` 里。
  - 路径："笔记本 / A / B"，笔记本名取自列表；读不到时写"笔记本"。
  - 摘要：`<bdi>` 里，截断处加"…"。
  - 最后改动（`changed_at`）：相对时刻，`<time>` 的提示是完整时刻。
  - 附件另带类型与大小（`formatBytes`）。
- **高亮**：`text-ranges.ts` 按区间切出 `<mark class="nw-hit">`。底色用 `--nw-mark-background`（移到 `:root`、`.dark` 之后结果页也有），另加粗，不只靠颜色。
- **去处**：
  - 页去 `/:slug/notebooks/:nb/pages/:id`，带 `arrived`。
  - 附件经 `openOf` 去它所在的位置：父页的附件一节（读到它那一行时焦点到那一行），根下的在笔记本首页，路由状态 `Located`，同 P2B。
  - 不签发下载地址。
- **链接的名称**是节点的名称。
  - 同名的（同一个名称、不同的位置）带路径："「名称」（笔记本 / A）"，同回收站的 `namesOf`。
  - 名称与路径都相同时（两本同名的笔记本里）再带编号。
- **"更多"**：`useLoadMore` 与 `readNext`，读到的一页写进"更多"开始时的那个键。
  - 上一页一项也没有而带着游标时（走查的界线），按钮写"继续查找"。
  - 焦点：读完最后一页、按钮消失时到第一项新加的（`useLoadMore` 的 `whole`）；还有游标时留在按钮上，由状态行的播报说出结果。
- **错误**：
  - 422：在页上说明（字段码的文字：太长、太多词、连续的字母太多、只有符号）。框下只说本地检查的。
  - 404 `workspace.not_found`：外壳照旧换成"页面不存在"。
  - 5xx 与网络：照 `onErrorRetry`，页上给"重试"。

### 4.4 无障碍与文字

- 每个新页面设文档标题；列表有名称；结果的链接名称带区分。
- 焦点：从别的页提交时到结果页的 `h1`；在结果页提交与换范围时留在控件上；"更多"见 4.3；读者动了就不抢（`watchReader`）。
- 文字：英文不用复数形式。"Results: {count}"、"Results: about {count}"、"Results: at least {count}"、"Results: {count}+"、"Found {count} so far; there may be more"、"{count} more found"、"Nothing found in this stretch; you can keep looking"、"The index is being built; results may be incomplete"；数字经 `Intl.NumberFormat`（所以是 "1,000+"）。
- 中文的"约 {count} 条"、"至少 {count} 条"、"{count}+ 条"、"已找到 {count} 条，可能还有更多"、"又找到 {count} 条"、"这一段没有找到，可以继续查找"、"索引正在建立，结果可能不全"、"继续查找"。

### 4.5 测试（vitest）

- **快捷键**：`isModShift` 的平台、Shift、Alt、非拉丁的键盘布局（`code`）；`isMod` 仍拒绝 Shift。
- **搜索框**：
  - 范围的提示、`aria-describedby` 与 `notebook_id`（在笔记本里、在结果页、在别处）。
  - 本地的检查与文字（字节、NUL）。
  - 组合中的回车（`isComposing`、229）不提交、确认之后的回车提交。
  - `Mod+Shift+F` 聚焦并选中；组合中不管；确认框开着时 `preventDefault`、焦点不动。
  - 焦点：从别的页提交时到结果页的 `h1`；在结果页上留在框里。
- **结果页**：
  - 每种状态行的文字（两种语言，三种 `count`、1,000 处、为空、`building`、读了更多页之后）。
  - 播报："又找到"、"这一段没有找到"、同样的文字再说。
  - 提交同样的 `q` 时重读并播报；后退不重读；再读一次时显示别处的变化（假接口在两次提交之间删掉一项）。
  - 高亮的区间（多字节、代理对、相邻的区间、截到长度）。
  - 摘要的截断。
  - 页与附件的去处（父页、根下）。
  - 同名的区分；接上的页按 id 去重。
  - "更多"与"继续查找"、焦点。
  - 范围的选择与不读的地址（不是 UUID）。
  - 404 与 422 的说明。
  - 获得焦点、重连、`pages` 事件时都不重读（推进时钟之后数请求）。
  - 经组合根到达（`renderApp`，应用自己的路由与事件处理表）。
- **假接口**：
  - 码的次序同服务端：游标 400 → `q` 422 → 工作区 404 → 笔记本 404 → `limit` 422。
  - 两层的次序。
  - 走查的界线开关让一页少于 `limit` 而带游标。

### 4.6 e2e

- **SE1**（`se1-search.spec.ts`，页面版本与 PAT 的接口版本调用同样的断言）：
  - 一本笔记本里写：
    - 标题含"数据库"的页。
    - 正文含"全文检索"、"Go语言"与"仓库"的页（标题里没有"库"）。
    - 正文含 "rapid" 与单独的 "AI" 的英文页；另一页只含 "said"。
    - 名称为"季度报告.pdf"的附件，在一页下。
  - 2 个字（"检索"）、3 个字以上（"全文检索"）、混合（"Go语言"）、字母的子串（"api" 找到 rapid；若定为 P，改成 "rap"）、两个字母的整词（"ai" 找到单独的 AI，不找到 "said" 那一页）。
  - 标题与正文的命中、摘要里的高亮、准确的数目（数据小，第一页就走完）。
  - 单个汉字"库"只找到标题含它的页，不找到只在正文里含它的页。
  - 附件按名称找到，"打开"去父页的附件一节，焦点在它那一行。
  - 删除那一页之后立即不再出现；从回收站恢复之后立即回来。页面版本在同一个标签页里经左栏的框再搜一次，不重新加载。
- **SE2**（`se2-search-visibility.spec.ts`）：
  - 管理员建：开放可编辑的笔记本、私密笔记本、只把访客显式加入的笔记本，各有带同一个词的页。
  - 成员看不到私密的。
  - 访客只看到被加入的那一本。
  - `notebook_id` 限定一本；看不到的那本答 404，页面上说明并给"搜索整个工作区"。
  - 页面版本另核对：在笔记本里提交时默认限定这一本，换成"全部笔记本"。
- **数据库**：SE 不写库，不做数据库的断言（10.1 的例外，00 号文档第 9 节已写）。e2e 的服务照默认每 5 秒跑任务，SE 不依赖它跑过没有。
- **夹具**：
  - `fixtures/search.ts`：`search(api, pat, slug, q, {notebookId, cursor, limit})`。
  - `wiki-search.ts`：`searchFor(page, q)` 经左栏的框、`searchResults(page)`。
  - 访客与第二个账户经 `joinAs`、`joinOnboarded`。
- **反向对照**：
  - 只搜标题的单字改成搜正文。
  - 整词改成子串。
  - 结果页的键不带历史条目（同样的 `q` 再提交不重读：删除之后再搜仍列出它）。
  - 附件的去处。
  - 范围的默认值。
  - "去掉队列的候选"不在 e2e 里做：任务每 5 秒一次，它失败与否看时机；交错 3 确定地核对它。

## 5. 实施步骤

A（分支 `m8-p5a`，从 `main`）：

1. **实测**（3.9 第 1 步）：重建语料、加工具、量；按规则定下写法与几个界线，写回本文 3.9 与第 9 节（只改文档）。
2. **迁移**与 schema、迁移的测试；运行时的授权、`sqlc.yaml`。
3. **domain**：分词（定下的写法、版本）、`ParseQuery`、两种写法的模式与大小写的类、高亮、游标、计数的估计；单元测试与复查蕴含键的性质测试。
4. **存储**：语句、单元的入队、笔记本删除的入队；适配器测试、模式的性质测试、计划测试。现有的测试照旧通过。
5. **任务**：`Drain`（锁、一遍、最短间隔、失败的节点）、River 的任务（唯一、时限）、配置、队列的接线、reindex 的入队；种子的 `queueSeeded`；`checkSearch`、增量等于重建、交错。
6. **搜索**：用例、端口（`FindBySlug`、`VisibleNotebooks`）、动作与规则、契约、处理器、`make gen`；矩阵两行、可见性的集成测试、整个程序。网页的门禁：生成的类型，`TrashAttachment` 改名。
7. **守卫**：守卫与 Makefile（时限照本机的三倍）；`BenchmarkSearch`、`BenchmarkSave` 在 30 万页与两边上复现，重跑删除与撤销的基准，写回 3.9 与 M12 的移交。
8. **收尾**：反向对照、门禁、CI 三份的时间（组合根那一份过约 13 分钟时分开）、审查。

B（分支 `m8-p5b`，A 合并之后；服务函数与类型随第一次用到它的那一步加入，每一步 knip 都绿）：

1. 服务与类型、路由、结果页（状态行、列表、高亮、摘要、路径）、`text-ranges.ts`、`--nw-mark-background`。
2. 搜索框、范围、`isModShift`、组合中的回车与快捷键、焦点。
3. 附件的去处（`open-node.ts`）、同名的区分、"更多"与"继续查找"、播报、错误、文字。
4. vitest 补齐；e2e SE1、SE2；输入法清单第 21 步；反向对照、门禁、审查。

## 6. 测试与验证

- **门禁**：`make lint-go lint-web knip gen-check test-go test-web build`、e2e 全量。
- **反向对照**，各改坏一处，测试要失败：
  - A：
    - 分词：正文收了单字、64 的界线、`data:` 串、Mc 当作分隔、截断。
    - 查询：键、只搜标题、模式的边界与转义、大小写的类少了轨道的成员（如 ẞ）、65 个字母的段不答 422。
    - 维护：入队的四种情形、导入不以批量、`QueueSearch` 去掉 `ORDER BY 1`（交错 6）、版本号的出队、`SKIP LOCKED`、`Drain` 不取锁（交错 9）、最短间隔、失败的节点挡住整批、任务的判断。
    - 读：队列的候选、批量的进了候选、活着与可见的过滤、名称层的排除、走查的最后一行与 `walked`、三种游标、按层的计数、`at_least`、摘要的位置与上下文的字符、UTF-16 的区间。
    - 码的次序；`notebook_id` 的 404；数据库错误去掉消息；`VisibleNotebooks` 交空的 panic；River 的唯一含 `completed`。
  - B：4.5 的每种状态与焦点、组合中的回车与快捷键、不随事件重读、再提交重读、范围的默认值、附件的去处、去重。
- **审查**：每部分三位 Opus 审查者并行，之后 Opus 核对修复，直到一轮没有行为上的发现。
  - A：分词与查询的语义、实测与守卫；队列与任务的并发、可见性与权限；契约、测试与文档。
  - B：界面与无障碍；状态与读；测试与 e2e。

## 7. 完成标准

- 第 2 节的每一项做完，3.10、4.5、4.6 的测试通过，门禁全绿。
- 3.9 的目标在 30 万页上达到：第 1 步与第 2 步的数写回本文与 00 号文档 4.12，字母文字的写法定下。
- 审查记录：设计 [reviews/P5-design-review.md](reviews/P5-design-review.md)，实施 `reviews/P5A-search-server-review.md`、`reviews/P5B-search-web-review.md`；00 号文档的进度表与本文档的"结果"一节更新。
- **P5 的文档提交做的改动**（随本文档一起）：`00-M8-design.md` 的 2.1 的 16 条。
- **随 A、B 的合并改的**：
  - `v0.1-design.md`：
    - 1.3"中文搜索"一行：选择改为"应用计算的词（CJK 二元组、标题的单字、字母文字的三元组或词）存在 `page_search` 的 `tsvector` 里，GIN 索引，后台的任务维护，读时对当前的正文复查（仍然只要 PG）"；放弃的加"pg_trgm + ILIKE：30 万页时 1–2 个字 2.5–3 秒（M8 的实测）"。
    - 7.1：pg_trgm 一条改为"我们的表上没有三元组索引"；分词依赖 Go 的 Unicode 表，升级之后 `nervewiki reindex`。
    - 7.2：末句改为 `page_search` 与 `page_search_queue` 一行（派生，不带外键，没有 `deleted_at`）。
    - 8.5：任务表加搜索的建索引（`page.index_search`，自己的队列）。
    - 8.6（:799）：reindex 加"也让搜索的行以批量入队，由服务在后台重建"。
    - 9.2：左栏加搜索框；结果页。
    - 12.2（:912）：M8 一行的"全文搜索（pg_trgm + ILIKE …）"改为"全文搜索（应用计算的词与 GIN，按权限过滤，摘要片段）"。
    - 12.3（:932）："`grep` 用 M8 的三元组索引"改为"用 M8 的搜索索引（4.12）"。
    - 13.1：
      - 第 1 条：单元入队（导入以批量）；reindex 入队。
      - 第 3 条：搜索的集成测试名。
      - 第 4 条：`searchWorkspace` 的码的次序。
      - 第 5 条：队列的行按 id 升序、在加锁次序里的位置；任务从不等。
      - 第 6 条：`page_search` 照派生的索引表，任务删除行。
      - 第 10 条：搜索语句的错误只记 SQLSTATE。
      - 第 11 条：`page.NewSearchIndex(pool)`、`VisibleNotebooks` 给 page。
      - 第 15 条：`page.search_index_interval`。
      - 第 16 条：两张表与序列的授权。
      - 第 21 条：最后一跳，搜索经端口得到看得到的笔记本。
      - 第 23 条：`page.index_search` 在自己的队列、唯一的选项不含 `completed`。
      - 第 27 条：搜索的游标带层。
      - 第 31 条：`q`、`limit`、走查、计数的上限。
    - 13.2（B）：
      - 第 7 条：搜索的键带历史条目，再提交重读，不随事件重读。
      - 第 12 条：只导航的表单不经 `useForm`。
      - 第 17、19 条：列表与"更多"；第 19 条（:1147）的"四处共用"改为"五处"。
      - 第 20 条：`isModShift` 与 `Mod+Shift+F`，组合中不生效。
      - 第 23 条：不进事件的处理表。
      - 第 26 条：焦点。
    - 13.4：
      - 第 3 条：SE 不写库。
      - 第 4 条：搜索的交错与 `checkSearch`。
      - 第 6 条：矩阵；以 SQL 种下的节点要入队。
      - 第 8 条：新的守卫与 Makefile。
      - 第 9 条：计划测试，复查在走查之上。
      - 第 10 条：搜索的增量等于重建。
    - 14：
      - 中文搜索一行改写为实测之后剩下的局限（同 00 号文档第 10 节）。
      - glibc 一行改为"我们的表上没有三元组索引；`pg_trgm` 扩展与启动的自检照留"。
  - `README.md`：
    - 命令表（:57）的 `reindex` 一行与"链接索引"一段（:164）：也让搜索的行以批量入队，打印入队的数目，由服务在后台重建；建索引期间结果页写明可能不全。
    - 部署一节：`q` 进反向代理的访问日志。
    - 任务一句加搜索的建索引。
  - `M4-pages/manual/P6-ime-checklist.md`：
    - 表头（:3–9）：来源加 M8 总设计第 9 节与 P5 4.2；执行人加"M8 的第 21 步"；结果加"第 21 步写进 M8 收尾审查记录的'人工验收'一节"。
    - 第 21 步：
      - 搜索框里用拼音输入"shuju"、候选框开着时按回车：只确认输入法，不提交；确认之后回车才去结果页。
      - 在编辑器里拼音组合中按 `Ctrl+Shift+F`（mac 上 `Cmd+Shift+F`）：正文里没有半截拼音。
      - 中文输入法开着、没有组合时按它：记下聚焦搜索框还是被输入法切成繁体。三个浏览器各做一遍。
  - **M12 的移交**：`M12-release/handoffs/M4-performance.md` 加一项，接在 P4 的第 16 项之后（P5 先合并时后合并的一方改号）：
    - 3.9 的数。
    - 更大的语料与并发下的搜索。
    - 任务的吞吐、导入与升级时的积压。
    - 计数的误差。
    - 1 MB 的词的界线、编辑中的大页每 30 秒重建。
    - 繁简映射。
    - 搜索的并发上限。
  - B 的界面打磨项在 P5B 收尾时列出，随 v0.1 之后的那一次打磨做。
  - **M9 的移交**：新建 `M9-mcp/handoffs/M8-P5-search.md`（`status: open`，`from: M8/P5`，`to: M9`），见第 8 节。

## 8. 交给后面的 Phase

- **收尾**：00 号文档 4.12 的数以 A 的第 2 步为准；第 3 节第 6 条逐项对照 3.9。P5A 合并时照第 1、7 步写了。
- **M9**（`grep`、`index`，[M9 的移交](../M9-mcp/handoffs/M8-P5-search.md)）：
  - `grep` 用这个索引：
    - 从正则的字面片段取键：用 `regexp/syntax` 解析，取每段必须出现的字面串，CJK 2 个字起给二元组，字母 3 个起给三元组，大小写按标志。
    - 候选并上队列里准确的节点，在 SQL 里以正则复查，路径的 glob 再筛；`building` 时同样说明。
    - 取不出键的正则只能有界地扫：同 `MaxSearchWalk` 的走查，答"继续"。
    - 字母文字定为三元组（T，3.9）：字母与数字的任意子串（3 个起）都取得出键。
  - 队列让刚写的页立即可查，agent 写完即 grep 不会漏。agent 以 batch 大量新建时（导入以外的单元是准确的），看任务的吞吐是否跟得上。
  - MCP 的 `search`（若加）调用同一个 `Search` 用例。
- **M12**：第 7 节的移交（[性能移交](../M12-release/handoffs/M4-performance.md)第 17 项：3.9 的数、300 ms 的尾巴、更大的语料与并发、命中不按范围、按 NFC 读的代价、任务的吞吐与积压、重建的 WAL 与磁盘……）；搜索的并发上限与按工作区的预算；繁简映射（OpenCC 一类，在分词与复查两边）；NFKC、重音与完全折叠的复查；`TitleKey` 对切罗基文不幂等（P5A 审查 A-M1 的附带发现，[打磨移交](../M12-release/handoffs/M5-polish.md)第 25 条）。

## 9. 结果

### 9.1 A：服务端（2026-10-16，合并 `760757aa`）

- 提交：实施：搜索的表、分词与查询 `6f2785b0`，队列、任务与接口 `7277c41d`，候选按键读节点 `8ac758a2`，守卫与基准 `00a89fb4`，内置的配置带上搜索的间隔 `3aa867f0`，基准在每个阶段以访客读 `7f8f8514`，候选只带笔记本的条件连接 `dfafcf7a`（从 `main` 的 `2800db3b` 开始）；修复一轮先在 `2800db3b` 上以 cherry-pick 重建，`3aa867f0` 并进 `7277c41d`、让每个提交都通过 `TestBuiltInProfiles`，是 `83a5de6e`、`d49c407a`、`4d4b5718`、`47055a0e`、`e3de3566`、`0b2f2ba4`（树与 `dfafcf7a` 相同）；审查的修复：后台任务的节点以批量入队、任务先做准确的 `4c273998`，分词 2（别的文字的长段、`data:` 的末尾、切罗基文）`093511a7`，不是 NFC 的正文按 NFC 读 `5e079634`，被隔开的单字的词只在没有正文键的查询里只搜名称 `ff844811`，`page_search_hits` 的 `work_mem` `3921345e`，计数的取样与 `at_least` `c25babb7`，启动时丢掉留在运行中的行、一次运行只试一次 `d3f1f448`，任务在锁的连接上、连接池的预算 `77d49608`，摘要的边上的命中 `1b3c0fb7`，reindex 的入队失败 `0b1233d0`，`ANALYZE` 跨运行计数 `bd1b430b`，交错 3 `8905bf9d`，契约的走查界线与折叠相同的词 `ca97838b`，取样从走查之下接着走 `443712e0`，守卫的底与单独的一行 `f2c3638d`，NFC 的代价写进注释 `480c031f`；修复核对第一轮之后的修复二轮：计数只在第一页的走查没有跑时取样、页满时不算最后一个通过 `9c589f68`，开始不了的事务让运行失败 `9c4bb9e8`，契约（后台的恢复与撤销、摘要的 NFC、`at_least`、单独的情形）`a02f4bdf`，`config.yaml` 的注释 `1a60651e`，连接池的消息 `bef3a1d0`，00040 的注释 `2c480a68`，反引号结束 `data:` 串 `d89dd4eb`，测试 `47eb518c`，守卫的三行重量 `f1b5cb55`；修复核对第二轮之后的修复三轮：门槛数比例自己的通过 `14751b0f`，页满而只查了一个候选的测试 `766d6b91`，运行的错误带着这一批的失败 `11ec0391`，连接池的消息先说 worker 的数 `8023c188`，422 的用词含组合记号 `fdc09444`。合并 `760757aa`（第二个父提交 `fdc09444`）。
- 实测（3.9）：字母文字定为三元组（T；P 在常见的词上 2.5–3.7 秒）；`MaxSearchWalk` = 500，`MaxSearchCountScan` = 20,000，名称层不设界线，`fastupdate` 开，任务的写在自己的事务里以 `work_mem` 64 MB、待处理列表 16 MB，`MinSearchRebuild` = 30 秒。照写的语句在建索引与待处理列表满时整表扫描、逐行解压正文，5–40 秒：正文层的候选改经 `page_search_hits` 与数出的 `LIMIT`，第 7 步在四种连接里取只带笔记本条件的。产品在 30 万页上：三个阶段各 204 格里 3、9、14 格的 p99 过 300 ms（至多 719 ms，没有过 1 秒的），都是重的类：主持者接受、移交 M12；体积 2.46–2.55 倍；任务从空建 285–329 页/秒；保存多 0.3–0.5 ms，5 万个节点的删除多 149–426 ms。
- 审查：三位审查者（Opus）。A、B、C 各一条 Important：泰文、老挝文、高棉文、缅甸文的分句过了 64 个码位整段不进索引（高，A-I1；64 的界线只对 ASCII 的字母与数字）；停下的进程留在运行中的搜索任务挡住之后的每一次运行，直到 River 的救援（高，B-I1；启动时丢掉）；后台的恢复与撤销以准确入队，之后罕见词答一串空页（中高，C-I1、B-M3；改为批量）。中低 4 条：被隔开的单字的词只搜标题（A-M2）；长的 `data:` 串连之后的文字一起丢掉（A-M3）；不是 NFC 的正文哪种写法都找不到（A-M4）；短的走查的计数（C-M1）。其余是低（切罗基文的大小写；走查里一个都没通过时的"约 n0"；任务的设置的测试；WAL 与磁盘没写进文档；任务的连接不在池的预算里；守卫的时限没有余地；约 200 万个节点之后有损的位图）与细节。修复核对第一轮：审查的处置都成立，修复带来三条新的发现（计数的新取样在 30 万页上把 `参考文献` 的第一页推过 300 ms，I-1；运行的连接断了时逐个节点记"没写"，M-1；契约没写后台的恢复与撤销要等任务，M-2）与七条细节，修复二轮照主持者的决定改；第二轮：各条都改对了，一条低（页满停在第 2 个通过时 `about` 只依据一个通过，F-1，主持者定为 `at_least`）与四条细节，修复三轮照改；第三轮：没有剩下的行为上的发现，只记一条观察（页满停在第 3 个通过时偶尔答"1,000+"，O-1，移交 M12）。逐条见[审查记录](reviews/P5A-search-server-review.md)。
- 与设计不同之处已改在第 3 节（3.1–3.10），3.11 分实施时定下的与审查带来的列出；2.2 记下结果；第 8 节加移交的链接。
- 反向对照：实施之后 domain 33 个、app 39 个（一个挂到时限，也算抓到）、适配器 70 个、处理器 31 个、整个程序 12 个（4 个第一次存活，测试改为每次写之前先让任务跑过之后被抓到），都被抓到；审查者 B 1 个存活（任务的设置成了会话级的，即 B-M1），A、C 以探针代替；审查的修复之后 36 个：35 个被抓到，1 个等价；修复核对第一轮 16 个都被抓到；修复二轮 15 个都被抓到，核对者 10 个里 1 个存活（页满的比例的 `Checked > 1` 没有测试，修复三轮补上）；修复三轮 5 个都被抓到，核对者 8 个里 2 个等价。
- 门禁：本机 `dfafcf7a` 与 `480c031f` 上 `make lint-go gen-check`、`make lint-web knip test-web build-web`（3,240 个测试）、`make test-go`（788、833 秒）、两份组合根、`make e2e`（245 个）通过，修复一轮的 22 个提交每个都编过、vet、通过不要数据库的测试；CI：`3aa867f0`（run 38700649712）、`dfafcf7a`（run 38736007129）、`480c031f`（run 38762693235）全绿（3.9）；修复二轮、三轮的头 `f1b5cb55`、`fdc09444` 上同样的门禁通过（`make test-go` 868、899 秒；`fdc09444` 上第一次在负载 15 时 Docker 的 API 超时、一个包的数据库容器没起来，重跑通过），每个提交编过、vet、通过不要数据库的测试；CI：`f1b5cb55`（run 38814229551）、`fdc09444`（run 38823209418）全绿（3.9），最后一次竞态 7:22、组合根一 5:32、组合根二 7:46、成本 6:38、守卫 6:37、web 4:02、e2e 4:39、镜像 1:12。
- 交给后面的：B（`m8-p5b`）以这份契约为准（单独的情形；`exact` 含只在名称里找的，`at_least` 的三种；至多 64 个连续的 ASCII 字母与数字；走查至多 500 页的正文；折叠相同的词是一个；后台的恢复与撤销之后带 `building`）；M9：`grep` 用这个索引（[M9 的移交](../M9-mcp/handoffs/M8-P5-search.md)）；M12：[性能移交](../M12-release/handoffs/M4-performance.md)第 17 项（3.9 的数、300 ms 的尾巴、更大的语料与并发、命中不按范围与跨工作区的计时、按 NFC 读的代价、任务的吞吐与积压、重建的 WAL 与磁盘、日期的查询……），[打磨移交](../M12-release/handoffs/M5-polish.md)第 25 条（`TitleKey` 对切罗基文不幂等）。
- 负责人可以改判的取舍：300 ms 的尾巴接受（重的类）；后台的恢复与撤销以批量入队（其间找不到、带 `building`，直到任务建好）；按 NFC 读的代价（走查 500 页约 20–35 ms）为了对留着；启动时丢掉留在运行中的行（v0.1 只跑一个服务）；计数的取样只在第一页的走查没有跑时、页满时取 (k − 1) / (n − 1)，比例依据的通过少于 2 个时（页满停在第 2 个通过时也是）答 `at_least`。

### 9.2 B：前端

（P5B 合并之后写。）

## 附：查过的代码

行号是 `main` 的 `422abd35`（所引的代码与 `1f732c96` 相同）。网页的路径从 `web/apps/web/src/` 起。服务端的从 `server/internal/` 起，模块在 `modules/` 下：`page/…` 指 `server/internal/modules/page/…`，`bootstrap/…`、`platform/…`、`shared/…` 指 `server/internal/` 下的。spike 指 `scratchpad/m8/spike-search-report.md`。

| 决定 | 依据 |
|---|---|
| 负责人的决定与要改的 00 号文档（2.1） | `docs/v0.1/M8-history-search/00-M8-design.md:30`（第 1 节）、`:42-43`（第 2 节）、`:54`（第 3 节第 6 条）、`:66,77`（4.1）、`:211-223`（4.12）、`:233-234`（4.13）、`:249,251`（4.14）、`:269`（第 5 节）、`:289`（P5 一行）、`:297`（移交的落实）、`:308,311`（写给后面的 M）、`:337`（第 8 节）、`:347,359-362`（第 9 节）、`:368`（第 10 节）、`:392`（第 11 节第 10 条）；spike 第 3 节"What the product must decide"（:486-493）、B 的 Latin 语义（:195、:237）、短词整词（:479）、问题 5（:492） |
| 侧表、不放进 `page_contents`、不用表达式索引（3.2） | spike 2.1 的表（:123-135）、第 3 节第 7 条（:403-405）、2.3（:233，混在一起的 TOAST 6.2 GB 对 1.6 GB） |
| 派生的表不带外键、没有 `deleted_at`（3.2） | `bootstrap/purge_test.go:54-92`（`TestEverySoftDeletedTableHasAPurger`、`TestPurgersComeBeforeTheTablesTheyReference`）；`migrations/sql/00020_linking_indexed_pages.sql:1-4`；`migrations/sql/00017_page_edit_sessions.sql:1-4`（`edit_sessions`）；v0.1 设计 13.1 第 6 条"派生的索引表" |
| 行记下分词的版本、reindex 入队（3.2、3.4） | `linking/module.go:181`（`indexed_pages.extractor`，改了要求 reindex）；`linking/adapter/postgres/queries/pages.sql:7-8,71-72`；`shared/title_key.go:13-18`；`README.md:57,164`（reindex、没人写的时候执行） |
| 表的列与界线（1、3.3） | `migrations/sql/00012_page_nodes.sql:5-38`（`name` 1–255 字节、`kind`、索引）；`00013_page_page_contents.sql:5-21`；`00001_platform_pg_trgm.sql:5`；`page/domain/content.go:12-29`（5 MiB、UTF-8、NUL）；`shared/title.go:13`；审查 A 在 PG 18.6 上的 `tsvector` 界线（词的字节之和 1,048,575，超过时报错） |
| 正文的写不改 `nodes.updated_at`，排序取两者较晚的（3.5） | `page/adapter/postgres/queries/contents.sql:27-32`（`WriteContent` 只写 `page_contents.updated_at`）；`queries/nodes.sql:89,99,115,155`（改名、移动、删除、恢复写 `nodes.updated_at`） |
| 不在单元里写索引（3.4） | spike 2.4（:242-257：107 ms、1.3 秒、5–9 秒的停顿、每页的键 0.64–0.89 倍）、第 3 节第 6 条（:402）、局限（:482）；`page/app/unit.go:151-210`（`Run`：事务里 `do` 之后 `tell`）、`:222`（改动按第一次改到的次序）、`:393-404`（`tell`）；`linking/adapter/postgres/store.go:24,49-59` 与 `queries/pages.sql:1-4`（按笔记本的 advisory lock）；`web/…/editor/autosave.ts:4,18-21`（停 2 秒）；`transfer/app/import_write.go:25-31`（批的上限）；`02-P2-trash.md` 3.11（5 秒的规则） |
| 入队的情形、导入以批量（3.4） | `page/domain/change.go:30-45,59-73`（没有类型与正文、`Moves`、`Then`）；`page/app/unit_create.go:68`、`unit_create_asset.go:45`、`unit_content.go:61-75`（相同的正文不写）、`unit_rename.go:45`、`unit_move.go:73-76`（子孙前后相同）、`unit_delete.go:29`、`unit_restore.go:119-123`；`page/app/import_writes.go:76-79`（`Kind: ChangesetImport`）；`bootstrap/registrants.go:253-263`（观察者的次序） |
| 笔记本删除、reindex 与清理（3.4） | `page/deletion.go`、`page/app/extension.go:184-199`（`NotebookDeletion`）；`page/adapter/postgres/queries/nodes.sql:184-204`（`DeleteNotebooksPages`）；`bootstrap/registrants.go:187-201`（linking 的笔记本删除）；`cmd/nervewiki/reindex.go:16-39`；`bootstrap/reindex.go:32-81,139-157`（写死 linking）；`linking/app/reindex.go:82-131`；`page/adapter/postgres/queries/purge.sql`（清理只按 `deleted_at`）；`page/purgers.go:15-24` |
| 任务在 River 上、自己的队列、唯一与时限（3.4） | `page/adapter/river/cleanup.go:52-59`（`CleanupJob`：periodic、`RunOnStart`、没有唯一的选项）；`river@v0.47.0/insert_opts.go:176-212`（`ByState` 的默认含 `completed`；必须含 available、pending、running、scheduled）；`asset/adapter/river/sweep.go:20-25,51`（River 默认的 1 分钟、`Timeout`）；`platform/jobs/jobs.go:98-110`（队列、`RescueStuckJobsAfter`）；`transfer/adapter/postgres/queries/jobs.sql:12`（固定键的 advisory lock）；`bootstrap/deps.go:240`（队列表）；`bootstrap/transfer_jobs_test.go:262-268`；`bootstrap/app_test.go:52-100`（`testConfig` 不经校验）；v0.1 设计 13.1 第 23 条 |
| `ANALYZE` 与授权（3.2、3.4） | `transfer/app/import_write.go:393-399`（每 10,000 个节点）；`page/adapter/postgres/queries/statistics.sql:4`；`deploy/runtime-grants.sql:18-26`；`server/sqlc.yaml:93`（page 的迁移列表） |
| 分词与大小写（3.3） | spike `scripts/bench/tok.go:18-28`（规整、`isCJK`、`isWord` 是 L、N、Mn、64）；spike 第 3 节的分词（:459-474）、2.1（:135）、2.3 的 Latin 语义（:237-240）、局限（:479）；审查 A 的实测：PG 18.6 的 `~*`、`ILIKE` 与 RE2 `(?i)` 在 13 对特殊字符上有 8 对不一致，天城文的元音符号是 Mc，ARE 里 863 个区间的类 |
| 两层、惰性复查与走查（3.5） | spike 2.3（:205-211、:234，`f20lazy` 28–37 ms；:236，标题 0.5–2.2 ms；:201，"的"1.2–1.8 秒）、Plan 5（:370-378）、2.6 的标题在前与相似度（:300-301）、可见性（:299）；Plan 3（:336-344，`latency` 14,028 个候选、225 个真的）；审查 A 的 SearchBodies 实测（4 万页：无界的排序溢出到磁盘，改正之后 top-N heapsort、147 次按键读） |
| sqlc 的写法（3.4、3.5） | 审查 A 以仓库的 sqlc 在 server 的副本上生成；`page/adapter/postgres/queries/nodes.sql`（`RestoreNodes` 的 `SELECT unnest(…), unnest(…)`） |
| 计数（3.5） | spike 第 3 节第 3 条（:395-399）、2.6 的有上限的计数（:293）；`api/modules/linking.yaml:282-286`（"1000 is as many or more"）；`web/…/pages/notebook/changeset-text.ts:82-86` |
| 摘要在 SQL 里切、路径（3.5） | spike 2.6 的摘要（:302）；`page/adapter/postgres/queries/trash.sql:34-46`（`TrashLines`：按键读祖先） |
| 按参数规划（3.5） | `platform/postgres/planned.go:10-39`；`page/adapter/postgres/store.go:43-45`；v0.1 设计 13.4 第 9 条；`M12-release/handoffs/M4-performance.md:40-44`（第 14 项） |
| 可见性与端口（3.6、2.1 第 3 条） | `notebook/visible.go:16-43`；`notebook/adapter/postgres/queries/notebooks.sql:42-55`（没有 `LIMIT`，每本一个成员数的子查询）；`shared/notebook_role.go:44-68`；`workspace/workspaces.go:19-50`（`FindBySlug`）；`workspace/memberships.go:17-25`（没有 slug）；`notebook/app/authorize.go:27-39`、`list_notebooks.go:27-49`（角色取 `Grant`）；`page/app/ports.go:34-51`（`Workspaces` 只有 `ShareByID`）、`:345-347`；`page/module.go:96-150`（`Deps`、交空 panic）；`bootstrap/deps.go:142-162`；`bootstrap/events.go:24,56-58` |
| 码的次序与绑定（3.6、3.7） | `notebook/app/list_audit_events.go:53-101`（游标 → 判定 → `PageSize`）；v0.1 设计 13.1 第 4 条；`shared/cursor.go:21-90`（`PageSize` 的 `out_of_range`）；`api/common.yaml:10-33`；`api/modules/linking.yaml:182-188`（`target` 不标 `required`）；`platform/httpserver/apierrors.go:100-121,137-153`（绑定的 400）；`bootstrap/contract_test.go:247-272`（NUL 与 `\xff`）；`platform/httpserver/apitest/operations.go:49-68,77-103,113-131` |
| 权限（3.8） | `access/domain/rules.go:36-55,60-137`；`access/app/authorizer.go:27-46`；`access/domain/decide.go:47-55`；`shared/authorize.go:11-23,31-62`；`bootstrap/actions_test.go:20-26`；`page/domain/actions.go:8-43` |
| 矩阵、种子与"列表等于逐项判定"（3.8） | `bootstrap/permission_matrix_test.go:78-90,158-176,216-220`；`permission_matrix_notebook_test.go:42-81,182-214`；`permission_matrix_coverage_test.go:125-156`（查询参数不核对）；`notebook_visibility_test.go:29-80,87-136`；`page_visibility_test.go:27-152`（`:43-55` 以 SQL 加节点，`:66` 按名称找附件）；`permission_matrix_seeded_test.go:545-673`（先起应用再以 SQL 种下）；v0.1 设计 13.1 第 3 条（"M8 的搜索同样"） |
| 日志（3.6、3.8） | `platform/httpserver/middleware.go:151-196`（`loggedPath`、`webPath` 不读查询串）；`apierrors.go:101-122,202-208`（`Write` 整个记下错误）；v0.1 设计 13.1 第 10 条 |
| 附件（3.5、3.7） | `asset/attachment_infos.go:26-32`；`asset/adapter/postgres/queries/infos.sql:1-4`；`bootstrap/assets.go:136-150`；`api/modules/page.yaml:1405-1424`（`TrashAttachment` 只在回收站的单项里引用）；`page/adapter/http/trash.go:81,87` |
| 契约（3.7） | `api/openapi.yaml:17-36`；`apitest/rules_test.go:42-54,95,109-128`；`api/modules/page.yaml:983-1030`（没有 slug 参数）、`:1056-1061`（`NodeKind`）、`:1086-1090`（`TreeNode.updated_at` 不含正文的写）、`:1101-1111`（`Ancestor`）、`:667-697`（`listTrash` 的说明）；`api/modules/notebook.yaml:356-364`（`Slug`）；`api/common.yaml:173-185` |
| 交错、不变式与重建（3.8、3.10） | `bootstrap/interleavings_page_test.go:43-120`（`checkPages`）；`interleavings_trash_test.go:28-80`（`holdNotebook`、`waitingFor`）；`interleavings_assets_test.go:192,233`（`purgeBefore`、`holdRow` 持 `asset_blobs`）；`interleavings_import_test.go:281-297`（`betweenUnits`）；`bootstrap/links_rebuild_test.go`（`TestTheIndexIsItsRebuild`）；v0.1 设计 13.4 第 4、10 条 |
| 计划与成本测试、CI（3.9、3.10） | `page/adapter/postgres/history_plans_test.go:176-234`（`planOf`，关掉整表扫描）；`trash_cost_test.go:147-227`（三种统计）；`Makefile:178-211`；`.github/workflows/ci.yml:26-39,72`；`M12-release/handoffs/M4-performance.md` 第 14、15 项（`422abd35`：守卫是本机的三倍、CI 约慢一倍、三份的时间）；`04-P4-background.md:750-752`（组合根那一份、M12 第 16 项）；v0.1 设计 13.4 第 8 条 |
| 与 P4A 的重叠（第 0 节） | `git diff main...m8-p4a --stat`（`page/app/unit.go`、`ports.go`、`module.go`、`adapter/postgres/store.go`、`adapter/http/trash.go`、`app/fakes_test.go`、`api/modules/page.yaml`、`bootstrap/deps.go`、`migrations/schema_test.go`、`sqlc.yaml`、生成物） |
| 实测的工具与语料（3.9） | spike 第 1 节（环境、语料、查询）、第 4 节（重跑的步骤、留下的文件）；`scratchpad/m8/spike/scripts/`（`build_pages.py`、`load.sh`、`run_read_all.sh`、`write_bench.sh`、`lock.sh`）、`bench/main.go`（`read`、`write`、`tok`）、`queries/queries.json`、`data/corpus/meta.tsv` |
| 搜索框的位置（4.2） | `app/layout.tsx:30-55`（顶栏在 `:slug` 之上）；`pages/workspace/workspace-layout.tsx:72-135`（左栏，`:107-127`）；`app/layout.test.tsx:21-43`；`v0.1-design.md:809-818`（9.2）；`00-M8-design.md:233` |
| 快捷键与焦点（4.2） | `app/shortcuts.ts:5-26`（`isMod` 拒绝 Shift、`dialogOpen`）；`app/shortcuts.test.ts:19-45`；`pages/notebook/quick-switch.tsx:34-49`（`window` 上听、确认框开着时也 `preventDefault`）、`:117`（`arrived`）；`editor/source-editor.tsx:235-243`（没有 `Mod-Shift-F`）；`reviews/M8-design-review.md:53`（输入法的简繁切换、Firefox 的全屏）；v0.1 设计 13.2 第 12、17、20 条 |
| 组合中的回车（4.2） | `reading/unresolved-links.ts:43`（唯一的 `isComposing`）；`pages/notebook/quick-switch.tsx:121-133`、`app/confirm-dialog.tsx:150-154`（没有防）；`docs/v0.1/M4-pages/manual/P6-ime-checklist.md:3-9`（表头）、第 1–20 步 |
| 路由、读、标题与列表的模式（4.3） | `app/routes.tsx:127-274`；`app/document-title.ts:12-20`；`lib/read-pages.ts:13-23`；`pages/load-more.tsx:26-50,65-172`（`whole`：`:47,127`）；`pages/page/version-page.tsx:31-37`（`unchanging`）；`app/providers.tsx:13-20`（SWR 的默认，缓存在一次登录里）；`app/event-stream.tsx:15-35`；`events/handlers.ts:39,62-123,272-276`；`pages/notebook/history-page.tsx:96-109,177,189`（播报与列表名） |
| 附件的去处、同名与图标（4.3） | `pages/page/attachment-row.tsx:17-44,95`（`Located`，注释点名 P5）；`pages/notebook/trash-row.tsx:56-61,93-106,161-190`（`openOf`、`namesOf`、`ItemIcon`）；`pages/notebook/trash-page.tsx:112`（`openOf` 只有回收站用）；`pages/page/attachments-section.tsx:354-397`（`useLocated`）；`app/arrival.ts:11,22-31`；`lib/twins.ts:8-20` |
| 高亮与文字（4.3、4.4） | `reading/reading.css:25,58-61,150`（`--nw-mark-background` 只在 `.nw-reading`）；`i18n/i18n.tsx:9-32`（没有复数）；`i18n/format.ts:56-71`；`i18n/messages/zh-CN.ts:269`、`en.ts:276`（"1,000 多"、"+"） |
| 假接口与 e2e（4.5、4.6） | `test/fakes.ts:164-204`；`test/page-server.ts:149-361`；`test/render.tsx:30-55`；`e2e/fixtures/test.ts:111-258`；`fixtures/invitations.ts:106-134`；`fixtures/workspaces.ts:84-97`；`fixtures/wiki-pages.ts:181-191`；`e2e/stories/trash/tb1-trash.spec.ts:34,222,297`（两个版本）；v0.1 设计 10.1、13.4 第 3 条 |
