# M9/P5 其余的故事的 MCP 版本与接入文档设计：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | [05-P5-stories-docs.md](../05-P5-stories-docs.md)（初稿，2026-10-23，176,469 字节、1,148 行；留作 `05-P5-stories-docs.draft1.md` 与 `parts-draft1/`）。对照的是 `main` `7dc4a686` 的代码与文档（P4 的文档提交），以及：[00 号文档](../00-M9-design.md)（4.4、4.16，第 3、7、9、10、11 节，"移交的落实"）；P1–P4 照建成的样子与它们第 8 节交给 P5 的；移交 M0/P1；`v0.1-design.md`；M0/P1 的实验；M4/P6 的输入法清单 |
| 审查方式 | 三位审查者（Opus）并行，只读。不运行任何 MCP 客户端，不读负责人的客户端配置，不下载。<br>A：故事的 MCP 版本、fixture、变异与 e2e 那一份（第 0、1、3、8、9 节，2.1 的 e2e 一侧）。<br>- 在 `review-A/tc/e2e` 的副本上照草稿改 fixture，跑一次 `tsc --noEmit`：只多出 MC2 的 `PageRead` 一个错（A-1）。<br>- 三位帮手（Opus，同样的规则）各核对一组故事：`helper-1.md` 管 PG1–PG4、PG9；`helper-2.md` 管 PG11、PG12、L1、L3、L4、L6、SE1、SE2；`helper-3.md` 管 AS2–AS4、HI1 与豁免。A 对着代码核对过它们的要点之后才收，下面标作 [H1]–[H3]。<br>- 没有起容器，没有跑 e2e。<br>B：README 的接入一节与反向代理（第 4、5 节，2.1 与第 7 节的 README 一侧）。<br>- `review-B/nginx/`：草稿的样例，`nginx -t` 通过。<br>- `nginx-route/`：样例加容器自己的回环上的一个假的上游，2 MiB 的 POST 发到每类路由。<br>- `caddy/`：I3 的 Caddyfile，量五个路由的上游 `duration`。<br>- 三次都是用完即删的运行，经 `heavy.sh`，`--rm --pull=never --network none`，没有映射端口，只用本机已有的 `nginx:1.29-alpine` 与 `caddy:2.10-alpine`。<br>- 读已装的 Codex 的二进制里的字符串被权限系统拒绝，所以客户端的行为只对着仓库的记录核对，其余标"未核对"。<br>C：人工清单、设计的修订与完整（第 2、6、7、10、11 节与附录）。<br>- 逐项列出 4.16、P1–P4 第 8 节与移交 M0/P1 交给清单的每一项，对到清单的步与预期，共 27 项。<br>- 2.1 与第 7 节的每段原文以 `grep -o -F` 对着 `7dc4a686` 数过。<br>- 说明的字节以 `instructions.go` 与 `answer.go` 的副本重算（`review-C/instbytes/`）。<br>- 看已装的客户端包被会话的权限分类器拒绝，同 B 标"未核对"。没有起容器 |
| 日期 | 2026-10-23 |
| 结论 | **各人的数**：<br>- A：Important 0、Minor 23、Nit 30（A 自己的 6 条与三位帮手的 24 条，按条数）。<br>- B：Important 1、Minor 10、Nit 11，过度设计 2。<br>- C：Important 4、Minor 16、Nit 13，过度设计 1。<br>合计 111 条。B-M9 与 C-M3 是同一个发现，B 的项目范围的代码块与 C-M6 的后一半是同一处，各合在一起改（"重叠"）。<br>**处理**：全部处理，没有不采纳的。修订直接写进设计，初稿留作 `05-P5-stories-docs.draft1.md` 与 `parts-draft1/`。处置由主持者定下（"主持者的决定"），负责人可以改判。<br>**五个 Important** 都照主持者的方向改了：<br>- README 的开头不再说样例在真实的客户端上走过，4.2 的第一行管它（B-I1）；<br>- 两个客户端各在自己的一本笔记本里做（C-I1）；<br>- F1 的占位是一个一直占着名额的循环（C-I2）；<br>- F1–F3 在账户入口上做，内置的说明里有 64 KiB 的那一行（C-I3）；<br>- Codex 的登录照主持者定的次序，`auth.json` 照指纹才复制回去（C-I4）。<br>**Caddy 现在就定了**（B-M1）。作者照 B 的脚本在 `heavy.sh` 之下重跑：缓冲的三个路由毫秒级，上传与导入约 3.85 秒。另量了缓冲在内存里还是磁盘上：30 MB 的请求体让 Caddy 涨到约 109 MB，没有临时文件。README 的样例 `caddy validate` 通过。于是 README 写出 Caddy 的样例，I3 改为在负责人的部署上确认（"复核的实验"）。<br>**作者的更正**：修订里有两处与审查所料不同：`make e2e` 先构建（A-8）；Caddy 的内存是本机量的，不是"照文档"，也不是"未核对"（B-M1）。<br>**修订的规模**：<br>- 9.2 的变异由草稿的 223–246 变为 223–251：235 分成 235a、235b，加上新的 247–251；<br>- 2.1 由 12 条变为 13 条，第 13 条是 4.16 的接入文档一条的令牌变量与 Caddy 的分句；<br>- 第 6.2 节（按步的来源）去掉，2.3 按来源带着步的编号。<br>**之后**：<br>- 对修订的三轮核对在后面的"第一轮复核""第二轮复核""第三轮复核"；这一节里被它们更正的说法就地标着"第 n 轮复核更正"。第一、二轮复核各有几个 Minor 与 Nit，都落下；第三轮复核（C）没有 Important 与 Minor，它的两个 Nit 与作者的两个 Nit 都落下。之后由主持者自己最后核对，不再有审查者的一轮。<br>- 设计在第四次修订之后是 262,560 字节、1,439 行，前几次的样子留作 `.rev1.md` 到 `.rev3.md` 与 `parts-rev1/` 到 `parts-rev3/`。<br>- `apply-00m9.py` 的 21 处改动（第三轮复核之后的样子）以 `--check` 对着 `7dc4a686` 的 00 号文档核对过，全部对上。只核对，没有写仓库（"00 号文档与别的文件的改动"）。<br>- 作者在第一轮修订里有一处过程的失误：一个留下来的容器，与两次不经 `heavy.sh` 的列表（"过程中的一处失误"） |

## 发现与处置

节号是修订后的。"第 n 条变异"是修订后的 9.2。

