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

## 3. A：服务端

### 3.1 文件

| 文件 | 内容 |
|---|---|
| `server/migrations/sql/00040_page_search.sql` | `page_search`、`page_search_queue`、序列与索引，已有节点以批量入队（3.2） |
| `deploy/runtime-grants.sql`、`server/sqlc.yaml` | 两张表的 DML、序列的 `USAGE`，`page_search` 的 `MAINTAIN`（任务的 `ANALYZE`）；page 的迁移列表 |
| `page/domain/search.go` | 分词 `SearchTokens(name, content)`（标题、正文两份）与版本 `SearchTokenizer`；`ParseQuery`（`Query`、`Term`：键、只搜标题、两种写法的模式）；界线：`MaxSearchQueryBytes` = 256、`MaxSearchTerms` = 8、`DefaultSearchLimit` = 20、`MaxSearchLimit` = 50、`MaxSearchWalk`、`MaxSearchCount` = 1,000、`MaxSearchCountScan`、`SearchCountSample` = 100、`MaxSearchLexemeBytes`、`MaxSearchWordRunes` = 64、`MinSearchRebuild` = 30 秒（3.3–3.5）；`SearchLimit(limit)` |
| `page/domain/search_hits.go` | 高亮：在名称与摘要里按词的规则找命中，答排好、不相交的 UTF-16 区间；摘要的上下文字符、空白合并与截断；`SearchCursor{Tier, At, ID}`；计数的估计 `EstimateCount` |
| `page/domain/errors.go`、`actions.go` | `ErrWorkspaceNotFound`（码 `workspace.not_found`）；`ActionSearch = "page.search"` |
| `page/app/ports.go` | `Workspaces.FindBySlug`；`VisibleNotebooks`；`SearchIndex`（入队、取锁、队列、来源、写行、出队、`ANALYZE`）；`SearchReads`（一个只读事务里的两层、计数、`building`、路径）；`AttachmentInfos` 已有 |
| `page/app/unit.go` | `Run` 在 `do` 之后、`tell` 之前 `queueSearch`（3.4） |
| `page/app/extension.go`（`NotebookDeletion.NotebookDeleted`） | 这次删除的节点入队 |
| `page/app/index_search.go` | `IndexSearch.Drain`：任务的一次运行（3.4） |
| `page/app/search.go` | 用例 `Search`（3.5、3.6） |
| `page/adapter/postgres/queries/search.sql`、`search.go` | 队列、任务、两层、计数与 `building` 的语句；搜索的语句经 `planned(ctx)`；任何错误只留 SQLSTATE |
| `page/adapter/http/search.go`、`handler.go`；`trash.go` | `searchWorkspace` 的处理器；`gen.TrashAttachment` 改名（:81、:87） |
| `page/adapter/river/search.go` | `SearchIndexJob`：kind `page.index_search`，队列 `page_search`（1 个 worker），唯一（不含 `completed`），时限 2 分钟，每隔 `page.search_index_interval`，启动时一次 |
| `page/module.go`、`page/search_index.go` | `Deps.VisibleNotebooks`、`Deps.SearchInterval`（0 时不注册任务，只在测试里）；`Jobs()` 加任务；`QueueSearch` 常量；只凭连接池的 `page.NewSearchIndex(pool)`（reindex 的 `Requeue`，测试的 `Drain`） |
| `access/domain/rules.go` | `page.search`：工作区级，每种角色 |
| `platform/config`、`configs/config.yaml` | `page.search_index_interval`（默认 5 秒，1 秒到 10 分钟） |
| `bootstrap/deps.go`、`reindex.go`、`registrants.go` | 接线：`VisibleNotebooks`、`Queues` 加 `page_search: 1`；reindex 在 linking 之后让这本笔记本里没有行或行的分词版本不同的节点以批量入队 |
| `api/modules/page.yaml`、`api/openapi.yaml` | `searchWorkspace` 与结构；`TrashAttachment` 改名 `AttachmentInfo`（同形，搜索也用） |
| `bootstrap/search_test.go`、`search_rebuild_test.go`、`interleavings_search_test.go`、`page_visibility_test.go`、`permission_matrix_page_test.go`、`permission_matrix_seeded_test.go`、`search_bench_test.go`；`page/domain/search_test.go`；`page/adapter/postgres/search_test.go`、`search_plans_test.go`、`search_cost_test.go`；`migrations/search_test.go`；`Makefile` | 3.10、3.9 |

### 3.2 迁移

**`00040_page_search.sql`**（归 page，13.1 第 7 条）：

```sql
-- +goose Up
-- A node's search lexemes (M8/P5 design 3.2): computed by the app (domain.SearchTokens) from the node's name and a
-- page's content as the search's job last read them. A derived table as linking's are: no foreign key, no
-- deleted_at, no purger (v0.1 design 13.1, item 6). The job keeps a row for each live node and removes a node's
-- row once the node is deleted or gone; reads join the live nodes, so a stale row is never an answer.
CREATE TABLE page_search (
    node_id   uuid PRIMARY KEY,
    -- The page's content revision the body was built from; null for an attachment, which has no body.
    revision  integer CHECK (revision >= 1),
    -- The name the lexemes were built from.
    name      text NOT NULL,
    -- The tokenizer the lexemes are of (domain.SearchTokenizer: its own version and Go's Unicode version): a row of
    -- another is rebuilt once nervewiki reindex queues it, as linking's extractor.
    tokenizer text NOT NULL,
    -- When the job last built the row: it builds it again no sooner than domain.MinSearchRebuild after.
    built_at  timestamptz NOT NULL,
    -- The name's: CJK unigrams and bigrams, the word runs' trigrams (or words), the short words.
    title     tsvector NOT NULL,
    -- A page's name and content's: CJK bigrams, the word runs' trigrams (or words), the short words.
    body      tsvector,
    CONSTRAINT page_search_body_check CHECK ((revision IS NULL) = (body IS NULL))
);
CREATE INDEX page_search_title_idx ON page_search USING gin (title);
CREATE INDEX page_search_body_idx ON page_search USING gin (body);

-- The nodes whose row may be stale (M8/P5 design 3.4): a unit queues each node it creates, restores, renames,
-- deletes or writes the content of, in its transaction; the job takes them off. Each queueing takes a new version
-- from the sequence, so the job takes a node off only as it read it. A bulk node (an import's, the backfill's,
-- nervewiki reindex's) is no candidate of a search, which says the index is being built; a user's write makes
-- it exact.
CREATE SEQUENCE page_search_queue_version_seq;
CREATE TABLE page_search_queue (
    node_id     uuid PRIMARY KEY,
    -- The node's notebook, which never changes: a search reads its notebooks' queue only.
    notebook_id uuid NOT NULL,
    version     bigint NOT NULL DEFAULT nextval('page_search_queue_version_seq'),
    bulk        boolean NOT NULL
);
CREATE INDEX page_search_queue_notebook_idx ON page_search_queue (notebook_id, bulk);
-- The live nodes so far, in bulk: the job builds their rows once the server runs.
INSERT INTO page_search_queue (node_id, notebook_id, bulk)
SELECT id, notebook_id, true FROM nodes WHERE deleted_at IS NULL ORDER BY id;

-- +goose Down
DROP TABLE page_search_queue, page_search;
DROP SEQUENCE page_search_queue_version_seq;
```

- **侧表，不是 `page_contents` 的列**：
  - spike 的 B 把词放在 `page_contents` 里，正文与词的 TOAST 混在一起，6.2 GB 对 1.6 GB，正文的读都跟着变慢（spike 3 第 7 条 (a)）。
  - 分词只有一份，在 Go 里；表达式索引要另写一份 SQL 的分词，不走索引的计划会逐行分词。
- **`page_search` 不建 `notebook_id`、`kind`**：读时连 `nodes`，活着、可见、类型、名称都以它为准。队列的行带 `notebook_id`：搜索只读它的笔记本的队列，一个工作区的积压不拖慢别的工作区。
- **版本号取自序列**：重新入队也取新的，同一个 `(节点, 版本)` 不会再出现（没有 ABA）。版本号的更新不改索引的列，是 HOT 的；`bulk` 至多由真变假一次。
- **`tsvector` 不带位置**，由 `array_to_tsvector(@lexemes::text[])` 写。一行的词的字节之和至多 `MaxSearchLexemeBytes`（3.3）。
- **GIN 的 `fastupdate` 用默认（开）**：
  - 只有任务写这两个索引，待处理列表的合并由任务付，请求从不碰它。
  - 读多扫至多 4 MB 的待处理列表（`gin_pending_list_limit` 的默认）。
  - 开与关由 3.9 实测定。
- **授权**：两张表的 DML、序列的 `USAGE`；`page_search` 加 `MAINTAIN`，任务大量写之后 `ANALYZE`，同导入。
- **迁移测试**：
  - `schema_test.go` 核对表、约束、序列与索引的名字。
  - `TestChecksRejectCounterexamples` 加反例：附件带 `body`、页没有 `body`。
  - Up 带数据跑：活着的节点以批量入队、版本各不相同，已删的不入。
  - Down 带数据跑一次。

### 3.3 分词：页的词与查询的词

**规整与字符的类**（`domain/search.go`，照 spike 的 `tok.go`）：

- 词用 `NFC(Fold(NFC(s)))`，同 `shared.TitleKey`。复查与高亮看原样的文字（下）。
- **CJK**：`unicode.Han`、`Hiragana`、`Katakana`、`Hangul`、`Bopomofo`，以及 `ー`、`ｰ`、`〆`。
- **字母文字**：字母（L）、数字（N）与组合记号（Mn、Mc、Me），不是 CJK 的。拉丁、西里尔、希腊、泰文、天城文等都算（天城文的元音符号是 Mc，算作分隔会把"हिन्दी"切开）。
- 其余（空白、标点、符号、emoji）是分隔。
- 一段连续的 CJK 是一个 CJK 段，一段连续的字母文字是一个字母段。段的长短（1–2 个、3 个起、64 的界线）按完全折叠之后的字符数算，模式的边界用同一个分类。
- **不进索引**：
  - 超过 `MaxSearchWordRunes` = 64 个的字母段（base64 一类的长串）。
  - 不含空白、以 `data:` 开头、长过 64 个字符的串（Markdown 里内嵌的图片）：它的 base64 切成很多短段，会把一页的词的预算用在没有意义的串上。

**页的词**（`SearchTokens`），同一个函数给任务用，也给测试的重建用：

| | 标题（`title`：名称） | 正文（`body`：名称 + 换行 + 正文，只有页） |
|---|---|---|
| CJK 段 | 每个字（单字）与相邻两字（二元组） | 二元组 |
| 字母段，3 个及以上 | T：每三个相邻的（三元组）；P：整个字母段 | 同标题 |
| 字母段，1–2 个 | 整个字母段 | 同标题 |

- 正文里不收单字：负责人的第一个决定。spike 量过，"的"在正文里 1.2–1.8 秒，去掉它还省下 0.93 + 0.24 GB。
- 正文里也收名称的词，所以"每个词在名称或正文里"一次 `@@` 就能找出候选（3.5）。
- 词按第一次出现的次序收，字节之和到 `MaxSearchLexemeBytes` = 1,000,000（PostgreSQL 的界线是 1,048,575，超过时报错）为止，其余丢掉。
  - 约 16 万个不同的二元组，约 2 MB 以上的中文才到；正文至多 5 MiB，所以这是已知的局限，写进 4.12 与第 10 节，3.9 记下语料里到界线的页数。
- `SearchTokenizer` 是分词的版本加 Go 的 Unicode 版本（如 `1/15.0.0`），写进每一行。分词或 Go 的 Unicode 数据变了时，发布说明要求执行 `nervewiki reindex`，同标题键与 linking 的提取规则。
- 代价：spike 的 Go 分词每 10 KB 0.24 ms、200 KB 4.3 ms（P 的写法）；T 多三元组，3.9 实测。

**查询**（`ParseQuery(q)`，与资源无关，在找工作区之前，3.6）：

- **取值**：去掉首尾空白之后 1–256 字节；是合法的 UTF-8、没有 NUL；按 Unicode 空白（含全角空格 U+3000）切成词，按规整之后的形式去重，至多 8 个；词里连续的字母至多 64 个（更长的串不进索引，永远找不到）。
- **错误**：不合时答 422 `validation_failed`，字段 `q`：缺了或为空是 `required`，太长、太多词或字母段超过 64 个是 `too_long`，坏的 UTF-8 或 NUL 是 `invalid_format`，全部的词都没有键是 `not_allowed`。
- **每个词分成段**，得到它的键与模式：
  - **键**：
    - 2 个及以上字的 CJK 段给二元组。
    - 3 个及以上字母的段，T 给三元组；P 给整段，词末的一段作前缀（`'linux':*`）。
    - 1–2 个字母的段给整段。
    - 只有一个字的 CJK 段给单字，只作标题的键。
  - **只搜标题的词**：有键的段只有一个、是一个汉字（"锅"、"锅！"）。别的词里的单字段只给标题的键：`A股` 在正文层以 `a` 找候选，在标题层以 `a` 与 `股`。
  - **没有键的词**（只有标点或符号，如 "#"）照样复查，只是不进索引。全部的词都没有键时答 422。
  - **模式**：词的 NFC 原文，外面一个捕获组。
    - **大小写在模式里**：每个有大小写的字符写成它的 `unicode.SimpleFold` 轨道的方括号（`ß` → `[ßẞ]`，`σ` → `[σςΣ]`，`k` → `[kKK]`），别的字符转义。PostgreSQL 用区分大小写的 `~` 与不带 `i` 的 `regexp_instr`，RE2 不带 `(?i)`：两边拿到同一个不用再折叠的模式，按构造一致，也不依赖 PostgreSQL 的 Unicode 版本。
    - T：开头是 1–2 个字母的段时，左边要有边界；结尾是这样的段时，右边要有边界。
    - P：开头的字母段都要左边界，结尾 1–2 个字母的段要右边界。
    - 边界是文字的开头、结尾，或一个不是字母文字的字符（CJK 算边界）。
  - **两种写法由同一个构造器给出**：PostgreSQL 的 ARE，给 `~` 与 `regexp_instr`；Go 的 RE2，给高亮。CJK 与字母文字的范围由 Go 的 `unicode` 表生成成方括号里的区间（审查 A 核对过 863 个区间的类在 ARE 里能编译，在 110 万字符上多约 1 ms）。