### A：故事的 MCP 版本、fixture、变异与 e2e 那一份

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| A-1 | Minor | `readPage` 挪进 `fixtures/mcp.ts`，而 `PageRead` 没有导出。MC2 的 `readAll` 的签名写着它（`mc2-read-list.spec.ts:86`），所以第 1 步编不过：A 在副本上跑 tsc，只多出这一个错 | `PageRead` 与 `readPage` 都导出，MC2 导入两者。第 1 步由 `make lint`（`pnpm -r run check:types`）证明编得过。改在 3.1 第 4 条、2.1 第 1 条、第 8 节第 1 步、第 0、10 节与附录 |
| A-2 | Minor | PG7 (MCP) 去掉了会话的断言，却对会话什么也不断言。MCP 的写若开了或留着编辑会话，就会锁住网页，今天没有测试抓得到 | 第 4 步之后断言 `sessionsOf(db, page.id)` 为 `[]`（`assert/page.ts:474-480`）。改在 3.1、3.2 PG7、2.1 第 3 条与 9.1 |
| A-3 | Minor [H1] | PG1 的 `name` 取"答复的 created 的最后一项的路径去掉 .md"，得到的是 `"Roadmap/Q4"`：`created[].path` 是从根起的整个路径。`expectNewPage` 会在 `nodes.name` 失败 | 用测试给的字面的标题（"Roadmap"、"Q4"）（3.2 PG1） |
| A-4 | Minor [H1] | PG3、PG4 没有核对移动与删除是 agent 的：`expectMoved`、`expectSubtreeDeleted`、`expectDeletionRevisions` 都不读 `changesets.client`，e2e 里也没有别的测试核对 MCP 的删除的客户端 | PG3 第 2 步加 `expectChangesetOf`（items `[b]`，versions `[]`）；PG4 第 4 步加同样的一个（items `[doomed, child, grandchild]`，versions `[]`）。改在 3.2、9.1 与 2.3 的 P2 一行 |
| A-5 | Minor [A、H2] | L6 (MCP) 第 3 步可能把第 1 步晚到的帧当成它等的那一帧：Doc 的写同样发 `links {targets: [hub]}`，而 L6 (API) 也不 settle。这时丢掉 MCP 的正文替换的 `links` 事件的变异照样通过 | 照主持者的决定（第 3 条）：第 1 步之前建一个 Marker，开流之前 `settleEvents`；L6 (API) 在同一个提交里同样补上。改在 3.1、3.4 L6、第 8 节第 5 步、9.1 与第 10 节 |
| A-6 | Minor [A、H2] | 两处不对：<br>- "Go 里没有 MCP 的写到事件流的测试"：`bootstrap/events_pages_test.go:141-161` 经笔记本入口的 `batch` 读它的帧。<br>- 第 235 条放在 `Unit.tell`：那里喂每个观察者，MCP 的链接的索引也会一并停掉，MC4 今天就会失败 | 3.3 改正：一个 MCP `batch` 的帧 Go 里测过；C7 (MCP) 是每个工具单独的帧与跨两页的 batch 的第一个测试。第 235 条分成两条：<br>- 235a：`bootstrap/events_registrants.go:23-32` 的 `PagesChanged` 对 MCP 的写返回 nil，C7 (MCP) 第 2 步与上面的 Go 测试都失败；<br>- 235b：linking 的索引对 MCP 的单元不 `x.publish`（`linking/app/index.go:85`）。<br>改在 3.3、9.2。（第一轮复核更正：照这样放，235b 在 `index.go:85` 分不出 MCP 的单元，改为两处的变异，A2-M1） |
| A-7 | Minor | 第 236 条说"C7 (API) 的子树的删除同样"，不对：删除 Parent 与 Minutes 是两帧，C7 (API) 在删除之后只读一帧。C7 (MCP) 也从不核对流是安静的 | 照主持者的决定（第 3 条）：<br>- C7 (MCP) 以安静的检查结束：Eng 的一次 Marker 的写，它的帧要是 B 的下一帧；<br>- C7 (API) 在同一个提交里加同样的检查；<br>- 9.2 第 236 条写明今天的 C7 (API) 抓不到，补上之后抓得到。<br>改在 3.3、第 8 节第 5 步与 9.2 |
| A-8 | Minor [A、H1] | 几条服务端的变异照放的位置，要么不改二进制，要么改不了：<br>- 语句的变异在 sqlc 生成的常量里：#229 要改 `gen/sessions.sql.go:19`；#234 的来源是 `activity.sql:5-10`，不是 `list_ownerless.go`。<br>- #230 在 `text.go` 拿不到调用的 `base_version`。<br>- #227、#229 的"API 版本同样"不对。<br>- #237 在 L3 第 2 步失败，不是第 5 步。<br>- #238 应是 `path_writes.go:36-40` 的 `read = nil`。<br>- #241、#245 没有位置。<br>- #243 说的机制不存在 | 每一行照改：<br>- #229、#234 改在 `gen/*.sql.go`；<br>- #230 改在 app 里，`write_path.go:250` 与 `edit_page.go:143-146`；<br>- #241、#245 写明位置与另外谁抓到，以及 #245 的时间的洞；<br>- #243 改在 `bootstrap/assets.go:78-91`。<br>第 8 节第 8 步写明：服务端的变异先 `make build`；语句的变异改在 `gen/*.sql.go`；改着变异时不执行 `make gen*`；变异从不提交。作者更正了 A 的一处说法（"作者的更正"第 1 条）。改在 9.2 与第 8 节第 8 步 |
| A-9 | Minor | 第 224 条不是 `writtenOf` 的节点检查的反向对照：PG7 第 3 步比的是字面的 `"<id>:3"`，不论 `writtenOf` 查不查节点都会失败。`writtenOf` 自己的检查从没被证明会跑 | 照主持者的决定：第 224 条是普通的变异，MC3 也抓。`writtenOf` 的检查以两次伪造的答复证明各抛出一次：一次的 `version` 以别的节点的 id 起头，一次 `at: null`。这只记进审查记录，不提交。改在 3.1、第 8 节第 8 步与 9.2 |
| A-10 | Minor | PG8 (MCP) 唯一的变异不是 PG8 的：#230 是 MC3 的。只有 PG8 (MCP) 做的过时的 `edit` 没有变异 | 新的第 247 条：`edit` 不看 `base_version`（去掉 `edit_page.go:141-146` 的两个 `VersionMismatch`），PG8 (MCP) 第 3 步失败（9.2） |
| A-11 | Minor [H1] | PG4 第 3 步的锁是子树的次序（`edit_lock.go:87-113`），不是按 id；`lock_holders.go:49` 是 linking 的 `pages_locked` 的。按 id 排的期望只因 UUIDv7 通常递增才通过 | 字面地写 Child、Grandchild（"按子树的次序"），引 `edit_lock.go:87-113`（3.2、附录） |
| A-12 | Minor [H1] | PG3 的标题说"每次都在新的兄弟之后"，只有第 3 步查次序 | 第 2 步经 REST 读 A 的子页（`["A1", "B"]`）。新的第 248 条只有 MCP 抓：`unit_relocate.go:70` 把节点放在最前（3.2、9.2） |
| A-13 | Minor [H3] | 21 个里只有 AS3 (MCP) 在 9.2 里没有变异 | 新的第 249 条：附件改名时 linking 不改写属性里的链接（`linking/domain/rewrite.go:76-78`）。AS3 (MCP) 第 2 步失败，AS3 (API) 也抓，是共用的覆盖（3.5、9.2） |
| A-14 | Minor [H2] | PG12 写得不够：<br>- 读者的六个写没有参数；<br>- 陌生人的账户入口的清单少了 `batch`；<br>- 成员的 `move`、`delete` 的答复没有断言 | 列出六个调用与参数，`base_version` 取读者自己的 `readPage`；陌生人的清单加 `batch`；断言成员的答复（3.2 PG12、9.1） |
| A-15 | Minor [H2] | L4 的 `index {tag}` 与 REST 的 `getTag` 比较的是同一条语句（`TagPages`），选标签的变异会让两边一起变 | 每个 `index {tag}` 的答复先与字面的列表比（照 MC7 按写的 id 排），再与 `getTag` 比；折叠与 API 版本的字面比；`#proj` 与 `getTag("proj")` 比，因为 REST 不去掉 `#`（3.4 L4、9.1、9.2 第 241 条） |
| A-16 | Minor [H2] | SE1 让 `expectFound` 的 `marked` 变成可选，API 与页面版本就失去了对它的守护 | 照主持者的决定：`marked` 仍是必须的，MCP 的调用处以 `{marked: false}` 不比。反向对照是第 251 条：`foundOf` 给 `marked: []`，SE1 (API) 失败。改在 3.7 SE1、第 8 节第 7 步、9.2 与 7.2 的 13.4 第 3 条。（第一轮复核更正：类型以重载 `Omit<Found, "marked">` 写，A2-N1） |
| A-17 | Minor [H2] | SE1 的路径到 id 的对照没有来处：`written()` 只给四个 id。每个查询又只有一个命中，"次序"什么也不比 | REST 的每个命中的路径由 `ancestors` 与 `node.name` 拼出，与 MCP 的 `[path, kind]` 比；只说"路径与数"，不说次序（3.7 SE1、9.1） |
| A-18 | Minor [H2] | SE2 的 `notebooks` 不是搜索的可见性（`search` 按 `Visible.VisibleIn`），七本里只搜了两本 | 每个账户搜它列出的每一本：管理员 4 本、成员 2 本、访客 1 本，各恰好 1 个命中（3.7 SE2、9.1） |
| A-19 | Minor [H2] | PG11 的 `meta.path` 只是回显请求，看不出读经过了祖先 | 读 `a/b/c.md`，期望 `meta.path` 为 `"A/B/C.md"`：按键匹配，答存下的名称（`read_path.go:80`）。另读 `"B/C.md"`，答 `page.not_found`（3.2 PG11、9.1） |
| A-20 | Minor [H3] | AS2 的 `link` 分不出链接与名称：唯一的名称的链接就是名称本身 | 先在笔记本的别处上传第二个 `chart.png`，这样 Guide 的上传的答复里就是路径的链接。必须先传，因为 `assert/asset.ts:190` 比的是上传的答复的 `link`（3.5 AS2、9.1） |
| A-21 | Minor [H3] | AS2、AS3 去掉了 3.1 说留着的 REST 的读 | 照主持者的决定留着：AS2 的 `getView` 的 `<img>`；AS3 的 `getView`、`getPageProperties` 与改名之后的 `readContent`。3.1 写明规则"REST 的读照旧"，各故事的"不同"列出去掉的（3.1、3.5、9.1） |
| A-22 | Minor [H3] | AS4 第 4 步的"签名仍有效，404 说的是行不在了"不对：内容路由因节点已删除答 404（`asset/app/content.go:59-64`），在读行之前 | 改为 404 说节点在回收站里，行由 `expectAssetsDeletedWithNodes` 看到（3.5 AS4） |
| A-23 | Minor [H3] | TB1、TB2 记在"MC4 与 PG4 的 MCP 版本覆盖"不对：MC4 的 `expectTrashRoot` 不带 `{alone: true}`，PG4 (MCP) 根本不查回收站 | PG4 (MCP) 第 4 步加 `expectTrashRoot(db, doomed.id, {pages: 3, assets: 0, height: 3}, {alone: true})`，TB 的记账挪到它（3.2、3.8 的 TB 一行、2.1 第 8 条、9.1） |
| N-A1 | Nit | 第 0 节说 e2e 的改动"只加测试""两处类型的放宽"，草稿自己做得更多：去掉 MC3 (activity)，改 SE1 的 `expectFound`，把常量提到顶层，放宽是三处 | 第 0 节列出每一处 e2e 的改动 |
| N-A2 | Nit | 第 4 步也要去掉 MC3 开头注释里活动的那一句（`mc3:18-19`） | 照改（3.2 PG14、第 8 节第 4 步） |
| N-A3 | Nit | C7 (MCP) 不调 `edit`，3.3 却把它算进到达单元的工具 | 第 2 步的正文的写改为 `edit`（3.3）。（第一轮复核更正：第 2 步因此自相矛盾，A2-M2） |
| N-A4 | Nit | 3.3 靠"MC3 (page) 已看到一次 MCP 的写更新打开的页"。MC3 不持着流，那次更新可能来自重连时的刷新 | 写作弱的支持，不当证据（3.3） |
| N-A5 | Nit | 3.1 说两个 agent "各用自己的 PAT 与名称"，而 PG2、PG12、SE2 用默认的名称 | 改为"各一个 PAT；要分开它们的变更集时各有自己的名称"（3.1） |
| N-A6 | Nit | 2.1 第 7、8 条与 7.1 引 AS4 一行为 `:592`、TB 一行为 `:594`，实为 `:589`、`:592` | 照改（同 C-N6） |
| H1-1 | Nit | PG2 的 `STRASSE.md` 的拒绝是 API 版本的 `twin`（`pg2:54-55`），不是"另加的" | 照改（3.2 PG2、9.1） |
| H1-2 | Nit | PG4 第 2 步比整个文字与 `fields` `[["recursive", "required"]]` | 照改（3.2 PG4）。（第一轮复核更正：整个文字要带码的前缀 `validation_failed: `，A2-M3） |
| H1-3 | Nit | PG1 第 5 步把 `fields` 映成 `[field, code, message]`，才分得开两个 `invalid_format` | 照改（3.2 PG1、9.1） |
| H1-4 | Nit | PG9 断言 `total_lines` `[4, 1, 3, 1, 3, 1]`，钉住"只有 `\n` 分行" | 照改（3.2 PG9、9.1） |
| H1-5 | Nit | PG3 的移动的 `at` 由 MC4 核对，写明 | 照改（3.2 PG3、附录） |
| H1-6 | Nit | PG2 不要 `acute` 常量：API 版本有字面的（`pg2:49`） | NFD 的字面提到 PG2 文件的顶层，两个测试共用（3.2 PG2、第 8 节第 2 步） |
| H2-1 | Nit | 第 232、239、240 条的位置：`list_by_pattern.go:296-298`；`locks.go:60`、`:67`；`linking/app/rewrite.go:62-64` | 照改（9.2） |
| H2-2 | Nit | 附录里 PG11 应引 `list_by_pattern.go`，不是 `list.go` 的结构 | 照改（附录） |
| H2-3 | Nit | PG12：读者加 `list {}`；六个 `forbidden` 与 API 版本共用（`pg12:59-69`）；"从不答 `page.not_found`"引 `decideNotebook` | 照改（3.2 PG12、附录） |
| H2-4 | Nit | L3：`user_id` 取 `accountIdOf`；断言第 6 步的答复与它的变更集 | 照改（3.4 L3） |
| H2-5 | Nit | L6：断言 MCP 自己的 `next_cursor`；按写的 id（`byId`）排，不靠 UUIDv7；写"第一个 `links` 帧" | 照改（3.4 L6） |
| H2-6 | Nit | SE1：断言 `delete` 的答复 `[storage.id, 2, 1]`；摘录用 REST 的 `cut_start`、`cut_end` | 照改（3.7 SE1） |
| H2-7 | Nit | SE2：加 `truncated: false`；说名称怎样由路径得来 | 照改（3.7 SE2） |
| H2-8 | Nit | 3.1 把 L3 列为 `readPage` 的使用者，它只经 REST 读 | 从那张清单里去掉（3.1） |
| H3-1 | Nit | 第 244 条也由 `page/adapter/mcp/history_test.go:110-150` 抓。只有 MCP 抓得到的变异是：经 `GetVersion.AtPath` 读版本时丢了 BOM 或 CRLF | 写明；新的第 250 条（`get_version.go:86`），只有 HI1 (MCP) 第 4 步抓（9.2） |
| H3-2 | Nit | AS4 的清理不是"TR 的豁免"，是平台的定时任务 | 照改（3.5 AS4、2.1 第 7 条、9.1） |
| H3-3 | Nit | AS4 的第二个 API 测试不写 MCP 版本，更好的理由是：大小读的是没删的 `asset_blobs`，第 4 步已经证明了 | 照改（3.5、2.1 第 7 条） |
| H3-4 | Nit | AS4 可以断言删掉的附件的文件仍在存储里，恢复要用它们 | 加上，用 `blobPath`（3.5 AS4 第 3 步、9.1） |
| H3-5 | Nit | HI1：列出去掉的两处 API 的检查（`held.id`、`base_revision`）；`readContent === third` 留作 `readPage(...).text` | 照改（3.6、9.1） |
| H3-6 | Nit | AS2 以 MCP 的 `write` 种 Guide，违背 3.1 的"准备照 API 版本经 REST" | Guide 经 REST 种，版本取 `readPage`（3.5 AS2） |
| H3-7 | Nit | C6 的说法（3.8、第 0 节、PG4 第 4 步）："过期的会话不再拒绝 agent 的写"应为"agent 的删除" | 照改 |
| H3-8 | Nit | HI4 有三个调用者：撤销的再删除、REST 的删除、网页的删除，不只是"REST 的删除之后" | 照改（3.2 PG4、3.8、附录） |
| H3-9 | Nit | AS5 的"MCP 自己不送附件的内容：图片在答复里"读起来自相矛盾 | 改为"答复里只有图片；别的附件经内容路由下载"（3.8 的 AS5 一行） |
| H3-10 | Nit | `Module.Tools` 在 `page/tools.go:31-37`，不是 `:30-33` | 照改（3.8、附录） |

A 的几个结论：

- **四处可改判的取舍都成立**：PG14 由 MC3 挪来；C7 以帧断言、不开浏览器；PG7、PG8 以 `expectVersions` 代会话的断言。nginx 按路由不在 A 的范围。
- **豁免**：共 49 个故事，22 个有 MCP 版本，27 个豁免（表里 14 行），与 `00-M9-design.md:598` 相同；没有既不覆盖也不豁免的故事。每条豁免的理由对着代码成立，只有 TB1、TB2 的记账除外（A-23）。
- **fixture 的四处改动**：三处放宽让每个已有的调用者的断言照旧，A 的 tsc 编得过；`readPage` 的挪动见 A-1。
- **过度设计**：e2e 一侧没有。

### B：README 的接入一节与反向代理

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| B-I1 | Important | README 的开头说样例"在真实的客户端上照人工清单走过一遍"，而 P5 合并时没有走过。直到 v0.1 的最后，读者都会以为样例测过了，包括那个一旦出错就会把 PAT 写进会提交的文件的样例（`--scope project` 的命令）。4.2 也没有一行管它 | 写过渡的说法："照两个客户端的文档与 M0/P1 的实验（Claude Code 2.1.285、Codex 0.157.1）写成，还没有在真实的客户端上照人工清单走过"，并说明两种标记。4.2 的第一行管它：清单跑过之后写明日期与两个客户端的版本（4.1、4.2、4.3） |
| B-M1 | Minor | Caddy 的按路由的 `request_buffers` 现在就定得了。`caddy:2.10-alpine`（2.10.2）本机就有；B 在隔离的容器里跑了 I3 的 Caddyfile：缓冲的三个路由的上游 `duration` 约 1 ms，上传与导入约 3.85 秒。留到 I3，等于默认的代理到 v0.1 的最后都没有保护 | 照主持者的决定（第 2 条）现在定下：<br>- 作者照 B 的 `run.sh` 重跑，量缓冲在哪里，并 `caddy validate` README 的样例（"复核的实验"）；<br>- 5.1 写出 Caddy 的样例，带"在内存里、在认证之前"的一句。这句是本机量的（"作者的更正"第 2 条）；<br>- I3 改为在负责人的部署上确认；<br>- 5.2、7.1 的 M12 一行、7.2 的 v0.1 第 11 节、2.1 第 9 条与新的第 13 条随之；<br>- S1 合并之前重跑这个脚本：9.3 照抄 Caddyfile 与 `run.sh`，写进第 8 节第 12 步与第 10 节 |
| B-M2 | Minor | 导入的 `location` 的 `client_max_body_size 512m` 比服务端收的少 64 KiB：请求体是 zip 加表单，见 `start_import.go:24` 的 `Envelope` | 改为 `513m`，注释与 README 写"导入包的上限加 64 KiB"。上传的大小（`asset.max_bytes` 加 64 KiB，52,494,336 字节）也在其内；管着的是导入的上限加 64 KiB（536,936,448 字节），`validate.go:291-292` 让它不小于前者（5.1、5.2、2.1 第 11 条、7.2） |
| B-M3 | Minor | `location /` 的 nginx 的 1 MB 只在默认时等于 `server.max_body_bytes` | 样例写 `client_max_body_size 1m;`，注释"等于 server.max_body_bytes（默认 1 MiB），改了它时一起改"；2.1 第 11 条与 5.2 写"默认的" |
| B-M4 | Minor | 样例靠继承，本地的改动或升级会悄悄打破它：别处的 `proxy_request_buffering off` 会被继承；在一个 `location` 里加 `proxy_set_header`，它就不再继承 `server` 里的四行 | 两个缓冲的 `location` 显式写 `proxy_request_buffering on;`；样例之后加一句说 `proxy_set_header` 的继承（5.1、5.2） |
| B-M5 | Minor | 没有一条升级的说明让已有的部署改代理 | 升级的说明加 M9 的一条，在 `README.md:334` 之后、`:336` 之前（5.1、5.2、7.3） |
| B-M6 | Minor | 剩下的风险说慢的发送者占着名额"至多 30 秒"，照 2.1 第 9 条是约 45 秒 | README 写"至多约 45 秒"（请求体 30 秒，之后调用 15 秒）；2.1 第 9 条与 7.3 同样 |
| B-M7 | Minor | "管理员可以撤销那个 PAT 或结束那个会话"：没有这样的管理员的路。撤销 PAT 只撤自己的（`revoke_api_token.go:12-15`），管理员命令按邮箱 | README 写：由 `api_tokens`、`auth_sessions` 那一行的 `user_id` 找到账户，请他撤销，或以 `users reset-password` 撤销这个账户的全部会话与 PAT。00 号文档 `:612` 与 M12 的 `M4-performance.md:95` 的同一句一并改正（2.1 第 9 条、7.3、5.1、5.2） |
| B-M8 | Minor | 两个客户端一个环境变量，同时用两个客户端的人就只有一个 PAT，历史里分不清是谁：调用无状态，记下的名称是最后握手的那个 | 照主持者的决定：样例与清单里每个客户端一个变量（`NERVEWIKI_CLAUDE_TOKEN`、`NERVEWIKI_CODEX_TOKEN`），加一句理由；4.3 说明；2.1 第 13 条改 4.16 的 `${NERVEWIKI_TOKEN}`；改 7.2 的 6.3 |
| B-M9 | Minor | 4.2 的"没有一句是没核对过的客户端的行为"不成立。没核对、也不在表里的有：审批与注解、"不再问"、项目的服务第一次先问、`MAX_MCP_OUTPUT_TOKENS`、405 一行 | 同 C-M3 一处改：README 里客户端的每一句在样例里、标着"照文档"或"未核对"，或由仓库的记录核对过；4.2 每句一行，修订时 14 行（第一轮复核之后 15 行，第二轮复核之后 16 行）。另外：<br>- 4.2 的结语改写；<br>- 审批的一句改写，不再说客户端照注解；<br>- "例如 Claude Code"换成 P1 的试用实际跑过的 SDK 的客户端。<br>改在 4.1、4.2 与第 10 节 |
| B-M10 | Minor | 429 一行只说令牌的桶，而失败闸门按 IP 答同样的 429 | 429 一行加上失败闸门（按 IP），以及反向代理之后要设 `server.trusted_proxies`（4.1、4.3） |
| B-N1 | Nit | 4.1 的 `read` 写"中文约 1.5 万字"，清单 C7 写"约 1.3 万个汉字"，同一个界线两个数 | 两处用一个数"约 45 KiB（中文约 1.5 万字）"，由 `read.go:60-92` 与 `answer.go:25-60` 推出（4.3）。作者另更正了 B 的算术，数照旧（"作者的更正"第 3 条） |
| B-N2 | Nit | HTTP 的表没有 SDK 自己的纯文字的 400、415 | 加一行（SDK `streamable.go:347-398`）（4.1、4.3） |
| B-N3 | Nit | `server.public_url` 只在 `read` 一行的括号里 | 写进"先准备"（4.1） |
| B-N4 | Nit | 没有一句说只经 HTTPS 连 `/mcp` | "先准备"加一句；nginx 的样例加 `# listen 443 ssl; …`（4.1、5.1、5.2） |
| B-N5 | Nit | "事件流经下面两份配置都验证过"：M5/P2 验证的是一个 `location /` | 改为"事件流经过的那一部分与验证过的配置是同一组指令"（5.1、5.2） |
| B-N6 | Nit | nginx 把缓冲的请求体写到临时的磁盘上，在服务端认证之前 | MCP 一条写明：nginx 的磁盘，照 nginx 的文档；Caddy 的内存，量过（5.1） |
| B-N7 | Nit | `export NERVEWIKI_TOKEN=nwk_pat_…` 会进 shell 的历史 | 改为 `read -rs … && export …`（README 与清单） |
| B-N8 | Nit | 审批只管 MCP 的工具：能执行命令的 agent 读得到环境里的令牌，可以直接调 REST | 加这一句（4.1、4.3）。（第一轮复核更正：客户端的一半标"未核对"，4.2 给它一行，C2-Mc） |
| B-N9 | Nit | 5.2 的"附件的列表与下载经 `location /`"：没有附件的列表 | 改为下载经 `location /`；导入的列表落在不缓冲的 `location`，没有请求体，无害（5.2） |
| B-N10 | Nit | "读超时不短于 45 秒"随两个设置 | 加"改了这两个的，不短于它们的和"（5.1） |
| B-N11 | Nit | nginx 的样例在事件流的一条里，它却是整个站点的配置 | 样例自成一条"样例（整个站点）"（5.1、5.2） |
| B 过度设计 1 | — | 上限表的说明一行重复了 README 的"MCP"一节 | 采纳，改为一个指向（4.1、4.3） |
| B 过度设计 2 | — | HTTP 表的 405 一行：运维从来不必照它做什么 | 采纳，去掉，理由在 4.3 |