- **`tsquery`**（文字由 Go 写，词里只有字母、数字与组合记号，没有要引的字符；以 `::text::tsquery` 传）：
  - 标题层：全部词的标题键相与。
  - 正文层：不是只搜标题的词的正文键相与（`body_keys`），加上只搜标题的词的单字在标题里（`title_keys`）。两者至少有一个；没有只搜标题以外的词时这一层不跑。
  - 所以 "锅 #" 的正文层候选是标题有"锅"的页，再复查 "#" 在名称或正文里。

**语义**（写进 4.12 与契约的说明）：

| 词 | 例 | 在哪里找 | 怎样算命中 |
|---|---|---|---|
| 单个汉字 | 锅 | 只在名称 | 子串 |
| 2 个及以上汉字 | 中国、数据库 | 名称或正文 | 子串 |
| 3 个及以上字母或数字 | api、latency、2020 | 名称或正文 | T：子串（api 找到 rapid）；P：词的开头 |
| 1–2 个字母或数字 | ai、go、C、20 | 名称或正文 | 整词（`Go语言` 的 Go 是整词：汉字算边界；`20` 找不到 `2020年`） |
| 混合 | Go语言、C++、node.js、A股 | 名称或正文 | 整个词作子串，两端 1–2 个字母的段另要边界 |
| 几个词 | 中国 历史 | 每个词各在名称或正文里 | 全部命中 |

- `%`、`_`、`\` 与正则的元字符照字面匹配（转义）。
- 大小写不敏感是对称的：同一个轨道的字符互相找得到（"ΛΟΓΟΣ" 找到 "λογος"，"straße" 找到 "STRAẞE"）。完全折叠的展开不互通："strasse" 找不到 "straße"。正文里不是 NFC 的组合字符（少见）只有同样的写法找得到。全角与半角、繁简、重音不互通。
- **键与复查**：命中的定义是"键在索引里，且复查通过"。同一轨道的字符完全折叠相同，所以复查通过时查询的键都在这一页的词里，例外只有不进索引的部分：超过 64 个字母的段、长的 `data:` 串、超出 `MaxSearchLexemeBytes` 的词。这些在节点排队时由复查找到，建好之后找不到；写进 4.12 与第 10 节，性质测试在例外之外核对（3.10）。

### 3.4 索引的维护：队列与后台的任务

**为什么不在单元里写 `page_search`**（spike 2.4 的数，B 的写法没量过、只能按每页的键比）：

- **每次自动保存**（停 2 秒就是一次）：GIN 每次插入一页的全部键。pg_trgm 的数是 `fastupdate = off` 时 10 KB 107 ms、200 KB 1.3 秒；`on` 时有 5–9 秒的停顿。这段时间持着正文行。而且观察者在 linking 的按笔记本的锁之后，这本笔记本的保存都会排在它后面。
- **导入的一批**（100 页、8 MiB）：照上面的数一批要十几到几十秒，其间持着笔记本行 `FOR NO KEY UPDATE`，挡住这本笔记本的保存。单元"满负载 5 秒"的规则（[02-P2](02-P2-trash.md) 3.11）。
- **界线处的恢复与撤销**同样。

所以单元只把节点放进队列，一条语句，约是写一行；行由后台的任务重建。

**准确与批量**：

- **准确**（`bulk = false`）：用户的写，即导入以外的单元，与笔记本删除。搜索把它们当作候选、对当前的状态复查，所以写了立即反映。
  - 用户的写受请求的速率所限，任务的吞吐（3.9 的目标）高于它；同步的恢复一次至多 100 页（P2 的页数界线），后台的恢复与撤销按批到来。所以准确的积压只有几十到几百页。
- **批量**（`bulk = true`）：导入的单元、迁移的补入、`nervewiki reindex`。它们一次可以有几万到几十万个节点：当作候选时，正文层的走查全花在它们上面，每次搜索答一串空的"继续查找"（审查 A-I2）。所以它们不当候选，搜索的答复带 `building`，结果页写明"索引正在建立，结果可能不全"，任务建好之后消失。
- 用户的写让批量的行变成准确的（`bulk` 取两者的与）。

**入队**（`Unit.queueSearch`，`Run` 在 `do` 之后、`tell` 之前，同一事务）：

- 从合并的改动取节点，以下情形入队：
  - 新建：没有"前"，含恢复。
  - 删除：没有"后"。
  - 改名：前后名称不同。
  - 写了正文：`Revision` 不为零。
- 移动与重新排序不入队：路径不参与匹配。
- 导入的单元（`Kind` 为 `import`）以批量入队，别的以准确入队。

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
- **`nervewiki reindex`**：每本笔记本在 linking 重建之后（`page.NewSearchIndex(pool).Requeue`）：
  - 按 id 每批读 1 万个"活着、没有行或行的 `tokenizer` 不是当前的"节点，各一个事务以批量入队（同一条 `QueueSearch`）。
  - 旧的行在重建之前照常回答；当前的行不入队，所以只为 linking 执行的 reindex 不让搜索重建。
  - 命令只入队，由服务的任务重建，CLI 打印入队的数目。
- **清理不碰这两张表**：节点被删时任务已删掉它的行；节点在队列里时被清理了，任务读不到它，删掉行。

**任务**（`app.IndexSearch.Drain`；River 的 `page.index_search`，`adapter/river/search.go`）：

- **调度**：
  - 每隔 `page.search_index_interval`（默认 5 秒）一次，服务启动时一次。
  - `InsertOpts{Queue: "page_search", UniqueOpts: {ByState: available、pending、running、retryable、scheduled}}`：排队与运行中至多一个；不含 `completed`，否则完成的那个挡住之后 24 小时的定时插入（River 0.47 的默认）。
  - 在自己的队列 `page_search` 上，一个 worker：不占默认队列的两个 worker。
  - 时限 2 分钟（River 默认的 1 分钟会截断最后一批），远小于 `RescueAfter`。
  - `Deps.SearchInterval` 为 0 时不注册任务：只在测试里（`testConfig` 不经校验），测试经 `page.NewSearchIndex(pool).Drain` 自己跑（3.10）。它取同一把锁，但不等 `MinSearchRebuild`：测试要做空队列；最短间隔由 `page/app` 的测试以假的时钟核对。
- **同时只有一个**：`Drain` 在一个专用的连接上 `pg_try_advisory_lock`（固定的键，同 transfer），拿不到就返回，做完放开；进程退出时连接断开，锁随之放开。两个同时跑会丢更新（审查 A-M4：较早读的那个在较新的之后写回旧的行，节点却已出队），River 的救援、测试与命令里的 `Drain`、第二个进程都可能让它发生。
- **一次运行**按 `node_id` 走一遍队列，每个节点至多读一次，做到队列的末尾或 1 分钟之后不再开始新的一批：
  1. `QueuedSearch`：`node_id > @after ORDER BY node_id LIMIT 100` 的 `(node_id, version)`，不加锁。
  2. `SearchSources`：按键读这些节点的类型、名称、是否已删、正文的版本号与字节数，以及它们现在的行的 `revision`、`name`、`tokenizer`、`built_at`。要重建的页的正文另一条语句读，一批至多 8 MiB（同导入的批）；超出的留到下一批，`@after` 只前进到做完的最后一个。
  3. 在 Go 里逐个判：
     - 节点不在或已删：删它的行。
     - 行的 `revision`、`name`、`tokenizer` 都是现在的：不动。
     - 行在 `MinSearchRebuild` = 30 秒之内建过：跳过，留在队列里（仍是候选，结果照样准确）。一页编辑中每 2 秒保存一次时，至多每 30 秒重建一次（审查 A-M9）；没有行时立即建。
     - 否则分词、写行。
  4. **事务一**：这一批的写与删，按 `node_id` 的次序（每行一条 `INSERT … ON CONFLICT (node_id) DO UPDATE`，`array_to_tsvector`，经 pgx 的 batch），提交。
     - 失败时这一批逐个节点各一个事务再写一次。还失败的节点记下 id 与 SQLSTATE、这次运行跳过它：它留在队列里（仍是候选），下一次运行再单独试一次。一个写不进的节点不挡住整个队列。
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
  6. 一次运行写满 10,000 行就 `ANALYZE page_search`，同导入的节拍：`@@` 的选择度靠它的统计。
- **为什么对**（只有一个任务时）：
  - 任务写的总是它读队列之后读到的节点的状态。之后的写让版本号变大，出队的条件不成立，节点留在队列里。
  - 等锁之后 PostgreSQL 按新的行版本重判 `version` 的条件，同理。
  - 第 4、5 步之间中断：行写了、节点还在队列里，下次读到它与行相符，直接出队。
  - 所以任何时候：每个活着的节点有 `revision`、`name` 是现在的行，或在队列里（`checkSearch` 不做空队列也核对它，3.10）。
- **不等别人**：
  - 事务一只锁 `page_search` 的行，只有这一个任务写这张表。
  - 事务二 `SKIP LOCKED`。
  - 单元至多等事务二删它那个节点的一行。
- **导入**：每批让它的至多 100 个节点以批量入队，任务随后跟上；期间这本笔记本的搜索带 `building`。3.9 量任务的吞吐、导入 1 万与 5 万页之后清空的时间。
- **升级**：00040 让全部活着的节点以批量入队；任务建完之前（30 万页的目标是 1 小时）每次搜索都带 `building`，只找得到之后写过的。v0.1 还没有发布过，只影响开发的库。

### 3.5 查询：两层、复查、游标、计数与摘要

**候选**：索引的命中并上队列里准确的节点（新写的、恢复的、改过名的，它们的行可能还没重建），连活着的节点、限看得到的笔记本：

```sql
SELECT s.node_id FROM page_search s WHERE /* this tier's keys */
UNION
SELECT q.node_id FROM page_search_queue q WHERE NOT q.bulk AND q.notebook_id = ANY(sqlc.arg(notebooks)::uuid[])
```

- 队列里行已是现在的节点（只在任务的两个事务之间一瞬）当候选也只是多复查一次，不另比行。
- **排序**：按 `sort_at = greatest(n.updated_at, c.updated_at)`（最后一次改动：正文、名称或位置）与 id 倒序；附件没有正文，`greatest` 略过空值。

**两层**：

- **名称层**（第 0 层）：每个词都匹配名称的活着的节点，页与附件都在。
  - 名称在排序与 `LIMIT` 之前检查（`n.name ~ ALL(@terms)`）：名称至多 255 字节、在 `nodes` 里、不用解压，审查 A 估 1 万个约 5–10 ms。这一层不走查，只取 `limit + 1` 个；页的正文只为答出的行按键读，给摘要。
- **正文层**（第 1 层）：活着的页，满足三条：
  - 不是每个词都匹配名称（那些已在第 0 层）：`NOT n.name ~ ALL(@terms)`，在 `LIMIT` 之前。
  - 只搜标题的词匹配名称：`n.name ~ ALL(@titled)`，在 `LIMIT` 之前。
  - 别的词各匹配名称或正文：走查时对正文复查。
- **走查**：正文层至多走查 `MaxSearchWalk` 个页的正文，直到 `limit + 1` 个通过。走查不排序正文；正文按键只为走查的行读一次，拼接与复查各算一次（审查 A 在 PG 18.6 上核对过这个写法：top-N heapsort、按 `page_contents_pkey` 读走查的行、顶上没有排序）。

```sql
-- name: SearchBodies :many
-- The body tier of a search (M8/P5 design 3.5): the live pages of the notebooks given whose lexemes hold the keys,
-- or that wait in the queue as a user wrote them, below the cursor, newest first, whose name misses some term and
-- holds each titles-only one; walk of them at most have their content read, by key, and checked as they are now,
-- until max_rows pass. The sort carries no content; OFFSET 0 keeps each lateral one subquery, so the concatenation
-- and the check run once a row. The last row walked comes back passing or not, with walked: more than walk is
-- more to walk. Planned with its arguments (postgres.Planned): the keys and the notebooks decide the plan.
SELECT w.id, w.notebook_id, w.name, w.sort_at, w.seq::integer AS seq, w.walked::integer AS walked, x.passed,
    e.at::integer AS at, substr(b.doc, greatest(e.at - 41, 1), 162)::text AS excerpt, length(b.doc)::integer AS chars