B 对作者的四个矛盾都同意：
1. README 只有一个 `location /`，同时容不下上传不缓冲与 `/mcp` 缓冲；
2. 4.4 的"Claude Code 显示'HTTP 429 …'"没有核对过；
3. "样例出自人工清单的那一遍"在 P5 合并时做不到，README 的开头因此也错了（B-I1）；
4. 第 10 节的"占到读完请求体"与 `largebody.go:166-172` 不符（B-M6 是它的后果）。

### C：人工清单、设计的修订与完整

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| C-I1 | Important | C 部分两个客户端在同一本笔记本里各做一遍。第一个改过了页，第二个的 C1–C6 达不到预期，那些"不通过"会被读成客户端的错 | 照主持者的决定：一个客户端一本，`MCP 清单 CC` 与 `MCP 清单 Codex`，`seed.mjs` 各跑一次，两个客户端的配置里分别写 `$NB_CC`、`$NB_CODEX`。每个说到笔记本的步都核对过（A5、A6、B3、C、F、H1、I1、I2）（6.1 准备 2、4–6） |
| C-I2 | Important | F1 照写的产生不了它的 503：curl 的占位约 20 秒就答完，而 agent 生成约 2 万 token 的参数要几分钟，调用到时名额是空的 | 照主持者的决定取 C 的第一种：<br>- 占位是一个循环（`--limit-rate 40k`，每轮约 26 秒，短于 30 秒的读超时）；<br>- agent 的调用发出时它要在跑，在服务的日志里以带 `own: true` 的 `large request bodies are saturated` 一行看到；<br>- 这一步的最后 `kill %1`。<br>改在 6.1 F1。（第一轮复核更正：循环要带刹车，C2-Mb；第二轮复核更正：命令挪到表下的代码块，C3-Nb） |
| C-I3 | Important | 种下的 `AGENTS` 页在笔记本入口整个取代内置的说明。F1–F2 要看的"大的写一个接一个"那一行只在内置的说明里，工具的说明里没有 | 照主持者的决定：<br>- F1–F3 在账户入口上做，它的握手留着内置的几行；agent 先 `open_notebook`；<br>- F 的开头说 agent 有哪些说明；<br>- 步骤的开头说明，C 的"照说明"的几步靠的是工具的说明（在 `tools.json` 里核对过），在笔记本入口上照样成立。<br>改在 6.1 |
| C-I4 | Important | 负责人自己的 Codex 其间若刷新过令牌，`auth.json` 的复制回去不安全，会把他登出；"做完"又是先删临时目录、再复制回去 | 照主持者定的次序：<br>1. API 密钥（推荐）。<br>2. 在临时的 `CODEX_HOME` 里另 `codex login`，之后记下负责人自己的 Codex 还能不能用（第二次登录影响不影响第一次，未核对）。<br>3. 最后才复制 `auth.json`：复制时记下 `shasum`；只在原来的仍是那个指纹、临时的变了时复制回去；两个都变了时重新 `codex login`；复制回去在删临时目录之前；其间不在别处用 Codex。<br>"做完"的次序随之改正（6.1、第 11 节） |
| C-M1 | Minor | F2 的 503 假定客户端同时发两个写，而两个客户端都没有这样做的记录。两个约 2 万 token 的参数也可能过了输出的上限 | 预期按日志里的两个请求重叠与否分开写；每个写 67,500 字节；限速的配置有名字（`$CC/throttled.json`、Codex 的地址），并重启客户端（6.1 准备 5、F） |
| C-M2 | Minor | 4.2 把 48 KiB 的 `read` 那一句指到 C5（重叠的 `aa`） | 改为 C7，`grep` 的 48 KiB 另见 C11（4.2） |
| C-M3 | Minor | README 的几句客户端的行为既不在 4.2，也没有步核对，第 10 节的标准因此做不到 | 同 B-M9 一处改；第 10 节的标准改为"4.2 的表之外没有客户端的行为"（4.2、第 10 节） |
| C-M4 | Minor | D 部分每个注解的类只试一个工具、只试默认的许可模式，却没说 | D 的开头写明按类各取一个；D1、D4 加 `grep`；负责人记下自己的 `defaultMode` 与 `mcp__` 的规则；6.2 说明为什么只试默认的模式，"不在这份清单里的"列出别的模式（6.1、6.2） |
| C-M5 | Minor | Claude Code 的隔离部分靠没核对过的行为：`--strict-mcp-config` 是否也挡住插件与连接器，`--mcp-config` 的文件展不展开 `${…}`。会话的记录也写在 `~/.claude/projects/` | A1 预期 `/mcp` 只有 `nervewiki`，否则记下；401 时改用写着字面的令牌的文件（`chmod 600`，做完删掉）；隔离一节写明会话的记录在哪里、之后可以删（6.1） |
| C-M6 | Minor | A3 让负责人自己拼项目范围的命令，而 README 唯一完整的命令写的是 `~/.claude.json` | A3 写出完整的命令；README 给项目范围的命令自己的代码块，同 B 的一处（6.1 A3、4.1、4.3） |
| C-M7 | Minor | Codex 的工作目录没说；两个 agent 都可能用自己的本地文件工具来答 | Codex 在自己的空的临时目录 `$CX` 里起；每个会话以一句话开头，说页在 Nerve Wiki 的这本笔记本里（6.1） |
| C-M8 | Minor | 重启 dev 的实例会让两个浏览器都登出：dev 每次启动换一把签名私钥 | 准备 1 以 `openssl genpkey` 生成一把，导出 `NWIKI_AUTH__JWT__PRIVATE_KEY_FILE`；重启之后浏览器照样登录着（6.1） |
| C-M9 | Minor | C10 的"小"图可能不小；`wide.png` 的"约 2–3 MiB"贴着 3 MiB 的内联上限 | `small.png` 每边不过 1,000 像素、几十 KB；`wide.png` 2,500–7,000 像素、2.0–2.8 MiB，并写明怎样查（6.1） |
| C-M10 | Minor | I 部分没有负责人能直接跑的命令 | 给出两条 docker 命令：nginx（回环的端口、`conf.d`、四个 `location` 的上游都换成 `host.docker.internal`）与 Caddy（I3 在本机的替代）；做完按名字停下（6.1 I、做完） |
| C-M11 | Minor | E4 把 A 的密码写在命令行上 | 改用 `read -rs` 与 `jq`，命令在表下的代码块（6.1） |
| C-M12 | Minor | 清单没说怎样算"通过" | "结果"一行写通过的规则（6.1） |
| C-M13 | Minor | 2.1 第 10 条留着第 3 节第 13 条（`:67`）说样例出自清单；2.2 也没说保留原写法的那条路（先跑一遍） | 2.1 第 10 条也改 `:67`；不先跑清单写作主持者的决定，负责人可以改判；2.2 照旧为空，指向它（主持者的决定第 4 条） |
| C-M14 | Minor | M9 收尾时关掉 M0/P1 的移交，等于在量之前就关了 | 那一行与那份移交在清单跑过之前都是 open（7.1、第 11 节、2.3） |
| C-M15 | Minor | 给 M10、M12 的移交没有放进它们的 `handoffs/` | 第 11 节逐份写出文件与文字，P5 的文档提交写进去：<br>- M12 `M5-polish.md` 第 31 项；<br>- `M7-transfer.md:29` 的部署一条；<br>- `M4-performance.md` 第 18 项的三处与一条新的；<br>- M10 `M9-P1-mcp-registry.md` 的第 6 项，并写明为什么选它。<br>改在第 11 节、7.1、7.3、第 10 节 |
| C-M16 | Minor | P4 的"截取的行"被合并掉了：没有一行种下的文字给出 `cut` 的命中 | `命中.md` 种第 601 行：约 3,000 字节，中间有 `FIXME`，行尾另有可辨的字；新的一步 C15（6.1） |
| C-N1 | Nit | H1 的"C14 之后是 `Guide/已锁.md`"：C14 以网页的整组撤销结束，那一页又是 `Guide/锁.md` | 照改，C14 自己也写 `Guide/锁.md`（6.1） |
| C-N2 | Nit | E2 的 `/token` 是 POST，网页对 GET、HEAD 之外的方法答 405 | 写"GET 是网页的 200 `index.html`，POST 是 405 `Allow: GET, HEAD`"（6.1、附录） |
| C-N3 | Nit | B2 也要记下 Codex 调没调工具 | 照改（6.1） |
| C-N4 | Nit | D4–D7 的"换成……"与 E3–E5 要说退出、改、再启动 | 照改（6.1） |
| C-N5 | Nit | H3 不带 `recursive` 时先答数的拒绝（先数后锁） | 预期写明（6.1） |
| C-N6 | Nit | 行号：AS4 一行 `:589`、TB 一行 `:592` | 照改（同 N-A6）（2.1、7.1） |
| C-N7 | Nit | 2.1 第 9 条引的 M12 移交的话是 P2 第 8 节的，不是移交自己的 | 照移交的原文引（2.1 第 9 条） |
| C-N8 | Nit | 7.1 以省略号引 M0/P1 一行，脚本要匹配的是全文 | 照全文引（7.1） |
| C-N9 | Nit | v0.1 13.4 第 3 条留着"M9/P2 的 S1 加 MC3 的 MCP 版本与活动"；00 号文档的 13.4 第 3 条（`:493`）没有 PG7、PG8 的例外 | 7.2 的 13.4 第 3 条以"MC3 的活动由 P5 挪进 PG14 (MCP)"结尾；2.1 第 3 条也改 `:493` |
| C-N10 | Nit | "做完"可以加：`unset` 令牌的变量，停下 `make run`，说明会话的记录可以删 | 照改（6.1） |
| C-N11 | Nit | 2.3 的 M0/P1 一行：第 1 项的后一半（`SessionTimeout`）不会出现，调用无状态 | 写明（2.3） |
| C-N12 | Nit | 准备 2 没说 B 怎样加入 | 写明：A 在成员页邀请，B 在另一个浏览器里注册并接受（6.1） |
| C-N13 | Nit | 隔离一节的"复制回 `~/.codex/auth.json`（它变过时）"里，"它"读起来像临时的那份 | 重写那段，没有含糊的代词（6.1） |
| C 过度设计 | — | 2.3（按来源）与 6.2（按步）是同一个对照写了两遍 | 采纳：去掉 6.2（原 6.3 成为 6.2）；2.3 按来源带着步的编号；第 6 节的开头说为什么 |

C 的几个结论：