FROM (
    SELECT v.*, row_number() OVER (ORDER BY v.sort_at DESC, v.id DESC) AS seq, count(*) OVER () AS walked
    FROM (
        SELECT n.id, n.notebook_id, n.name, greatest(n.updated_at, c.updated_at)::timestamptz AS sort_at
        FROM (SELECT s.node_id FROM page_search s
              WHERE (sqlc.narg(body_keys)::text IS NULL OR s.body @@ sqlc.narg(body_keys)::text::tsquery)
                  AND (sqlc.narg(title_keys)::text IS NULL OR s.title @@ sqlc.narg(title_keys)::text::tsquery)
              UNION
              SELECT q.node_id FROM page_search_queue q
              WHERE NOT q.bulk AND q.notebook_id = ANY(sqlc.arg(notebooks)::uuid[])) k
        JOIN nodes n ON n.id = k.node_id
        JOIN page_contents c ON c.node_id = n.id
        WHERE n.deleted_at IS NULL AND c.deleted_at IS NULL
            AND n.notebook_id = ANY(sqlc.arg(notebooks)::uuid[])
            AND (greatest(n.updated_at, c.updated_at), n.id) < (sqlc.arg(below_at)::timestamptz, sqlc.arg(below_id)::uuid)
            AND NOT n.name ~ ALL(sqlc.arg(terms)::text[])
            AND n.name ~ ALL(sqlc.arg(titled)::text[])
        ORDER BY sort_at DESC, n.id DESC
        LIMIT sqlc.arg(walk)::integer + 1
    ) v
) w
LEFT JOIN LATERAL (SELECT c2.content || '' AS doc FROM page_contents c2
                   WHERE c2.node_id = w.id AND w.seq <= sqlc.arg(walk)::integer OFFSET 0) b ON true
LEFT JOIN LATERAL (SELECT NOT EXISTS (SELECT 1 FROM unnest(sqlc.arg(bodied)::text[]) p
                                      WHERE NOT (w.name ~ p OR b.doc ~ p)) AS passed
                   WHERE b.doc IS NOT NULL OFFSET 0) x ON true
LEFT JOIN LATERAL (SELECT min(nullif(regexp_instr(b.doc, p, 1, 1, 0, 'c', 1), 0)) AS at
                   FROM unnest(sqlc.arg(terms)::text[]) p WHERE x.passed) e ON true
WHERE x.passed OR w.seq = least(w.walked, sqlc.arg(walk)::integer)
ORDER BY w.sort_at DESC, w.id DESC
LIMIT sqlc.arg(max_rows);
```

- **`SearchTitles`**：候选是 `s.title @@ @keys` 并上队列，`LEFT JOIN page_contents`，页与附件都在；`WHERE` 里 `n.name ~ ALL(@terms)`，`ORDER BY … LIMIT @max_rows`；之后为这些行按键读正文（`LEFT JOIN LATERAL … OFFSET 0`）、取摘要的位置，没有正文的词时没有摘要。
- **外层的 `ORDER BY`** 沿用子查询的次序，不再排序，复查保持惰性（计划测试钉住，3.10）。
- **数组与参数**：`terms`（全部的词）、`titled`（只搜标题的）、`bodied`（其余的）是 3.3 的 ARE 模式（各带一个捕获组，`regexp_instr` 的第 7 个参数取它的位置）；`keys`、`body_keys`、`title_keys` 是 3.3 的 `tsquery`。`~ ALL(数组)` 是一个运算，不是每行一个子查询。
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
- **否则按层估**：
  - `n0`：名称层的命中，名称检查过，数到 1,001；另数其中的页 `p0`。
  - `c1`：正文层的候选（与走查同样的键与队列，活着、看得到的页，名称不检查），数到 `MaxSearchCountScan`；正文层的候选约是 `c1 − p0`。
  - `r1`：正文层的走查里通过的比例。第一页没走到第 1 层时（名称层填满了这一页），另以 `SearchBodies` 从第 1 层的顶走查 `SearchCountSample` = 100 个取它。
  - 估计 `N = n0 + max(c1 − p0, 0) × r1`。

  ```sql
  -- name: SearchCount :one
  SELECT t.titles, t.title_pages, b.bodies FROM
      (SELECT count(*)::integer AS titles, (count(*) FILTER (WHERE kind = 'page'))::integer AS title_pages
       FROM (SELECT n.kind FROM (/* SearchTitles' candidates */) k JOIN nodes n ON n.id = k.node_id
             WHERE n.deleted_at IS NULL AND n.notebook_id = ANY(sqlc.arg(notebooks)::uuid[])
                 AND n.name ~ ALL(sqlc.arg(terms)::text[])
             LIMIT 1001) a) t,
      (SELECT count(*)::integer AS bodies
       FROM (SELECT 1 FROM (/* SearchBodies' candidates */) k JOIN nodes n ON n.id = k.node_id
             WHERE n.deleted_at IS NULL AND n.kind = 'page' AND n.notebook_id = ANY(sqlc.arg(notebooks)::uuid[])
             LIMIT sqlc.arg(cap)::integer) a) b;
  ```

- **种类与值**（`SearchCount{value, kind}`）：
  - `n0` 到了 1,001，或 `N` ≥ 1,000：1,000，`at_least`（"1,000+"）。
  - `c1` 数到了上限：`N` 只是下界，`at_least`（"至少 N"）。
  - 没有正文层（每个词都只搜标题）时 `n0` 就是准确的数，`exact`。
  - 否则 `about`，取两位有效数字。
  - 一个都还没通过、又没走完时为空：网页说"还没有找到"。
- **为什么按层**：名称很短，名称层的通过率接近 1；正文层的候选对 3 个字以上、对字母的三元组只是上界（spike 的 Plan 3 里 `latency` 的三元组候选 14,028 个、真的 225 个）。混在一起的比例会把"名称里 25 个、正文里约 300 个"的 `kernel` 说成"1000+"（审查 A-I3）。
- **`MaxSearchCountScan`** 初值 20,000，第一页的 p99 超过目标时减半，直到达标（3.9）。

**`building`**：每一页都带：`EXISTS (SELECT 1 FROM page_search_queue WHERE bulk AND notebook_id = ANY(@notebooks))`。为真时这些笔记本里还有没建好的节点，结果可能不全。

**摘要与高亮**：

- **摘要**：在 SQL 里只为答出的行算（至多 `limit + 1` 行）。按词定位正文里第一处命中（`regexp_instr` 取捕获组的位置），取它之前 40、共 160 个字符，前后各多取一个字符作边界的上下文，与正文的字符数一起带回，不传整页的正文（spike 2.6：在 SQL 里切比回传整页再切便宜）。
- **在 Go 里**（`search_hits.go`）：
  - 在带上下文的摘要上按 3.3 的 RE2 模式找出每个词的全部命中（逐个找，从上一个命中的词的末尾接着找，不吞掉边界的字符）：上下文的字符让词边界看得到摘要外的那个字符。
  - 再去掉上下文的字符、把连续的空白合并成一个空格，区间随之换算，落在去掉的字符上的部分截掉；前后是否截断看位置与字符数。
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
2. **`ParseQuery`**：422 `validation_failed`（字段 `q`）。在找工作区之前：`q` 是自由文字，它的检查不说出工作区是否存在，同正文的大小与 NUL（13.1 第 4 条的例外）。
3. **`Workspaces.FindBySlug`**：没有这个工作区时 404 `workspace.not_found`。
4. **判定** `page.search`（`Target{WorkspaceID}`）：`ErrNotVisible` 也答 404 `workspace.not_found`。每种角色都允许，所以没有 403。
5. **`VisibleNotebooks.VisibleIn(ws, user, grant.WorkspaceRole)`**：
   - 带 `notebook_id` 而它不在其中：404 `notebook.not_found`。不答空列表，同别的接口。
   - 否则只搜它。
6. **`limit`**（`domain.SearchLimit`）1–50，默认 20：越界答 422（字段 `limit`，`out_of_range`）。分页的参数在资源之后，同 `listTrash`、`listChangesets`。
7. **看得到的笔记本为空时**答空的一页，`count` 为 `exact` 0，`building` 为假。
8. **一个只读事务里**：两层、第一页的计数、`building`、路径；之后高亮与附件的信息。

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
  - `SearchCount {value: integer 0–1000, kind: exact|about|at_least}`："`exact` 在第一页走完了全部候选时；`about` 是约数；`at_least` 是下界，1000 总是 `at_least`；只在第一页；还没有一个结果而查找没有走完时为空"。
  - `building`："这些笔记本里还有节点在等后台建索引（导入、升级、reindex 之后）：结果可能不全"。
- **结构**：
  - `SearchHit {node: SearchNode, ancestors: Ancestor[]（根在前）, title_hits: TextRange[], excerpt: SearchExcerpt|null, changed_at, attachment: AttachmentInfo|null}`。
  - `changed_at`：最后一次改动名称、位置或正文的时刻。不叫 `updated_at`：`TreeNode.updated_at` 不含正文的写。
  - `SearchNode {id, notebook_id, kind: NodeKind, name}`。
  - `TextRange {start, end}`：UTF-16 码元，`end` 不含，整数 ≥ 0；一个列表里的区间排好序、不相交（服务端合并不同的词的重叠）。
  - `SearchExcerpt {text, hits: TextRange[], cut_start: boolean, cut_end: boolean}`：原样的 Markdown，空白已合并。
  - 可空的对象照 P2 3.9 写成 `anyOf`。
- **`TrashAttachment` 改名 `AttachmentInfo`**：说明改为附件的类型与大小，回收站的单项与搜索都引用它。一种形状一个名字，P1A 审查 C6。它只在服务端的 `trash.go` 里有引用（网页只有生成的类型），与别的模块的结构不重名。
- **说明写明**：
  - 两层与次序。
  - 用户的写按已提交的状态；`building` 时可能不全。
  - 走查的界线：一页可能少于 `limit` 个、甚至没有而带着 `next_cursor`。
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
  3. 删除 → 任务删掉行 → 从回收站恢复：任务再跑之前搜索就找得到（队列）；之后重建。撤销一次删除同样。
  4. 单元持着队列的一行时任务出队：跳过、不等，下一次运行再读。持行用新的 `holdQueueRow`（`holdRow` 持的是 `asset_blobs` 的行）。
  5. 删除笔记本，任务同时在建它的页：搜索从不列它们，下一次删掉行。
  6. reindex 的入队停在一批的中间（持着前一半的行）时，一次删除子树的单元入队同样的节点：都按 id 的次序，单元等到 reindex 提交，不死锁，两边都完成。
  7. 导入的两批之间（`betweenUnits`）搜索：第一批的页不在结果里、答复带 `building`；`Drain` 之后找得到、`building` 为假。
  8. 任务停着时节点被清理（`purgeBefore`）：下一次删掉它的行。
  9. 两个 `Drain`：测试的连接持着任务的 advisory lock 时 `Drain` 立即返回、什么都不写；放开之后照常。

### 3.9 大小与实测

**目标**（第 3 节第 6 条的"目标"，300k 页、热、默认设置）：

| 项 | 目标 |
|---|---|
| 每类查询第一页（两层、计数、摘要、路径）与"更多"的 p99 | ≤ 300 ms，可见 200/200、10/200 本与限定一本 |
| 建索引时（30 万个节点以批量排队、任务在跑）的同上 | ≤ 300 ms，答复带 `building` |
| 准确的积压 1,000 页、1 万个附件时的同上 | ≤ 300 ms；记下第一页为空的查询的比例 |
| 体积 | `page_search` 的表、TOAST 与两个索引合计 ≤ 正文（UTF-8）的 3 倍（spike 的 B 约 2.7 倍） |
| 任务的吞吐 | 语料的页（平均约 8 KB）持续 ≥ 100 页/秒；200 KB 的页 ≥ 1 MB/秒。所以 30 万页从空建完 ≤ 1 小时，导入 1 万、5 万页之后约 2、10 分钟清空 |
| 保存时多出的开销 | 一页的保存 p99 多 ≤ 2 ms；1 万个节点的单元多 ≤ 0.2 秒（产品的代码上） |
| 计数 | 每个查询的估计（含 `at_least` 的值）与真实的数相差在 2 倍之内，或都 ≥ 1,000 |

只记录、不设目标：冷的、不开并行的（`max_parallel_workers_per_gather = 0`）；`VisibleIn` 在 200、5,000 本笔记本时；常见的键在 30 万页上的计划；1 MB 与 5 MiB 的页每次重建的时间、WAL 与索引在清理之前的增长；语料里到词的字节界线的页数；名称层对最常见的键的代价。

**第 1 步：用 spike 的语料与工具先量，再写代码**（P5A 的第一步，结果写回本节与 4.12）：

1. **语料**：照 spike 第 4 节重建 30 万页。
   - 按日期的目录下载同一期：zhwiki 20261001 的同样几个分卷，enwiki 20261008 的第一卷前 250 MB，约 3.1 GB。
   - `extract.py`、`build_pages.py`（种子 20261013），以留下的 `data/corpus/meta.tsv` 核对是同一份。
   - 拿不到这一期时用新的一期，写明数字与 spike 不能逐格对照。
2. **工具**：`bench`（Go，pgx，`DescribeExec`，`SET jit = off`）加：
   - 本文的分词，T 与 P 两种：就是要进产品的 `domain/search.go` 的草稿；P 的写法先在重建的写池上与 `tok.go` 核对一致（字符的类改了 Mc、Me 与 `data:`，差别只在那里）。
   - `page_search` 与 `page_search_queue` 两种写法的表（3.2 的 DDL）。
   - 3.5 的语句原样（审查 A 改正过的写法），一次请求一个只读事务。
   - 计数的估计；任务的一次运行（读、分词、每行一条 upsert、出队）。
3. **查询**：spike 的 28 个（`queries/queries.json`），加：
   - 1–2 个字母的整词：`ai`、`go`、`c`。
   - 词中间的子串：`api`（rapid）。
   - 由常见三元组组成、真的少于 20 页的词（P5A 从 df 表里挑）：走查的界线最先用完的情形。
   - 名称里很多、正文的通过率低的词（像 `kernel`），与计数数到上限的词：计数的两种难处（审查 A-I3）。
   - 单字加别的词：`锅 中国`。单字的混合：`A股`。正文层没有键的：`锅 #`。8 个词的查询。
   - 最常见的键：`the`、`ing`、`a`、`of`、`中国`；记下它们在 30 万页上的计划（计划测试关掉了整表扫描，看不到生产的选择）。