- **第 11 节负责人的决定**：草稿与它们没有矛盾。两份清单在 v0.1 的最后跑，M9 在那之后才完成；体验的打磨在 v0.1 之后；agent 的历史只读；锁不能接管；`AGENTS`；`search` 与两个客户端的文档。
- **作者的矛盾**：在 C 核对过的地方都成立，即过时的行号、`collab.ts:50`、名额的一句、TB 的记账、豁免的两类、4.4 的 429、样例、README 的一个 `location /`、13.4 第 3 条的 MC5–MC7、MC3 就是 PG14 的形状。
- **2.1 的旧文字**：十二条的每段原文在 `7dc4a686` 恰好出现一次。

## 重叠

| 发现 | 也是 | 合在一处的处置 |
|---|---|---|
| B-M9 | C-M3 | README 里客户端的每一句在样例里、标着"照文档"或"未核对"，或由仓库的记录核对过，4.2 每句一行；第 10 节的标准是"4.2 的表之外没有客户端的行为" |
| B 的项目范围的代码块（B-M9 之下） | C-M6 的后一半 | README 给 `--scope project` 的命令自己的代码块，清单的 A3 写出完整的命令（4.1、6.1） |
| N-A6 | C-N6 | `:589`、`:592`（2.1 第 7、8 条，7.1） |
| H3-7 | A 的豁免一节的 C6 一句 | "agent 的删除"（3.8、第 0 节、PG4 第 4 步） |

## 主持者的决定

主持者在四份修订的简报里定下处置，作者照做，负责人可以改判。这一节是第一份简报的，第 1–10 条；后三份的在各轮复核的一节里，第 11–23 条。其中经 Caddy 时早答的拒绝成了 502 的那一件，处置是留着 Caddy 的样例、P5 不加服务端的代码，服务端怎样改交 M12（第 12 条）。

1. **四处可改判的取舍都维持**：
   - PG14 的 MCP 版本由 MC3 的活动挪来，不另写一遍（2.1 第 5 条）；
   - C7 的 MCP 版本以事件流的帧断言（第 6 条）；
   - PG7、PG8 以 `expectVersions` 代会话的断言（第 3 条），加上 A-2；
   - 反向代理的样例按路由（第 5 节）。

   负责人可以改判（2.2）。
2. **Caddy 现在定下**（B-M1）：
   - 5.1 现在就写 Caddy 的样例；
   - 加一句 Caddy 把请求体缓冲在内存里、在认证之前，标"照 Caddy 的文档"还是"未核对"，取能引的那一个；
   - I3 改为在负责人的部署上确认；
   - 5.2、7.1 的 M12 一行与 7.2 的 v0.1 第 11 节随之；
   - 第 9 节写 S1 的实施者在合并之前重跑 B 的脚本（`run.sh`，经 `heavy.sh`，`--pull=never --network none`），记下用时；
   - B 的 Caddyfile 与脚本复制进 `scratchpad/m9/p5/exp/caddy/`，并引它们。
3. **L6 (API) 与 C7 (API) 在同一个提交里修**（A-5、A-7）：它们是已有的测试的洞，与 MCP 版本的是同一个，修法各一两行。
4. **不先跑人工清单**（C-M13，2.1 第 10 条）：这是主持者照负责人的规则定的，规则是 v0.1 的最后只验收一次（第 11 节的取舍第 12 条）。2.1 第 10 条的理由写明它是可以改判的决定；2.2 照旧为空，除非有事真的要负责人。
5. **Codex 的登录的次序**（C-I4）：
   1. 临时的 `CODEX_HOME` 里用 API 密钥，推荐；
   2. 其次在临时的 `CODEX_HOME` 里另 `codex login` 一次，之后记下负责人自己的 Codex 还能不能用，第二次登录影响不影响第一次标"未核对"；
   3. 最后才复制 `auth.json`：复制时记下原来的 `shasum`；到最后只在原来的仍是那个指纹、临时的又变了时才复制回去；两个都变了时重新 `codex login`；复制回去在删临时目录之前；其间不在别处用 Codex。

   "做完"的次序与 C-N13 的代词一并改正。
6. **清单的另外三个 Important**：
   - C-I1：一个客户端一本笔记本，`seed.mjs` 各跑一次；每个说到笔记本的步都要核对，C14、H1 在内。
   - C-I2：取循环的占位（C 的第一种）。F1 写明 agent 的调用发出时它要在跑、怎样在服务的日志里看到，这一步的最后按作业号停下它。
   - C-I3：F1–F3 在账户入口上做，agent 先 `open_notebook`，每个 F 步写明 agent 有哪些说明；核对别的"照说明"的步在笔记本入口上照样成立。
7. **其余的照审查者的建议**，其中几条带着具体的做法：
   - A-1：导出 `PageRead`，写明实施者怎样证明第 1 步编得过（`make lint`）。
   - A-8：第 8 节第 8 步写明服务端的变异先 `make build`；语句的变异改在 `gen/*.sql.go`，从不在带着变异时 `make gen*`，从不提交。
   - A-9：去掉第 224 条的反向对照的说法；`writtenOf` 的检查以伪造的答复证明一次，记进审查记录，不提交。
   - A-12：第 2 步读 A 的子页，加只有 MCP 的变异。
   - A-16：`marked` 仍是必须的，在 MCP 的调用处不比，`foundOf` 上做反向对照。
   - A-21：留着 REST 的读，3.1 的规则不变。
   - A-23：PG4 (MCP) 第 4 步加 `expectTrashRoot(…, {alone: true})`，TB 的记账挪给它。
   - B-M8：每个客户端一个变量，一句理由，清单随之。
   - B-N1：README 与 C7 用一个由代码推出的数。
   - B 的两条过度设计都采纳。
   - C-M3：第 10 节的标准放宽为"4.2 的表之外没有客户端的行为"。
   - C-M6：A3 写出完整的命令，README 给它自己的代码块。
   - C-M14：M0/P1 的一行与移交在清单跑过之前都是 open。
   - C-M15：第 11 节逐份写出文件与文字，P5 的文档提交写进去；给 M10 的另开一份还是加进 `M9-P1-mcp-registry.md`，作者选一种并写明为什么。
   - C-M16：种下截取的行，加它的一步。
   - C 的过度设计：2.3 按来源带着步的编号留着；6.2 去掉，或由清单的步号生成，作者选（作者去掉了它）。
8. **过渡的写法只有一条规则**：README 与清单里客户端的每一句，在样例里、标着"照文档"或"未核对"，或由仓库的记录核对过；4.2 逐句列出。
9. **每条新的变异记进 9.2**：它的位置（`7dc4a686` 的 `file:line`）、要抓到它的故事与步，以及 API 版本抓不抓得到。
10. **规则**：
    - 仓库只读，只在 `scratchpad/m9/p5/` 之下写，不下载；
    - 不运行任何 MCP 客户端，不碰负责人的客户端配置；
    - 帮手用 Opus，边写边存；
    - Docker 只经 `heavy.sh`，只为 Caddy 的重跑。

## 作者的更正

修订时作者自己核对出的、与审查或简报所料不同的：

1. **`make e2e` 先构建**（A-8）。A 写的是"`make e2e` 与 `playwright test` 跑 `bin/nervewiki`"，所以变异之后要先构建。实际上 `e2e: build`（`Makefile:80-81`）会先重建，只有直接的 `pnpm exec playwright test <文件>` 跑上一次的二进制，让服务端的变异读作"没抓到"。第 8 节第 8 步照这样写。A 在第一轮复核里认为这个更正对。
2. **Caddy 的内存是量出来的**（B-M1）。简报让作者在"照 Caddy 的文档"与"未核对"里取能引的那一个，两者都不对：作者在本机的 Caddy 2.10.2 上量过。`exp/caddy/mem.sh` 把 30 MB 的请求体以每秒 3 MB 发到 `/mcp`，`mem-out.txt` 里 VmRSS 由 43,684 kB 涨到第 6 秒的 109,252 kB，`/tmp`、`/var/tmp` 下没有新的文件。所以 README 写"我们在 Caddy 2.10.2 上量过"。B 在第一轮复核里认为对，并在分开的进程里量得 30 MiB 涨约 66 MB。（第一轮复核更正：`mem-out.txt` 的"之后"的 220,236 kB 算着同一个进程里的上游与它回显的 30 MB，B2-N1）
3. **B-N1 的算术**：B 按每行 32 个字算；种子的 `长文.md` 每行是"`第 N 行：`"加 24 个字。数"中文约 1.5 万字"照旧。B 在第一轮复核里认为对：这一页 69,492 字节、约 2.16 万个汉字，第一次 `read` 约 560 行、约 1.5 万字。
4. **草稿自己的几处**，修订时找到：
   - MC7 抓不到第 241 条（它的两页直接标着 `plan`，`mc7:90-92`），L4 (API) 抓得到；
   - 上传的 `parent_id` 为空时被拒，要省掉不写（`asset/adapter/http/upload.go:143-149`），清单照这样写；
   - `长文.md` 约 69 KB、约 2.2 万字，不是 65 KB、1.9 万字；
   - MC2 的 `PageRead` 在 `:64-73`；
   - P1 的"没有令牌时"一段的"例如 Claude Code"是推断：P1 的试用跑的是官方 TypeScript SDK 的 OAuth 客户端，README 照实写，Claude Code、Codex 各走哪一条标"未核对"。
5. **2.1 新的第 13 条**：B-M8 与 B-M1 改的两件事，4.16 的接入文档一条（`:384`）里也写着：`${NERVEWIKI_TOKEN}`，与 Caddy 的"核对过才写"。00 号文档第 10 节（`:612`）的 Caddy 的一句同样要改。作者加了第 13 条，并改了 2.1 第 9 条。

复核的几轮里作者的更正，写在各轮的一节：B2-M1 的表里 nginx 的两格没有量过（第一轮复核）；I3 不再说 401 的 502 没有 `WWW-Authenticate`（第二轮复核）。

## 审查者核对过、没有问题的

- **A**：
  - **模型**：种下的数据经 REST，每个 agent 一个 PAT，不开浏览器，值来自答复。`at` 是单元的一次时钟（`page/app/unit.go:250`、`:556-559`）。草稿没有一处由库造出期望的值、再与库比。
  - **fixture 的四处改动**：逐个调用者核对过，`expectContentWritten` 有 PG5、PG7、PG8、PG9、PG10、C10 与经 `expectEmbedded` 的 AS2，都只读那三个字段。
  - **PG7、PG8、PG14、C7 的细节**：帧的每一种形状都对；一个 batch 是一个单元、一次 `tell`；握手不写帧；`edit` 的 `old_string` 至少 1 字节。
  - **第 1 节的工具与注解**：笔记本入口 14 个、账户入口 16 个，九个读加 `notebooks`、`open_notebook` 是 `readOnlyHint`。
  - **2.1**：第 1、4、5、6 条成立。
  - **3.8 的数**：49、22、27。只有 HI4 调 `expectDeletionRevisions`。
  - **变异在写的那一步被抓**：H1 的 #223、#225、#226、#228、#231；H2 的 #232、#233、#239、#240、#246；H3 的 #242；#236 在 C7 (MCP) 第 3 步。
  - **Step**：除第 1 步（A-1）之外，每个 Step 自己是绿的，过 knip。
  - **e2e 那一份的余量**：21 个测试在本机各约 0.3–1 秒，CI 上多约 5–20 秒；3:50–4:15 的估计保守，在 20 分钟之内。不加 Go 的测试，组合根的分份不变。
  - **flake**：每个测试自己的团队；PAT 的名称不重复用；测试的限流实际不限；15 秒的期限够。
- **B**：
  - **工具与注解**：`tools.json` 逐个打印核对过。
  - **上限的数**：37,814,272 = 6 ×（5 MiB + 1 MiB）+ 64 KiB（`bootstrap/mcp.go:29`）；31,522,816；`37m`、`31m` 都不小于它们；10,000 个值；48 KiB；15 秒从读完请求体起；两个名额、每个凭证一个、2 秒、`Retry-After: 1`。
  - **HTTP 的答复**：文字逐字对着 `edge.go`、`api.go` 核对过。
  - **curl 的自测**：与 e2e 的客户端用同样的头，不握手，不改这个令牌记下的名称。
  - **写的规矩、先准备、令牌在每个样例里的去处、审批**：与 v0.1 6.3 一致。
  - **nginx 的样例**：`nginx -t` 通过（nginx 1.29.8）。按路由跑过一次：`/mcp` 与 `/mcp?notebook=…` 落到 `= /mcp`；两个正文路由落到它们的 `location`；上传与导入不缓冲；`/mcp/`、`/MCP`、`/.well-known/…`、`/register`、事件流与网页的路径落到 `location /`，2 MiB 时得到 nginx 自己的 413；`Host` 都来自 `$host`。
  - **路由的全部**：大于 `server.max_body_bytes` 的请求体只在两个 `BodyLimits` 的路由、`/mcp` 与两个流式的上传上，三个特别的 `location` 就是全部。
  - **README 仍是一份文档**：新的一节的位置、5.1 替换的行与交叉的引用都对。
- **C**：
  - **清单期望的答复对着代码**：两种 401、403、`/register` 的 400、`/.well-known/` 的 404、`page.locked` 的三种文字、`found 2 times`、先数后锁、14 与 16 个工具、内联图片的 3 MiB 与 8,000 像素。
  - **日志与环境**：准备第 1 步说的日志行与字段都在；`NWIKI_` 与 `__`、`make dev-db`、`make build`、`make run`、`/settings/tokens` 都在；三个 PAT 在 `password_user` 的限流之内；登录答 `access_token`。
  - **`seed.mjs`**：对得上 API。
  - **C12**：`GrepLineCut` 约 2.2 MB，与作者的 `exp/insts` 数出的 34 条指令一致。
  - **说明**：4,027 与 3,957 字节，C 独立地重算过。
  - **其余**：PAT 跨过重启；I1 经 nginx 用浏览器可以；每一步有"记下的"一栏；G1 的交互的重启与 F2 的大小这两处偏离成立；清单的相对链接都解析得到；2.3 处置了 P1–P4 第 8 节与"移交的落实"给 P5 的每一项，M-4、M-14、M-16 除外。

## 未采纳

没有。下面几条有不止一种做法，取的都在审查者给的选项之内：
- A-5 取 settle；
- A-21 取留着 REST 的读；
- A-23 取 PG4 (MCP) 加 `expectTrashRoot`；
- C-I2 取循环的占位；
- C 的过度设计取去掉 6.2。

## 00 号文档与别的文件的改动

- **`apply-00m9.py`**（`scratchpad/m9/p5/apply-00m9.py`）随设计的提交落下：2.1 的 13 条、第 12 节的 P5 一行（"进行中（设计已审查）"）与第 13 节的一行，共 21 处改动。
  - 合并：第 1、4 条都在 `:539`，合成一处；第 2、3、8 条都在 `:557`，合成一处。
  - 分开：第 3 条另有 `:598` 与 `:493`；第 5、10、13 条各两处；第 8 条另有 `:592`；第 9 条四处。
  - 规则：每段原文恰好出现一次才改，有一处对不上就什么也不写。
  - 核对：以 `--check` 对着仓库的文件（`7dc4a686`）与 `HEAD` 的副本，都是"all 21 edits match"。另在 scratchpad 的副本上真的写了一次（`exp/00-M9-applied.md`，改了 16 行，含第 13 节新的一行）。只核对，没有写仓库。
- **不在设计的提交里的**：
  - 7.1 的 S1 合并之后的各处：第 12 节的 P5 一行改为"完成"、第 13 节合并的一行；"移交的落实"的 M0/P1 一行加清单的步；"写给后面的 M"的 M12 两条；"总体设计的修订"的 10.1 与 11 两条写成已落下的样子。
  - 7.2 的 v0.1 总体设计的五处：6.3、10.1、11、13.4 第 3 条与第 15 节，由 `apply-v01.py` 落下。
  - 7.3 的 M12 的性能移交第 18 项的三处。
  - 第 11 节的四份移交：M12 的打磨、转移、性能，M10 的 MCP。

  它们都在 S1 合并之后的文档提交里落下，那时照实际合并的样子写；设计的第 7 节开头、第 8 节第 13 步与第 10 节的"文档"一段都这样说（第三轮复核之后，第 22 条）。`apply-v01.py` 也在那时写，照样先以 `--check` 核对。

## 新的问题

要主持者看一下的几件：

1. **经 Caddy 时缓冲在内存里，在认证之前**（B-M1）：几个慢的、不带有效令牌的发送者，就能让代理各占一百多 MB。README 照实说，压测时要量它（第 11 节给 M12 的性能移交）。（第一轮复核更正：分开进程时，30 MiB 的请求体涨约 66 MB，接近 `/mcp` 上限的 36 MiB 涨约 116 MB，B2-N1）
2. **没有按令牌或会话撤销的管理员命令**（B-M7）：P5 只改正 README、00 号文档与 M12 的移交的文字。加不加这样的命令不在 P5 的范围，记下。
3. **README 带着"照文档""未核对"的标记，直到 v0.1 的最后**：这是不先跑清单的代价（主持者的决定第 4 条）。负责人改判时，实施者在合并之前照清单跑一遍，README 照 4.2 去掉标记、写上核对过的版本。
4. **2.2 照旧为空**。

## 复核的实验

都在 `scratchpad/m9/p5/exp/` 之下，不下载。容器只经 `heavy.sh` 起，`--rm --pull=never --network none`，没有映射端口，只用本机已有的 `caddy:2.10-alpine`。审查者的 `review-A/`、`review-B/`、`review-C/` 原样留着。

- **按路由的缓冲**（`exp/caddy/Caddyfile`、`run.sh`、`run-out.txt`）：B 的 `review-B/caddy/` 的副本，照原样重跑。200,000 字节的请求体以 `--limit-rate 50k` 发出，每个约 4 秒。上游的 `duration`：
  - 缓冲的：`POST /mcp?notebook=x` 0.000539 秒，`PUT …/content` 0.000599 秒，`POST …/pages` 0.00197 秒；
  - 不缓冲的：`POST …/assets` 3.8466 秒，`POST …/imports` 3.8396 秒。

  分块的请求到上游时仍是分块的（最后两行）。
- **缓冲在哪里**（`exp/caddy/mem.sh`、`mem-out.txt`）：30 MB 的请求体以每秒 3 MB 发到 `/mcp`，每秒读一次 Caddy 的 `VmRSS`，并找 `/tmp` 与 `/var/tmp` 下新的文件。VmRSS 由 43,684 kB 涨到第 6 秒的 109,252 kB，临时文件一直是 0 个。"之后"的 220,236 kB 算着同一个进程里的上游（第一轮复核更正，B2-N1）。`mem-out-first.txt` 是挂住了的第一次运行（"过程中的一处失误"），它到第 7 秒的数与第二次相同。
- **README 的样例**（`exp/caddy/readme/`：`Caddyfile`、`validate.sh`、`validate-out.txt`）：`caddy validate` 答 `Valid configuration`；`caddy adapt` 出的 `request_buffers` 是 38,797,312，`path_regexp` 是这三个路由。唯一的一行警告是缩进不是 Tab。
- **清单 C12 的模式**（`exp/insts/`）：`\pL{30}完成` 编译出 34 条指令，4 MiB 的一行只匹配得到开头约 2.2 MB，于是进 `partial_lines`。
- **00 号文档的改动**：见上一节。

## 过程中的一处失误

作者在第一轮修订里量 Caddy 的缓冲时出了一次错：

- **留下的容器**：第一次的 `mem.sh` 用了不带参数的 `wait`。它等着后台的 caddy，挂住了，同时持着 `heavy.sh` 的锁。作者以 TaskStop 停了自己的那个后台任务，以为容器会随之退出，可它没有。容器 `5fd97b49c20c`（`caddy:2.10-alpine`，`sh /w/mem.sh`，`--rm`，没有网络与端口）从 02:43 UTC 起一直空跑。作者在交回之前发现它，经 `heavy.sh` 以 `docker inspect` 看过，报告主持者。作者没有自己停它，因为规则是不停进程；主持者停了它。改好的 `mem.sh` 在等之前先停下后台的 caddy，第二次运行正常（`mem-out.txt`）。
- **两次不经 `heavy.sh` 的列表**：都是只读的。一次是 `docker image ls`，看本机有没有 caddy 与 nginx 的镜像；一次是 `docker ps -a --filter ancestor=caddy:2.10-alpine`，找留下的容器，正是它找到了 `5fd97b49c20c`。两次都报告了主持者。
- **之后**：
  - 第二份简报加了两条：列表也只经 `heavy.sh`；结束之前以 `heavy.sh docker ps -a` 确认没有留下的容器（第 15 条）。
  - 第二轮修订没有起容器，结束时的列表里没有作者的容器。
  - 第三轮修订不起容器（第 18 条）。
  - 三位审查者每一轮都以 `heavy.sh docker ps -a` 确认没有留下自己的容器。

## 第一轮复核

| 项 | 内容 |
|---|---|
| 复核对象 | 修订后的 [05-P5-stories-docs.md](../05-P5-stories-docs.md)（2026-10-23，244,663 字节、1,322 行；复核时的样子留作 `05-P5-stories-docs.rev1.md` 与 `parts-rev1/`）与上面的处置，对照 `main` `7dc4a686` |
| 复核方式 | 三位审查者（Opus）各复核自己的一份，只读，不下载，不运行任何 MCP 客户端。<br>A：<br>- 在 `review-A/tc/e2e` 里重跑一次 tsc，这次 `PageRead` 从 `fixtures/mcp.ts` 导出：只剩副本原有的三个错（MC1 经符号链接的 `node_modules` 导入 SDK，改动之前就有）；<br>- 唯一的 Docker 命令是 `heavy.sh docker ps -a`。<br>B：<br>- `review-B/early/`：先对修订后的 nginx 样例跑 `nginx -t`；再让 nginx 与 Caddy 的样例各转发一个上游的早答。这个上游是 Caddy 的二进制（Go 的 `net/http`，同服务端），不读请求体就答 503 `server_busy` 与 `Retry-After: 1`，同 `largeBody` 等不到名额时（`largebody.go:146`、`:166-170`）；量 nginx 时这个二进制从本机的 caddy 镜像拷进 scratch，用完删掉。<br>- `review-B/caddy-mem/`：代理与上游分在两个进程里量 Caddy 的缓冲，一直量到上传的最后。<br>- 都经 `heavy.sh`，`--rm --pull=never --network none`，没有映射端口，只用本机的镜像；结束时 `heavy.sh docker ps -a` 里没有 B 的容器。<br>C：<br>- 新的与改过的每段原文以 `grep -o -F` 对着 `7dc4a686` 数过；<br>- 这一轮不需要 Docker，结束时只跑了 `heavy.sh docker ps -a` |
| 日期 | 2026-10-23 |
| 结论 | **各人的数**：<br>- A：A-1 到 A-23 里 22 条落下，A-6 部分落下（235b）；30 条 Nit 里 28 条落下，N-A3、H1-2 部分落下。新的 Important 0、Minor 3（就是这三处剩下的）、Nit 3。作者对 A-8 的更正 A 认为对。<br>- B：自己的 24 条都落下，作者对 B-M1、B-N1 的更正 B 认为对。新的 Important 0、Minor 1（B2-M1）、Nit 5。<br>- C：自己的 34 条都落下。新的 Important 0、Minor 3、Nit 8。<br>**处理**：新的 23 条全部处理，处置由主持者定下（"主持者的决定（第一轮复核）"第 11–15 条），负责人可以改判。修订直接写进设计，复核时的样子留在 `05-P5-stories-docs.rev1.md` 与 `parts-rev1/`。<br>**经 Caddy 时早答的拒绝成了 502**（B2-M1）。服务端在读请求体之前就答的拒绝（503 `server_busy`、401、429），对几 MiB 以上的请求体经 Caddy 常常到客户端成了 502、没有 `Retry-After`；nginx 照原样转发。Caddy 的样例留着：它去掉了慢的发送者的风险，502 是变差了的错误信号，不丢数据。P5 不加服务端的代码，服务端怎样改交 M12。<br>**修订之后**：9.2 仍是 223–251；4.2 有 15 行 |

### 发现与处置

第一轮的条目照原样留着，复核更正的说法已就地标出。A 的复核把新的条目写作 M-1 到 M-3、N-1 到 N-3，C 的写作 M-a 到 M-c、N-a 到 N-h；这里加上审查者与轮次（A2-M1、C2-Ma 一类），免得与第一轮的混淆。

#### A

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| A2-M1 | Minor | 照放的位置，235b 分不出 MCP 与 REST 的单元：`linking.PagesChanged` 只带 `WorkspaceID`、`NotebookID`、`Changes`（`linking/app/index.go:13-17`），`bootstrap/linking.go:24-37` 转换页面的事件时丢掉了客户端。若对每个单元都跳过，L6 (API) 也失败，那一行的"API 版本也抓到？不"就错了。A 说这个错从它第一轮的建议起 | 照主持者的决定（第 11 条）取两处的变异：`bootstrap/linking.go:37` 以 `e.Write.Client.IsMCP()` 填 `linking.PagesChanged` 的一个新字段，`index.go:85` 在它为真时跳过 `x.publish`。写明这是测试一侧两个文件的临时改动，从不提交；最后一栏仍是"不"。9.2 另写为什么不取一处的形式：那样 L6 (API) 也失败。改在 9.2 与附录的 L6 一行 |
| A2-M2 | Minor | C7 (MCP) 第 2 步自相矛盾：第一次 `write` 的正文是空的，`edit` 却要替换其中的一行，而 `old_string` 至少 1 字节（`page/adapter/mcp/edit.go:15`） | 照主持者的决定（第 13 条）：第一次写 `"Draft\n"`，`edit` 以 `old_string: "Draft\n"` 与 `base_version "<id>:1"` 做，两次移动不改版本号。帧重核过：建页是 `{tree: true, pages: [{id, revision: 1}]}`，编辑是 `{tree: false, pages: [{id, revision: 2}]}`；自相矛盾的那一句去掉（3.3、附录） |
| A2-M3 | Minor | PG4 第 2 步的"整个文字"少了码的前缀。`text.go:35-38` 写成 `"validation_failed: " + …`，mcpserver 原样把它作 `message`（`mcpserver/errors.go:155-156`） | 照主持者的决定（第 13 条）：整个文字带 `validation_failed: `，`errorOf` 把它与文字块比（`e2e/fixtures/mcp.ts:171-176`）。第 3 节别的文字的期望都重核过：PG2、L3、AS4 是"包含"，PG1 比字段的说明（`page/domain/path.go:70-77`），只有 PG4 比整个文字，3.2 写明这一点（3.2 PG4、附录 `errors.go:155-156`、`:188-190`） |
| A2-N1 | Nit | SE1 的 `{marked: false}` 在 `marked` 必须时要一个类型 | `expectFound` 加一个重载 `(q, found: Omit<Found, "marked">, options: {marked: false})`；`mcpFoundOf` 返回这个类型，从不造假的 `marked: []`；原来的签名不变（3.7、第 8 节第 7 步、9.1） |
| A2-N2 | Nit | M12 的 `M5-polish.md:10` 的"第 30 条 M9"要随新的第 31 项改 | 第 11 节的移交 1 把它改为"第 30、31 条 M9"（那段原文在 `HEAD` 恰好一次）。C 没有另记，一处改 |
| A2-N3 | Nit | C7 (API) 的安静的检查要先把内联的 Marker 提成一个变量 | 照改，两行。版本号不用读：建时是 1，settle 写成 2，安静的写以 `base_revision` 2 做，帧里是 3（3.1、3.3、第 0 节、9.1） |