4. **形状**：
   - 第一页（含计数）、第三页之后的"更多"、只有名称层的。
   - 可见 200/200、10/200、限定一本（约 1,500 页）、5,000 个 id 的数组（实有的加上不存在的）。
   - 建索引时：30 万个节点以批量排队、任务在跑。
   - 导入 5 万页之后：清空期间与之后。
   - 准确的积压：最近写过、还没重建的 1,000 页；恢复回来的 1 万个附件。
   - 每格先跑一次热身，再至少 20 次；最近秩的 p99；记机器的负载。
   - 冷的（重启并逐出页缓存）只跑 `中国`、`数据库`、`latency`、`ai`。
5. **体积与建索引**：
   - 两种写法的表、TOAST、每个索引的大小。
   - 任务从空建完的时间与吞吐（`fastupdate` 开与关）：页/秒、MB/秒；导入 1 万、5 万页之后清空的时间。
   - 对照：表填满之后 `CREATE INDEX` 的时间。
6. **写**：
   - 任务每页的代价：10 KB、200 KB、1 MB、5 MiB，新建与重建，`fastupdate` 开与关，待处理列表空与满；1 MB、5 MiB 记 WAL 与索引的增长。
   - 待处理列表满（4 MB）时的查询。
   - 请求里的入队：一页、1 万个 id 一条语句。
7. **计数**：每个查询离线全量复查得出真实的数，与估计比。

**取舍的规则**（P5A 按数字定，写回）：

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

**第 2 步：在产品的代码上复现**（实施之后，手动，`heavy.sh` 持 `scratchpad/m7/heavy.lock`）：

- `bootstrap/search_bench_test.go`：
  - `BenchmarkSearch`：
    - `NERVE_SEARCH_CORPUS` 指向第 1 步的 COPY 文件，以 SQL 写进测试库的 `nodes` 与 `page_contents`。没有时跳过。
    - 全部以批量入队，以生产的 `Drain` 建完（计时，即吞吐）。
    - 经处理器在进程内跑各类查询：生产的中间件与 15 秒的请求期限，同 P2A 的 `newTrashBench`；含 `VisibleIn`。
    - 量体积。
    - 无统计、`ANALYZE` 之后、强制通用计划各一遍。
  - `BenchmarkSave`：经处理器保存一页（10 KB、200 KB）的 p50 与 p99。在 P5A 与它的基点上各跑一次（把基准文件复制到基点），差就是入队多出的开销。
- 重跑 `BenchmarkTrashDelete`（5 万个节点）与 `BenchmarkRevert`，同样在两边各一次，记入队多出的时间。
- 达不到第 1 步的数时按差别改，或回到取舍的规则。

**守卫**（`page/adapter/postgres/search_cost_test.go` 的 `TestTheSearchReadsInTimeAtAnyStatistics`，`//go:build !race`，加进 Makefile 的 `test-go-costs` 的 `-run` 列表）：

- **语料**：确定的合成语料，5,000 页、每页约 4 KB，常用汉字按 Zipf 分布，加一张英文词表；以生产的分词经 `Drain` 建好；另有 5,000 个以批量排队的节点与 500 页准确的积压。
- **用例**：每类的第一页与第三页，加走查用完的那一类、8 个词的与 `building` 时的。
- **建与入队**：`Drain` 建完 5,000 页；1 万个 id 的 `QueueSearch`。
- **方法**：一个连接上无统计、ANALYZE 之后、强制通用计划各读 8 次。
- **时限**：各取本机中位数的三倍（第 7 步量了再定，在请求的 15 秒期限之下），同 `8c83ccb7` 的规则。300 ms 的目标由 30 万页的基准核对，守卫只抓计划的退化（秒对毫秒）。
- **CI 的时间**（第 8 步量了写回）：
  - 成本那一份加约 1 分钟，现在 9–13 分钟，时限 20 分钟。
  - 竞态检测下组合根之外的包加适配器的测试与模式的性质测试，约 1 分钟，现在 4–6 / 15。
  - 竞态检测下的组合根加整个程序、增量等于重建、9 个交错（各起一个应用）、可见性的两遍与矩阵的两行，估计 1.5–2.5 分钟。P4 也在加；测试一步过约 13 分钟时，按测试名分成两份（Makefile 与 CI 的矩阵各加一份），同 M12 移交第 15 项的约定。

### 3.10 测试