#### B

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| B2-M1 | Minor | 经 Caddy 时，服务端在读请求体之前的拒绝（503 `server_busy`、401、429）对几 MiB 以上的请求体常到客户端成了 502，没有 `Retry-After`。上游答了、关了连接，Caddy 还在写缓冲着的请求体，报的是写的错误。样例让这件事在三个缓冲的路由上几乎每次都发生。网页的保存只在 503 带着 `Retry-After` 时重试（`web/apps/web/src/stores/page-editing.ts:336`），所以立刻失败；README 的"503……稍后重发"也到不了 agent | 照主持者的决定（第 12 条）：<br>- 留着 Caddy 的样例，P5 不加服务端的代码；<br>- README 在 Caddy 的样例之后加一句；<br>- 5.2 加一条：B 的表、502 的日志、上游是替身的说明、情形与为什么留着样例；<br>- 9.3 的合并之前的重跑带上 B 的探测，复制在 `exp/caddy/early/`、照抄进 9.3；<br>- I3 记下经 Caddy 的一次被拒的大的写显示什么；<br>- 第 11 节给 M12 的性能移交第 18 项一条：502，网页只在 503 带 `Retry-After` 时重试，401 的续期（`auth-middleware.ts:45`）；服务端的改法交 M12，只是一种可能；<br>- 2.1 第 9、13 条与 7.2 的 11 带同样的一句。<br>作者更正了 B 的表的 nginx 的两格（"第一轮复核的作者的更正"第 1 条） |
| B2-N1 | Nit | README 的"各占几十 MB"对最大的请求体少说了：分开进程时，36 MiB 的请求体涨约 116 MB，30 MiB 涨约 66 MB；5.2 的 220,236 kB 算着同一个进程里的上游 | README 写约 66 MB 与约 116 MB，以及"至多约 37 MiB 的磁盘（nginx）或一百多 MB 的内存（Caddy）"。5.2 加上分开进程的那一次（`exp/caddy/mem-separate/`）与 Go 的缓冲过了 32 MiB 再翻倍，并说明 220,236 kB 是怎么来的 |
| B2-N2 | Nit | 升级的一条的"不改时 `/mcp` 照常能用"只对设了 512m 的部署成立；"照下面'反向代理'的样例"没有这个标题 | 照改：指名"部署一节的'样例（整个站点）'"，分开写照旧的 README 设了 `512m` 的与留着 nginx 默认的 1m 的（后者过 1 MiB 的 MCP 调用得到 413）（5.1） |
| B2-N3 | Nit | "自己试一下"的 curl 只用 `$NERVEWIKI_CLAUDE_TOKEN` | 加"（只给 Codex 设了令牌的，换成 `$NERVEWIKI_CODEX_TOKEN`。）"（4.1） |
| B2-N4 | Nit | 第 11 节 `M7-transfer.md:29` 的"它大于附件的上限加 64 KiB"应为"不小于"：`validate.go:291-292` 许可两者相等 | 照改 |
| B2-N5 | Nit | "Caddy 默认不限"（读超时）没有标记，同一节的 nginx 的临时文件却标着"照 nginx 的文档" | 写"照 Caddy 的文档"（5.1） |

#### C

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| C2-Ma | Minor | 2.1 第 13 条只换了 4.16 的 Caddy 分句的括号，那一条就会前半说风险照旧、后半说 README 的样例去掉了它 | 照主持者的决定（第 14 条）：原文取 4.16 的整个 Caddy 分句，在 `7dc4a686` 恰好出现一次；新的文字与 README 的样例、2.1 第 9 条一致，含 502 的一句与"直接暴露时剩下的风险照旧"（2.1 第 13 条） |
| C2-Mb | Minor | F1 的循环没有刹车：`$TOKEN` 或 `$NB` 写错时每轮几毫秒，循环空转。错的令牌每轮都耗掉按 IP 的认证失败的闸门（每分钟 60，`config.yaml:74`），一秒之内这台机器上带令牌的请求都答 429；错的笔记本则耗掉这个 PAT 自己的桶 | 照主持者的决定（第 14 条）：<br>- 循环改为 `curl --fail … \|\| sleep 2`，HTTP 的错误之后先停 2 秒；<br>- 预期的开头写"日志里一串很快的 401、404 或 429 时，先 `kill %1`，改正 `$TOKEN`、`$NB` 再起"；<br>- 一句写出闸门的代价：闸门在认证之前按 IP 取，失败的令牌不退还（`api.go:346-350`）；<br>- 6.2 说明这个刹车。<br>改在 6.1 F1 与 6.2 |
| C2-Mc | Minor | 这一轮加的一句客户端的行为不在 4.2，第 10 节的标准差一句。那是 B-N8 的"agent 能执行命令时，它读得到环境变量里的令牌"：没有标记，也没有仓库里的记录 | 照主持者的决定（第 14 条）：继承环境是 shell 的事；客户端把不把环境交给它执行的命令（或滤掉一些变量）标"未核对"，照最坏的写。4.2 加一行："清单不核对：核对要让 agent 读出令牌"，"留作提醒"。4.2 成 15 行，4.3 指向它 |
| C2-Na | Nit | 准备 2 的"两个客户端在同一本里做，后一个会看到前一个改过的页"读起来像要共用一本 | 改为"若两个客户端在同一本里做……所以一个客户端一本"（6.1） |
| C2-Nb | Nit | A1 的退路只把 `mcp.json` 换成字面的令牌，`account.json` 与 `throttled.json` 同样要换 | 三份都写成字面的：`mcp-literal.json`、`account-literal.json`、`throttled-literal.json`，都 `chmod 600`，A5 与 F 用它们，做完删掉 `*-literal.json`（6.1） |
| C2-Nc | Nit | 6.2 为"满了时记 info"引 `largebody.go:188-226`，那是 `takeLarge`；info 的一行在 `saturated`，`:243-252` | 照改：`takeLarge` 的 `own` 引 `:188-226`，info 的一行引 `:243-252`（6.2） |
| C2-Nd | Nit | "做完"第 2 步在 `make run` 还在跑时删 `$KEYS`；`unset` 也可以带上 `EMAIL` | 次序改为：Codex 的登录；在服务还在跑时撤销 PAT；停下 `make run`、限速器与容器；最后删临时目录，含 `$KEYS`。`unset` 带上 `EMAIL`（6.1） |
| C2-Ne | Nit | Codex 登录的第二种只让负责人记下自己的 Codex 还能不能用，没给办法 | 加"不能用时在那个终端里（你自己的 `~/.codex`）`codex login` 再登录一次"，写在隔离一节与"做完"第 1 步（6.1） |
| C2-Nf | Nit | 2.1 第 9 条改过之后，00 号文档 `:612` 的那一句仍以"P5 的接入文档按路由写"结尾 | 第 9 条也改它，改为"README 的部署一节按路由写（nginx 与 Caddy 的样例，M9/P5）"，同 7.3 对 M12 的那一条的改法（2.1 第 9 条） |
| C2-Ng | Nit | 准备 4 的"`grep` 只给它前后约 1,000 字节"含糊：`GrepHitOf` 一共给约 1,000 字节，匹配的两边各约 500 | 改为"只给 `FIXME` 前后共约 1,000 字节"（6.1） |
| C2-Nh | Nit | F1 的括号"第二轮起 `占位.md` 已在，答 `page.title_taken`"不对：不带 `base_version` 写同样的正文答 `changed: false`，不是 409（`page/app/write_path.go:176-190`） | 改为第二轮起答 `changed: false`（6.1 F1） |

### 重叠（第一轮复核）

| 审查 | 审查 | 处置 |
|---|---|---|
| A2-N2 | （A 料 C 也会记；C 没有另记） | `M5-polish.md:10` 的一处改（第 11 节的移交 1） |
| A2-M1 | A-6（A 自己第一轮的建议） | 235b 成两处的变异（第 11 条） |
| C2-Mc | B-N8 | 那一句标"未核对"，4.2 给它一行（第 14 条） |

### 主持者的决定（第一轮复核）

主持者在第二份修订的简报里定下的处置（作者照做；负责人可以改判）：

11. **235b 是两处的变异**（A2-M1）：`bootstrap/linking.go:37` 以 `e.Write.Client.IsMCP()` 填 `linking.PagesChanged` 的一个新字段，`index.go:85` 在它为真时跳过 `x.publish`。写明这是测试一侧两个文件的临时改动，从不提交。这一行于是只有 MCP 抓。
12. **经 Caddy 的 502 交 M12**（B2-M1）：采纳，Caddy 的样例留着，P5 不加服务端的代码。要做的是：
    - README 在 Caddy 的部分加一句：服务端在读请求体之前的拒绝（503 `server_busy`、401、429），经 Caddy 可能以 502、没有 `Retry-After` 到达客户端，网页的大的保存与 agent 于是看到一次失败、不会重试；nginx 照原样转发，B 量过。
    - 5.2 写出 B 的数，并说明上游是一个同样不读请求体就答的 Go `net/http` 的替身，同 `largeBody`。
    - 9.3：B 的探测加进 S1 合并之前的重跑，复制进 `exp/caddy/early/`、照抄进 9.3，同 `run.sh`，免得夜里的清理删掉。
    - I3 记下经 Caddy 的一次被拒的大的写显示什么。
    - 第 11 节给 M12 一条，放进部署文档的移交或 `M4-performance.md`，看哪一个合适：502；网页的重试只等带 `Retry-After` 的 503（`page-editing.ts:336`）；401 的情形（大的保存碰上过期的访问令牌）。服务端怎样改留给 M12，例如早答之前把一个很快地到来的请求体短短地读掉；写作一种可能，不是决定。

    B 的 Nit 照 B 的建议。
13. **A 的其余**（A2-M2、A2-M3、A2-N1 到 A2-N3）：
    - C7 (MCP) 第 2 步先写 `"Draft\n"`，再以 `old_string: "Draft\n"` 与 `base_version "<id>:1"` `edit` 它，重核预期的帧；
    - PG4 第 2 步写整个文字，带着 `validation_failed: `，与 `text.go:35-38`、`mcpserver/errors.go:156` 造出的一样，并照样重核第 3 节别的"整个文字"的期望；
    - SE1 的不比以 `Omit<Found, "marked">` 写（重载，或另一个 `expectNamed`），从不用假的 `marked: []`；
    - C7 (API) 的安静的检查照 A 说的把 Marker 提出来；
    - `M5-polish.md` 的开头照 A 说的改，C 若也记，一处改。
14. **C 的三个 Minor 与 Nit**：
    - C2-Ma：2.1 第 13 条以 4.16 的整个 Caddy 分句（`:384`）为原文，改得与 README 的 Caddy 样例一致；核对新的原文在 `HEAD` 恰好一次。
    - C2-Mb：F1 的循环加刹车（`curl --fail … || sleep 2` 或同样的）；预期写明日志里一串 401 或 404 时，`kill %1` 停下循环、改正 `$TOKEN` 或 `$NB`；一句写出认证失败的闸门的代价。
    - C2-Mc：B-N8 的一句加标记，在 4.2 给它一行。agent 启动的进程继承客户端的环境，这是 shell 的事，不是客户端的；某个客户端交不交给它的命令工具未核对，客户端的一半写"未核对"。
    - Nit 照 C 的建议；第二种登录的办法是在负责人自己的 `~/.codex` 里再 `codex login` 一次，写明。
15. **过程**（"过程中的一处失误"之后）：
    - Docker 只经 `heavy.sh`，`--rm --pull=never --network none`，没有映射端口，容器的列表也一样；
    - 结束之前以 `heavy.sh docker ps -a` 确认没有留下作者的容器；
    - 修订之前先把第一轮修订的样子存作 `05-P5-stories-docs.rev1.md` 与 `parts-rev1/`。

### 第一轮复核的作者的更正

| # | 发现 | 处置 |
|---|---|---|
| 作者-1 | B2-M1 的表里 nginx 在 2 MiB、4 MiB 写着"503"，但 `review-B/early/out-nginx.txt` 只有 100 KiB、1 MiB、8 MiB 与 30 MiB，各一次；Caddy 的 100 KiB 与 1 MiB 也各一次。B 的表头"每格是三次"对这些格不成立 | 设计的表里那两格写"没量"，并写明哪些格只发了一次。B 在第二轮复核里认为这个更正对，并补量了：nginx 每个大小、每类路由三次，45 次都是带 `Retry-After: 1` 的 503（B3-N1）；Caddy 的 100 KiB、1 MiB 只量了 `/mcp`（B3-N3） |
| 作者-2 | 网页的 401 的路也被挡住了：网页只在 401 时续期再发（`web/apps/web/src/session/auth-middleware.ts:45`），经 Caddy 时那个 401 成了 502，一次大的保存碰上过期的访问令牌就立刻失败 | 写进 5.2 与 M12 的那一条，与 `page-editing.ts:336` 的只等 503 的重试并列。A、B 在第二轮复核里核对过这两处 |
| 作者-3 | 502 的那一条放在哪份移交 | 放进 `M4-performance.md` 第 18 项，加两条新的；`M7-transfer.md` 的部署一句指向它。理由：502 是名额与认证的早答经代理之后的样子，改法（若改）在服务端，与这一项的名额、慢的发送者是同一件事。B 在第二轮复核里认为对：第 18 项是那份文件的最后一项，新的两条接在它之后 |
| 作者-4 | 照简报重核了几处 | 第 3 节别的文字的期望都是"包含"或字段的说明，只有 PG4 比整个文字；2.1 第 13 条新的原文、第 9 条加的 `:612` 的一句与 `M5-polish.md:10` 的一句，在 `7dc4a686` 都恰好一次 |

### 第一轮复核审查者核对过、没有问题的

- **A**：
  - 第 0 节列出每一处 e2e 的改动，第 8 节的步与它一致，每一步加的导出与它的导入者在同一步（knip）。
  - 第 1 节：MC2 的 `PageRead` 在 `:64-73`，`readAll` 在 `:86`；满了时的一行在 `largebody.go:244-250`。
  - 2.1 的 e2e 一侧：第 1、3、5、6、7、8 条，`:493`、`:589`、`:592` 都对。
  - 3.1：oxlint 的 `no-await-in-loop`；`readPage` 的九个使用者。
  - 各故事的改动逐个对过：PG2、PG7、PG9、PG11、PG12、PG14、L3、L6、AS2、AS4、HI1、SE1。
  - 变异：A 列出的各条（235b 之外）的位置与步都对；#234 在 `gen/activity.sql.go:15-21` 时无主的笔记本的大小为 0；#245 的四处 `j.deleted_at IS NULL`；L6 (API) 与 C7 (API) 的修法都在今天的服务上通过，C7 (API) 在 #236 下失败。
  - 9.3 的 Caddy 的重跑：照抄的 `Caddyfile` 与 `run.sh` 与 `exp/caddy/` 的逐字节相同，`run-out.txt` 的数对得上，`heavy.sh` 的命令带 `--rm --pull=never --network none`、没有端口。
  - 第 10 节列出 223–251 与 235a、235b；第 11 节 e2e 一侧的目标文件与锚都在。
- **B**：
  - 4.1 的样例、变量与标记对着 4.2 的 14 行都对；清单照抄样例时只换地址、id 与变量。
  - 修订后的 nginx 样例 `nginx -t` 通过，按每个大小正确地转发早答。
  - README 的 Caddy 样例 `caddy validate` 通过，作者重跑的用时与 B 的一致。
  - 2.1 第 9、13 条的原文在 `:612`、`:384`，新的文字与 5.1 一致；7.2 的 6.3 与 11、7.3 的 M12 的改动与第 11 节的四份移交与 4.1、5.1 一致（B2-N4 除外）。
  - 附录里抽查的行都对。
- **C**：
  - 2.1 与第 7 节新的与改过的每段原文在 `HEAD` 都恰好一次；7.1 列的行号对得上文件。
  - 2.1 第 8 条的新证据：TB1、TB2 的五处都带 `{ alone: true }`，`mc4-move-delete.spec.ts:129` 不带。
  - 2.1 第 9 条的事实：撤销只撤自己的令牌；管理员命令按 `--email`；两种行都有 `user_id`；满了时的凭证是 `pat:<id>` 或 `session:<id>`；约 45 秒的算法。
  - 每个客户端的变量前后一致：`NERVEWIKI_TOKEN` 只留在第 13 条的原文里。
  - 种子的新数（`长文.md` 约 69 KB，第 601 行的截取的窗口）都对；E6、G1、G2 在导出了私钥的终端里重启。
  - 第 11 节的 E2E 的数等于 9.4 的；M10 的 69 字节与 4,027、3,957 字节与 C 之前重算的一致。

### 第一轮复核未采纳

没有。

### 第一轮复核的新问题

1. **经 Caddy 的 502 是窄的情形，却正是名额自己的拒绝**：缓冲让"慢的发送者占着名额"少见；而 Caddy 不缓冲的默认在 2 MiB 时也有这件事。P5 留着样例，服务端改不改、怎样改由 M12 定，部署文档至少写明用 Caddy 时的这个代价，或推荐 nginx（第 11 节给 M12 的性能移交）。
2. 第一轮的"新的问题"照旧。

### 第一轮复核的实验

都是 B 的，经 `heavy.sh`，`--rm --pull=never --network none`，没有映射端口，只用本机的镜像。作者把它们复制进 `scratchpad/m9/p5/exp/caddy/`，免得夜里的清理删掉。这一轮作者没有起容器，结束时 `heavy.sh docker ps -a` 里没有作者的容器。

- **经 Caddy 与 nginx 的早答**（`exp/caddy/early/`：`backend.Caddyfile`、`proxy.Caddyfile`、`run-caddy.sh`、`run-caddy2.sh`、`run-nginx.sh`、`site.conf`；结果在 `out-caddy.txt`、`out-caddy2.txt`、`out-nginx.txt`）。上游不读请求体就答 503 与 `Retry-After: 1`：
  - Caddy 缓冲的路由（`/mcp` / 正文的 PUT）：2 MiB 是 503 502 502 / 502 502 502；4 MiB 是 503 502 502 / 502 502 503；8 MiB 都是 502；30 MiB 是 502 502 502 / 502 502 503。
  - Caddy 不缓冲的附件：2 MiB 是 503 502 502；8 MiB 是 502 503 503；4 MiB 与 30 MiB 都是 503。
  - 100 KiB、1 MiB 只量了 `/mcp` 与附件，各一次，都是 503。
  - nginx 在 100 KiB、1 MiB、8 MiB、30 MiB 都把 503 与 `Retry-After` 照原样转发，各一次（第二轮复核更正：三次的补量，B3-N1）。

  Caddy 的日志里那些 502 是 `write: connection reset by peer`。
- **分开进程的内存**（`exp/caddy/mem-separate/`：`run.sh`、`backend.Caddyfile`、`proxy.Caddyfile`、`out.txt`）：30 MiB 的请求体 42,368 → 108,472 kB；36 MiB（37,748,736 字节，刚在 `/mcp` 的 37,814,272 之下）108,544 → 224,268 kB；都没有临时文件。

## 第二轮复核

| 项 | 内容 |
|---|---|
| 复核对象 | 第二次修订的 [05-P5-stories-docs.md](../05-P5-stories-docs.md)（2026-10-23，258,995 字节、1,402 行；复核时的样子留作 `05-P5-stories-docs.rev2.md` 与 `parts-rev2/`）与"第一轮复核"的处置，对照 `main` `7dc4a686` |
| 复核方式 | 三位审查者（Opus）各复核自己的一份，只读，不下载，不运行任何 MCP 客户端；都以与 `.rev1.md` 的差找出这一轮改了什么。<br>A：没有起容器。`heavy.sh docker ps -a` 滤掉别的会话的之后，什么也没有。<br>B：`review-B/early2/` 的两次运行：<br>- 同样的 Caddy 替身改为在读请求体之前答 401（带 `WWW-Authenticate`）或 429，结果在 `out-caddy-401-429.txt`；<br>- nginx 每个大小三次，结果在 `out-nginx-3x.txt`。<br>上游是同一个 `caddy` 二进制，从本机的镜像拷进 scratch，用完删掉。都经 `heavy.sh`，`--rm --pull=never --network none`，没有映射端口；结束时 `heavy.sh docker ps -a` 里没有 B 的容器。<br>C：2.1 与第 7 节的每段原文以 `grep -o -F` 对着 `7dc4a686` 数过，共 26 段，加上 v0.1 的四个锚、`M4-performance.md:95` 的三处与 `M5-polish.md:10`。没有起容器；`heavy.sh docker ps -a` 里只有别人的：别的会话的 testcontainers 与 `opennerve-wt-…`，负责人长期跑着的 `plane-app-*`、`agentforge-*`、`opennerve-superbase-*`，都没有碰 |
| 日期 | 2026-10-23 |
| 结论 | **各人的数**：<br>- A：第一轮复核的 6 条都落下，没有新的，A 的部分没有 Important 与 Minor。<br>- B：第一轮复核的 6 条都落下，作者对 B 的表的更正 B 认为对（"第一轮复核的作者的更正"第 1 条）。新的 Important 0、Minor 0、Nit 3，带着新的量。<br>- C：第一轮复核的 11 条都落下。新的 Important 0、Minor 1、Nit 2：Minor 是部署一节新的一句的 agent 的一半，那是 4.2 之外的一句客户端的行为。<br>**处理**：新的 6 条全部处理，处置由主持者定下（"主持者的决定（第二轮复核）"第 16–18 条），负责人可以改判。修订直接写进设计，复核时的样子留在 `05-P5-stories-docs.rev2.md` 与 `parts-rev2/`。<br>**三种早答都由替身量过**：<br>- 经 Caddy，答 401、429 的替身在 1 MiB 时照原样转发；8 MiB 时 12 次里 11 次是 502。<br>- nginx 每个大小、每类路由三次，45 次都是带 `Retry-After: 1` 的 503。<br>README 于是写"我们在 Caddy 2.10.2 上量过这三种，用的是在读请求体之前就答它们的替身，不是 nervewiki 本身"。<br>**修订之后**：4.2 有 16 行 |

### 发现与处置

B 把新的条目写作 B3-N1 到 B3-N3，C 写作 M2-a、N2-a、N2-b；这里写作 C3-Ma、C3-Na、C3-Nb。

#### B

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| B3-N1 | Nit | 5.2 的表的"没量"与只发一次的说明，可以照 B 新的一次改：`review-B/early2/out-nginx-3x.txt` 里 1、2、4、8、30 MiB 时 `/mcp`、正文的 PUT 与附件各三次，都是带 `Retry-After: 1` 的 503。这些文件要复制到 `exp/caddy/early/` 旁边，免得夜里的清理删掉 | 照主持者的决定（第 17 条）：<br>- 两格"没量"与 nginx 的 1 MiB 起都改为"503 503 503"；<br>- 表的引言写明哪些格只发了一次、各自出自哪个文件，表下的一句引 `early/out-nginx.txt` 与 `early2/out-nginx-3x.txt`；<br>- `review-B/early2/` 复制为 `exp/caddy/early2/`，第 5 节第 4 条、5.2 的标题、9.3 与附录引它 |
| B3-N2 | Nit | README 的"我们在 Caddy 2.10.2 上量过"，直到这一轮只对 503 的替身成立；现在对三种都成立。`early2/out-caddy-401-429.txt`：1 MiB 时 401 与 429 都照原样转发，各三次；8 MiB 时 401 在 `/mcp` 是 401 502 502、正文的 PUT 是 502 502 502，429 两处都是 502 502 502。5.2 可以加一行；I3 的"401……未核对"可以改为"替身量过（5.2）" | 照主持者的决定（第 17 条）：<br>- 5.2 加一张四行的表（替身答的、请求体、`/mcp`、正文的 PUT），写明只记了状态码、nginx 没有以这两个替身量、8 MiB 时 12 次里 11 次是 502。主表的列是路由，所以另成一张小表，不加进主表。<br>- README 写"我们在 Caddy 2.10.2 上量过这三种，用的是在读请求体之前就答它们的替身，不是 nervewiki 本身；nginx 照原样转发了替身的 503 与它的 `Retry-After`"，同 C3-Na 一处改。<br>- "后果"一段写 401 与 429 也由替身量过，网页只在 401 时续期再发。<br>- I3 的预期写"替身量过（P5 设计 5.2）……这一步是在你的部署上、以真的服务端核对"。作者另改了 I3 的一处（"第二轮复核的作者的更正"第 1 条） |
| B3-N3 | Nit | 5.2 的表的第一行：Caddy 缓冲的一列在 100 KiB、1 MiB 只量了 `/mcp`，各一次 | 第一行分成 100 KiB 与 1 MiB 两行；缓冲的一列写"503（只有 `/mcp`，一次）"，不缓冲的一列写"503（一次）" |

#### C

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| C3-Ma | Minor | README 部署一节新的一句（Caddy 的样例之后）有一半在 4.2 之外，第 10 节的标准又差一句："网页的大的保存与 agent 看到的是一次失败，不会等一等再重发"。网页的一半是我们的代码，核对过：保存只在 503 带 `Retry-After` 时重试（`page-editing.ts:336`），只在 401 时续期（`auth-middleware.ts:45`）。agent 的一半说的是 Claude Code 与 Codex 怎样对待 502，仓库里没有记录。服务端的事实（503 的 `Retry-After` 没了）就够作警告 | 照主持者的决定（第 16 条）：<br>- agent 的一半改为"agent 看不到 `Retry-After`（客户端怎样对待 502 未核对）"，网页的一半照旧；<br>- 4.2 加第 16 行：服务端的一半由替身量过（5.2），客户端的一半未核对；清单的 I3 记下经 Caddy 的被拒的写显示什么（状态码与头，不是客户端），客户端对 502 的做法清单不核对；"留作提醒"；<br>- 之后对着 4.2 的 16 行重读了 4.1 与 5.1 的每一句客户端的行为，每一句都在样例里或有它的一行。<br>4.1 里 `isError` 的结果"模型读得到码与说明"是 MCP 的结果的约定，归在 HTTP 的答复那一行之下，不另加一行 |
| C3-Na | Nit | 那一句说早答的 401、429 也成了 502"（我们在 Caddy 2.10.2 上量过）"，而探测只量了答 503 的替身。401、429 同理：都在读请求体之前答（`api.go:335-350`、`limit.go:24-36`） | 由 B3-N2 取代：B 补量了 401 与 429，README 用一个说法，三种都由替身量过，不是 nervewiki 本身 |
| C3-Nb | Nit | F1 的循环与 I3 的管道写在表的格子里，竖线成了反斜线加竖线。在渲染出的表里看是对的，照原文复制（编辑器、`less`）时却带上反斜线；F1 的两条竖线会被当作 curl 的参数，循环立刻坏掉。E4 已经把它的管道放在表下的代码块里 | 照主持者的决定（第 17 条）：<br>- F1 的 `node -e` 与占位的循环挪到 F 的表下的代码块"F1 的占位"；<br>- I3 的 8 MiB 的写挪到 I 的表下的代码块"I3 的被拒的写"，先经 `$CADDY`、再经 `$BASE`，一个循环；<br>- 两个格子指向它们。<br>整份文档里转义的竖线一处也不剩，之前只有 F1 与 I3 两处 |

### 重叠（第二轮复核）

| 审查 | 审查 | 处置 |
|---|---|---|
| C3-Na | B3-N2 | README 的一个说法：三种早答都由替身量过，不是 nervewiki 本身（第 17 条） |

### 主持者的决定（第二轮复核）

主持者在第三份修订的简报里定下的处置（作者照做；负责人可以改判）：

16. **C3-Ma 照 C 的建议**：agent 的一半改为"agent 看不到 `Retry-After`（客户端怎样对待 502 未核对）"，给它 4.2 的一行；之后对着 4.2 把 4.1 与 5.1 的每一句客户端的行为再核对一次。
17. **其余照审查者的建议**：
    - C3-Na：README 写明量的是什么，401 与 429 同理。由 B3-N2 取代，两者一个说法："替身量过，不是 nervewiki 本身"。
    - C3-Nb：F1 的循环与 I3 的管道挪到表下的代码块，同 E4；表的格子里别的命令有转义的竖线的，同样挪出。
    - B3-N1：5.2 的"没量"与只发一次的说明照 B 新的一次改；`review-B/early2/` 复制进 `exp/caddy/early2/`，并引它。
    - B3-N2：5.2 加 401 与 429 的替身；README 的"量过"于是管三种，写明是替身量的、不是 nervewiki 本身；I3 的"401……未核对"改为"替身量过；在你的部署上核对"。
    - B3-N3：Caddy 缓冲的一列的 100 KiB、1 MiB 标"只有 `/mcp`，一次"。
18. **过程**：这一轮不起容器，也没有要起的；修订之前先把第二次修订的样子存作 `05-P5-stories-docs.rev2.md` 与 `parts-rev2/`。

### 第二轮复核的作者的更正

| # | 发现 | 处置 |
|---|---|---|
| 作者-1 | I3 的预期原来说"（401 的也没有 `WWW-Authenticate`）"，而 `early2` 只记了状态码，没有记头 | 改为"503 的那些没有 `Retry-After`，401 的头替身没有记" |
| 作者-2 | 第 11 节给 M12 的那一条说"nginx 照原样转发"，nginx 只以 503 的替身量过 | 改为"nginx 照原样转发（量的是 503）" |
| 作者-3 | 清单的"结果"一行说"README 的'接入 agent（MCP）'里的客户端的句子（P5 设计 4.2 的表）"；4.2 现在也管"MCP"一节与"部署"一节的句子 | 改为"README 里客户端的句子（P5 设计 4.2 的表）" |

### 第二轮复核审查者核对过、没有问题的