- **domain**：
  - 分词：
    - 每类字符与段（CJK、字母、数字、`Mn`、`Mc`、日文的长音、韩文、天城文）。
    - 64 的界线、`data:` 串、按折叠之后的长短（ß、ﬃ）。
    - 标题与正文的差别（单字只在标题）。
    - 词的次序与 `MaxSearchLexemeBytes` 处的截断（字节之和）。
    - 泰文等带 `Mn` 的字母段（spike 的 Python 分词在这里与 Go 的不同）。
  - `ParseQuery`：
    - 每种取值的错误：缺了、空、257 字节、9 个词、65 个字母的段、NUL、坏的 UTF-8、全是标点。
    - 全角空格切词、去重、每种词的键与"只搜标题"（`锅`、`锅！`、`A股`、`锅 #`）。
    - 模式的边界：Go语言、C++、node.js、A股、单个字母、数字。
    - 大小写的类：每个多于一个成员的 `SimpleFold` 轨道，与完全折叠的特殊字符（ß、ẞ、İ、ı、ſ、K、µ、ǅ、ς、ﬃ）。
    - 两种写法的模式的转义（`%`、`_`、`\`、`[`、`(`、`.`）。
  - **复查蕴含键**（`TestTheRecheckImpliesTheKeys`，性质测试）：生成的文字与查询（ASCII、CJK、标点、空白、上面那些特殊字符、带重音的字母），RE2 的模式通过时查询的键都在 `SearchTokens` 的词里；生成器另造 3.3 的例外，核对它们正是例外。
  - 高亮：
    - 开头、结尾、多字节、代理对（emoji 在名称里）的 UTF-16 区间。
    - 上下文的字符：摘要开头的 1–2 个字母的词前面是字母时不高亮；空白合并之后的区间。
    - 相邻的整词；不同的词重叠的区间合并。
  - 计数的估计：`exact`、`about`、`at_least`（数到上限、到 1,000）、按层（名称多而正文的通过率低）、样本、没有一个通过。
  - 游标：层的切换、走查恰好用完、UTC。
- **用例**（`page/app`，假的端口）：
  - 码的次序：游标 → `q` → 工作区 → 判定 → 笔记本 → `limit`。
  - `notebook_id` 看不到时 404 而不是空列表。看得到的为空时答空、`count` 为 `exact` 0。
  - 两层：名称层不足时补正文层；走查的界线；三种 `next_cursor`；只在第一页计数；`building`。
  - 附件的信息与路径的组装。
  - 入队：四种情形、移动不入、导入以批量、id 已排序。
  - 任务（假的存储与时钟）：
    - 取不到锁时返回。
    - 一遍按 `node_id`、每个节点至多读一次；一批 8 MiB 的界线与 `@after`。
    - 删、不动、30 秒之内跳过、重建四种判断；分词的版本不同时重建。
    - 出队带着读到的版本号；第 4、5 步之间中断；写不进的节点逐个再试、记下、跳过，别的照常出队。
    - 每 10,000 行 `ANALYZE`；1 分钟的预算。
- **适配器**（真的 PostgreSQL）：
  - `QueueSearch` 的版本号取自序列、批量的与，`ORDER BY 1`；`DequeueSearch` 不删版本号变了的、跳过被持着的行；advisory lock 一次只给一个。
  - 两层的语句：
    - 子串、整词、只搜标题、每个词在名称或正文、名称层的排除、正文层没有键时以标题的单字找候选。
    - 准确的队列里的节点不靠索引也找得到；批量的不当候选、`building` 为真；已删的（含还有行的）与看不到的不在。
    - 走查的最后一行与 `walked`；摘要的位置。
  - 计数到 `cap`、名称层到 1,001、页的数。
  - **模式的性质测试**：生成器含每个多于一个成员的 `SimpleFold` 轨道与完全折叠的特殊字符，每个模式在 PostgreSQL 的 `~` 与 Go 的 RE2 里答得一样，`regexp_instr` 的位置等于 Go 的第一个捕获组。
  - 迁移测试（3.2）。
- **计划测试**（`search_plans_test.go`，`planOf` 再复制一份，同 P3）：在种下、ANALYZE 过的几千页上，关掉整表扫描（队列除外：它的整表扫描是对的）。
  - 两层分别走 `page_search_title_idx`、`page_search_body_idx`，`nodes` 按主键。
  - 正文层：走查的 `Limit` 直接在它的 `Sort` 之上，中间没有 `WindowAgg`；`page_contents` 在走查之后按主键读；`EXPLAIN VERBOSE` 里正文的拼接只出现一次；外层没有第二个排序。
  - 名称层：名称的检查在 `Limit` 之下，正文在 `Limit` 之上按主键读。
  - 计数走 GIN；`building` 走队列的索引。
  - 任务的读按主键；`TrashLines` 按键。
- **整个程序**（`bootstrap/search_test.go`，13.1 第 21 条）：
  - 组合根不交 `VisibleNotebooks` 时 `page.New` panic。
  - 经接口：
    - 新建页与附件、写正文、改名、移动、删除、从回收站恢复、撤销、还原版本、后台的恢复与撤销（P4 合并之后）。
    - 每一步之后搜索立即反映它，答复不带 `building`。
    - 导入一个小 zip：任务跑之前它的页不在结果里、带 `building`；`Drain` 之后找得到。
    - 做空队列之后 `checkSearch` 成立。
  - River：任务在 `page_search` 队列上（同 `TestTheJobsHaveTheirQueuesAndOutlastRiversRescue`）；间隔 1 秒时完成一次之后再次运行（唯一的选项不含 `completed`）。
  - `nervewiki reindex`：行是当前的时不入队；把行的 `tokenizer` 改旧之后以批量入队，`Drain` 之后是当前的。
- **不变式** `checkSearch`：
  - 不做空队列也成立：每个活着的节点有 `revision`、`name` 是现在的行，或在队列里（分词的版本旧了的行在 reindex 之前照常回答，不算）。
  - 做空之后：队列为空；`page_search` 的行恰是活着的节点的；`title`、`body` 等于 `SearchTokens` 的。
  - 加进交错与整个程序的收尾核对。
- **增量等于重建**（`search_rebuild_test.go` 的 `TestTheSearchIndexIsItsRebuild`，13.4 第 10 条）：随机的操作序列，沿用 P3 的性质测试的生成器（新建页与附件、改名、移动、删除、恢复、写正文、还原、撤销），另加一次小的导入；每隔几步做空队列，`page_search` 等于从头分词的结果。固定的几个种子。
- **矩阵与可见性**：3.8。
- **handler 与平台**：契约的结构、参数与各码；`count` 的界线与种类。

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

- **收尾**：00 号文档 4.12 的数以 A 的第 2 步为准；第 3 节第 6 条逐项对照 3.9。
- **M9**（`grep`、`index`，新的移交）：
  - `grep` 用这个索引：
    - 从正则的字面片段取键：用 `regexp/syntax` 解析，取每段必须出现的字面串，CJK 2 个字起给二元组，字母 3 个起给三元组，大小写按标志。
    - 候选并上队列里准确的节点，在 SQL 里以正则复查，路径的 glob 再筛；`building` 时同样说明。
    - 取不出键的正则只能有界地扫：同 `MaxSearchWalk` 的走查，答"继续"。
    - 字母文字若定为词的前缀（P），字母的任意子串取不出键。
  - 队列让刚写的页立即可查，agent 写完即 grep 不会漏。agent 以 batch 大量新建时（导入以外的单元是准确的），看任务的吞吐是否跟得上。
  - MCP 的 `search`（若加）调用同一个 `Search` 用例。
- **M12**：第 7 节的移交；搜索的并发上限与按工作区的预算；繁简映射（OpenCC 一类，在分词与复查两边）；NFKC、重音与完全折叠的复查。

## 9. 结果

（实施之后写：A、B 各一节，提交、实测、审查、与设计不同之处、反向对照、门禁、交给后面的。）

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