- **A**：
  - 9.3 的早答的探测：照抄的三个文件与 `exp/caddy/early/` 的逐字节相同；`heavy.sh` 的命令对。替身代替服务端成立：`largebody.go:146` 让过了门槛的声明的长度落到 `takeLarge`，`:166-170` 写出错误、不读请求体就返回。"多半答 502"与 `out-caddy2.txt` 相符。第 10 节要求把探测的状态码写进审查记录。
  - 网页的两条路：401 的续期（`auth-middleware.ts:45`）与只等 503 的重试（`page-editing.ts:336`）。
  - 附录这一轮新的各行：PG4、C7、L6、SE1。
  - 第 3 节的帧、9.1、9.2 的 #235b 与 #236、第 8 节第 5、7 步与第 10 节互相一致，A 的部分没有被改坏的。
- **B**：
  - B2-M1 的各部分都在，而且一致：README 的一句；5.2 的表、日志、替身的说明、情形与为什么留着；9.3 照抄的探测，三份文件在 `exp/caddy/early/`，镜像的 entrypoint 是 `null`，所以 `sh /cfg/run-caddy2.sh` 可行；I3；2.1 第 9、13 条与 7.2；第 11 节的那一条。
  - 401 的路对。
  - 第 18 项的位置对：它是那份文件的最后一项（到 `M4-performance.md:123`），新的两条保持三个空格的缩进。
  - B2-N1 到 B2-N5 都落下。
  - 改写的 B-N8 一句与它在 4.2 的一行成立。
  - 2.1 第 13 条的新原文与新文字，与 README 的样例、2.1 第 9 条一致。
  - M12 的改法写作一种可能。
  - 附录的代理一行的引用在 `7dc4a686` 都对。
  - 第 4、5 节没有退步。
- **C**：
  - 2.1 与第 7 节的每段原文在 `HEAD` 恰好一次，共 26 段，含 `:384` 的整个 Caddy 分句与 `:612` 的"P5 的接入文档按路由写"。v0.1 的四个锚、`M4-performance.md:95` 的三处与 `M5-polish.md:10` 的"第 30 条 M9"各一次。一行里的几处改动不重叠：`:384` 是第 10、13 条，`:612` 是第 9 条的四处。
  - 4.2 的 15 行之外只有 C3-Ma 的一句。
  - I3 新的记下的一栏：直接发的 401 在上传之前就到（curl 对这样大的请求体发 `Expect: 100-continue`）；经 Caddy 期望 502，与 5.2 一致；做完删掉 `big.bin`；两次各耗认证失败的闸门的一个单位，无害。
  - `M4-performance.md` 第 18 项的 502 一条：替身写明了；三种状态；网页的两条路；M12 的改法写作可能；`M7-transfer.md:29` 指向它；第 10 节与 7.3 写"两条新的"。
  - 附录这一轮改的各行都对：L6 的 235b、C7 的 `edit.go:15`、代理一行的早答。

### 第二轮复核未采纳

没有。B3-N2 的"加一行"以另一张小表落下（主表的列是路由），内容照 B 的。

### 第二轮复核的新问题

1. **替身不是 nervewiki 本身**：三种早答都只以替身量过。在真的服务端上是什么样子，由清单的 I3 在负责人的部署上核对。
2. 前面的"新的问题"照旧。

### 第二轮复核的实验

都是 B 的（`review-B/early2/`），作者复制为 `scratchpad/m9/p5/exp/caddy/early2/`：`backend.Caddyfile`、`backend-401.Caddyfile`、`backend-429.Caddyfile`、`proxy.Caddyfile`、`nginx.conf`、`site.conf`、`run-caddy.sh`、`run-nginx.sh`、`out-nginx-3x.txt`、`out-caddy-401-429.txt`。代理的配置与第一轮复核的相同（`proxy.Caddyfile`、`site.conf`、`nginx.conf`、`backend.Caddyfile` 逐字节相同）。这一轮作者没有起容器。

- **nginx，三次**（`out-nginx-3x.txt`）：1、2、4、8、30 MiB，`/mcp?notebook=x`、`/api/v0/pages/x/content`、`/api/v0/notebooks/x/assets` 各三次，45 次都是带 `Retry-After: 1` 的 503。修订后的样例 `nginx -t` 通过。
- **Caddy，401 与 429 的替身**（`out-caddy-401-429.txt`）：
  - 401 在 1 MiB：`/mcp` 与正文的 PUT 都是 401 401 401；在 8 MiB：`/mcp` 是 401 502 502，PUT 是 502 502 502。
  - 429 在 1 MiB：两处都是 429 429 429；在 8 MiB：两处都是 502 502 502。

  只记了状态码。

## 第三轮复核

| 项 | 内容 |
|---|---|
| 复核对象 | 第三次修订的 [05-P5-stories-docs.md](../05-P5-stories-docs.md)（2026-10-23，261,719 字节、1,438 行；复核时的样子留作 `05-P5-stories-docs.rev3.md` 与 `parts-rev3/`）与"第二轮复核"的处置，对照 `main` `7dc4a686` |
| 复核方式 | 这一轮只由 C（Opus）复核自己的一份（第 2、6、7、10、11 节与附录）。只读，不下载，不运行任何 MCP 客户端，不读客户端的配置；没有起容器，也没有执行 Docker 的命令。<br>- 与 `.rev2.md` 求差，对着 `revise-3-log.md`、`exp/caddy/early2/` 的探测文件与 `7dc4a686` 的仓库读；<br>- 把 F1、I3 的两个代码块取出来，`sh -n`、`bash -n`、`zsh -n` 都通过；取出的文件放在 `m9/p5/` 之下的临时目录，之后删掉。<br>B 的三个 Nit 的数由主持者对着 5.2 的两张表核对过（"主持者的最后核对"） |
| 日期 | 2026-10-23 |
| 结论 | **C**：第二轮复核的 3 条（C3-Ma、C3-Na、C3-Nb）都落下。新的 Important 0、Minor 0、Nit 2（C4-Na、C4-Nb，都是可做可不做的）。C 的部分没有 Important 与 Minor，第 10 节的标准（"4.2 的表之外没有客户端的行为"）成立。<br>**B 的三个 Nit**（B3-N1 到 B3-N3）落下：主持者对着 5.2 的两张表核对了 `early2/` 的数，nginx 45 次都是带 `Retry-After` 的 503，Caddy 的 8 MiB 的替身 12 次里 11 次是 502。<br>**处理**：这一轮的四个 Nit，C 的两个与作者的两个，都落下（"主持者的决定（第三轮复核）"第 19–23 条）。修订直接写进设计，第四次修订之后是 262,560 字节、1,439 行；复核时的样子留在 `05-P5-stories-docs.rev3.md` 与 `parts-rev3/`。<br>**`apply-00m9.py`**：随第 21 条改了 AS4 一行的新文字，仍是 21 处，`--check` 对着 `7dc4a686` 的 00 号文档全部对上。<br>**之后**：主持者自己最后核对，不再有审查者的一轮 |

### 发现与处置

C 把新的条目写作 N3-a、N3-b，这里写作 C4-Na、C4-Nb。

C 对第二轮复核的条目：

| # | 结论 | 理由 |
|---|---|---|
| C3-Ma | 落下 | README（`:694`）的 agent 的一半只说服务端的事，客户端的一半标"未核对"；网页的一半是我们的代码。4.2 有它的第 16 行（`:602`）。清单的"结果"一行改为"README 里客户端的句子"，也管部署一节的这一句 |
| C3-Na | 落下，由 B3-N2 取代 | README 写三种都由替身量过，不是 nervewiki 本身。C 对着 5.2 的第二张表核对了 `early2/out-caddy-401-429.txt`：1 MiB 时都照原样转发，8 MiB 时 12 次里 11 次是 502。`run-caddy.sh` 只记 `%{http_code}`，所以 I3 的"401 的头替身没有记"是对的 |
| C3-Nb | 落下 | 文档里没有一处转义的竖线。两个代码块（`:959-969`、`:1026-1035`）与 rev2 的命令逐个记号相同，只是转义的竖线成了竖线、分号成了换行；F1 的刹车留着；没有一行的反斜线之后有空格。F1 的块另写明在另一个终端里设 `BASE`。I3 的块先经 `$CADDY`（块的引言里定义）、再经 `$BASE`，与格子里的说法一致 |

新的：

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| C4-Na | Nit | I3 的循环把两个答复先后打在标准输出上，只靠次序才分得出哪个是经 Caddy 的。循环的变量 `base` 与 `BASE` 也只差大小写 | 照主持者的决定（第 19 条）：变量改名为 `addr`，循环的第一行是 `echo "== $addr"`（6.1 I3 的代码块） |
| C4-Nb | Nit | F1 的另一个终端有两件事。一是 `body.json`（1 MiB）落在那个终端的工作目录里，而新开的终端通常在 `~` 或仓库里，"做完"第 4 步却只删放 `seed.mjs`、`body.json`、`big.bin` 的那一个目录。二是那个终端设的 `TOKEN` 不在"做完"的 `unset` 里，只有"关掉设过令牌的终端"管得到它。这是 rev2 就有的，代码块让它更显眼 | 照主持者的决定（第 20 条）：代码块的引言写"在放 `seed.mjs` 的临时目录里"；"做完"的 `unset` 加上 `TOKEN`（6.1 F1、做完） |

### 主持者的决定（第三轮复核）

主持者在最后一份修订的简报里定下的处置（作者照做；负责人可以改判）：

19. **C4-Na**：照 C 的建议，I3 的循环的变量改名为 `addr`，每个答复之前打出 `echo "== $addr"`。
20. **C4-Nb**：照 C 的建议，F1 的引言写在放 `seed.mjs` 的临时目录里做，"做完"的 `unset` 加上 `TOKEN`。
21. **2.1 第 7 条的说法**（作者的）：改为"无主的笔记本的大小（AS4 的第二个测试）没有 MCP 版本"，`apply-00m9.py` 随之。
22. **第 8 节第 13 步写死**（作者的）：设计的提交的 `apply-00m9.py` 只带 2.1 的 13 条与第 12、13 节的两行；第 7 节与第 11 节其余的都在 S1 合并之后的文档提交里。第 7 节的开头与第 10 节的"文档"一段照它改，三处说法一致。
23. **过程**：修订之前先把第三次修订的样子存作 `05-P5-stories-docs.rev3.md` 与 `parts-rev3/`；不起容器；之后由主持者自己核对。

### 第三轮复核的作者的更正

| # | 发现 | 处置 |
|---|---|---|
| 作者-1 | 2.1 第 7 条给 AS4 一行的"无主的笔记本的大小一个测试也没有"，读起来像"一个测试也没有"；它说的是 AS4 的第二个测试没有 MCP 版本 | 改为"无主的笔记本的大小（AS4 的第二个测试）没有 MCP 版本"，`apply-00m9.py` 的第 7 条随之（仍是 21 处，`--check` 全部对上） |
| 作者-2 | 第 7 节的开头写"每处以脚本落下（`apply-00m9.py`、`apply-v01.py`）"，没说设计的提交的脚本只带哪些；第 10 节的"文档"一段把"2.1 各条（随设计的提交）"列在合并之后的文档提交之下 | 第 8 节第 13 步加一句：设计的提交的 `apply-00m9.py` 只落 2.1 的 13 条与 00 号文档第 12、13 节的两行；第 7 节其余的各处（7.1 的 S1 合并之后的各处、7.2 的 `apply-v01.py`、7.3）与第 11 节的四份移交都在合并之后的文档提交里。第 7 节的开头与第 10 节的"文档"一段照它改 |
| 作者-3 | 设计的状态一行还写着"……之后的第二次修订" | 改为"第一轮审查（A、B、C 三份）与修订的三轮核对之后的第四次修订，待主持者的核对" |

### 第三轮复核审查者核对过、没有问题的

- **C**：
  - **第 10 节的标准**：4.1 自 rev2 没有改动，第二轮复核时已逐句对到 4.2。5.1 里说到 agent、客户端、Claude Code、Codex 或 MCP 的每一行重读过，客户端的行为只有 Caddy 的那一句的 agent 的一半，它有第 16 行。"每个客户端都成了代理的 IP"（`:633`）说的是 HTTP 客户端的地址，不是 MCP 客户端的行为。4.2 有 16 行，它的结语成立。
  - **2.1 与第 7 节**：都没有改动，`HEAD` 仍是 `7dc4a686`，第二轮复核的"每段原文恰好一次"照旧成立。
  - **F1 的格子**：只是挪走了命令。26 秒的一轮、`changed: false`、闸门的警告与期望的 503 都不变。
  - **I3 的预期**：三种拒绝都引替身，与 5.2 的两张表一致；最后一句"这一步是在你的部署上、以真的服务端核对"说清了这一步的用处。
  - **第 11 节给 M12 的一条**：写"nginx 照原样转发（量的是 503）"，准确，nginx 没有以 401、429 的替身量过（`:746`）。
  - **9.3 的说明**（`:1317`）：提到的 `early2/run-nginx.sh`、`out-nginx-3x.txt`、`run-caddy.sh` 都在；它们要拷来的 `caddy` 二进制或替身的配置，不在合并之前的重跑里，也对。
  - **附录的代理一行**：加的 `early2/` 的两个文件都在。
  - **5.2 的数**：不在 C 的范围，因 I3 与第 11 节引它而核对。`out-nginx-3x.txt` 是 15 行、各三次，都是 `503+RA`；探测的 `proxy.Caddyfile` 是 README 的样例的形状。

### 主持者的最后核对

- 主持者对着 5.2 的两张表核对了 B 的 `early2/` 的数：nginx 45 次都是带 `Retry-After` 的 503，Caddy 的 8 MiB 的替身 12 次里 11 次是 502。
- 这一轮之后不再有审查者的一轮：主持者自己核对第四次修订的设计（262,560 字节、1,439 行）与 `apply-00m9.py --check`，之后提交设计、这份记录与 00 号文档的改动。
- 主持者核对的：
  - 第四次修订对 `.rev3.md` 的差，只有上面第 19–23 条与作者的三处；
  - `apply-00m9.py --check` 对着 `7dc4a686` 的 00 号文档，21 处全部对上；
  - 在 scratchpad 的副本上真的落一遍，16 行改动，逐行读过：2.1 的 13 条、第 12 节的 P5 一行与第 13 节的一行。
- 主持者自己改的两处：设计的状态一行写成审查之后的样子；"研究与实验"一行加上第三轮的 `early2/`。

### 第三轮复核的实验

这一轮没有新的实验：C 没有起容器，也没有执行 Docker 的命令；作者的修订也没有起容器。主持者核对的是第二轮复核的数据（`exp/caddy/early2/`）。

