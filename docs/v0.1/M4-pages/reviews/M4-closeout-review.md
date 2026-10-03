# M4 页面：收尾审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | `main` 的 `257b688`（M4 的六个 Phase 全部合并、P6 文档写好之后），M4 的改动是 `b2e7c91..257b688`。对照[文档约定](../../../README.md)的"M 完成"、[M4 总设计](../00-M4-design.md)、六个 Phase 文档与审查记录、收到的四份移交、[人工清单](../manual/P6-ime-checklist.md)与[总体设计](../../v0.1-design.md)中涉及 M4 的部分 |
| 审查方式 | 三位独立审查者并行（A 后端、B 前端与端到端、C 完成标准与文档），各在自己的 `git archive` 快照上做探针，仓库与别的 Docker 容器都没动过：<br>• **A**：`make lint-go`、`go test -race -count=1 ./...`、`server/tools` 的测试、`make gen-check`；page、platform/markdown、bootstrap 另跑 `-race -count=3`（285 个顶层测试、1684 个子测试，三遍）；交错 30–44 另跑 `-race -count=10`；deadcode；`FuzzRender` 约 860 万次、`FuzzParse` 约 470 万次；四个 bootstrap 探针（慢请求体、孤立代理项、多余字段、截断的 JSON）；36 项反向对照<br>• **B**：md-fixtures 自检、oxlint、格式、knip、三个类型检查；vitest 1350 个连跑三遍；`vite build` 与分包体积；`make e2e` 151 个，页面的故事另跑 `--repeat-each 3`（102 个）、`--workers 1`（34 个）；vitest 75 项、构建插件 3 项、e2e 8 项反向对照<br>• **C**：完成标准与 12.5 逐条；M4 文档、移交与总体设计引用的 83 个测试名与反引号里的路径逐个核对；审查记录的"处置"抽查约 70 条；`make check`、`make gen-check`、`make e2e`；3 项反向对照 |
| 日期 | 2026-10-03 |
| 结论 | 代码没有必须修的缺陷。处理完移交（A-I1、C-I1–C-I3、B-I3）、第 13 节（A-I2、C-I6）、README（A-M4、C-I4）与页面版本的断言（B-I1、B-I2、C-I5）之后，**等负责人执行完输入法清单即可收官**：<br>• 门禁、全部故事、持续集成全绿；M4 的完成标准与 12.5 逐条满足，人工验收一项待执行（见下）；<br>• 规模：Go 生产代码约 7.8k 行、测试 9.7k 行、生成物 3.1k 行、SQL 506 行、接口描述 1.2k 行；前端生产 TS/TSX 约 4.4k 行与 CSS 0.2k 行、测试 3.9k 行，端到端 2.2k 行；<br>• 没有上帝文件：page 模块的 app 层 27 个文件，一个用例或一个单元操作一个文件；最大的 Go 生产文件是 `harden/links.go`（465 行，理由见 A-N2）；前端最大的 M4 文件是 `stores/page-editing.ts`（约 390 行，会话在 M5 加锁之前拆出，B-N5），`page-tree.tsx` 在收尾拆开；<br>• 写入单元、加锁顺序、变更集与客户端、标题键、解析时机与预算、渲染器与用户 HTML 的分界、编辑会话、清理器与笔记本删除的三条路径、编辑器的换行记录与扩展管线，逐项对照代码，与 M4 总设计第 4、5、8 节一致；15 个交错（30–44）都在、结果确定。<br>31 项发现（三位审查者的 44 条，重复的合为一项）与 6 项疑问 项发现全部处理或写明去处；修复的差异另经独立核对，核对的发现一并处理 |

## 完成标准核对

| 标准 | 结论 | 证据 |
|---|---|---|
| 所有 Phase 完成，各有审查记录 | 满足（待人工验收） | P1–P5 已完成；P6 已合并、门禁全绿，状态"进行中"，待负责人执行输入法清单 |
| 本 M 的故事全部通过（本地与持续集成）；之前各 M 的故事仍通过 | 满足 | 本地 `make e2e` 152 个；持续集成的 e2e 任务为绿 |
| 故事表每一行、每个版本都有端到端测试，并做到表里写的每一句 | 满足（修复后） | PG1–PG14 共 14 个文件，每个故事都有接口与页面两个版本。B 逐句核对，缺的句子与落库断言见 B-I1、C-M1，都已补上 |
| 对等验收：两个版本调用同一组断言；例外 | 满足（修复后） | `e2e/fixtures/assert/page.ts`；PG5、PG8、PG9、PG10 的页面版本原来没有调用接口版本的断言（C-I5、B-I2），已补；PG13 的清理只有一个版本（M4 总设计第 3 节）|
| 人工验收 | **待执行** | [输入法清单](../manual/P6-ime-checklist.md)，结果记在 [P6 审查记录](P6-source-editor-review.md)的"人工验收"一节；通过之前 M4 不改为已完成 |
| 本 M 建立的扩展点已建好，有测试证明注册者能挂上 | 满足（修复后） | `page/extension_test.go`（守卫、观察者、参与者、客户端、外层事务、会话的否决者与订阅者、扩展到达阅读视图、`BodyLimits`）、`platform/markdown` 的扩展测试、`editor/extensions.test.ts`、`reading/enhancement.test.ts`；会话的否决者与订阅者原来没有"两个注册者按次序"的测试（A-M2），已补 |
| 之前各 M 的扩展点已注册：第一个注册者经每条触发路径各有整个程序上的行为测试 | 满足 | 笔记本删除的三条路径（`TestDeletingANotebookDeletesItsPages`）、活动的来源（`TestTheOwnerlessListShowsThePagesActivity`）、清理器（`purge_test.go`）、权限规则表；A 的 R01–R24 与 C 的 3 项把注册者换成空，都有测试失败 |
| M4 自己的扩展点没有注册者：最后一跳写进第一个注册者所在 M 的移交 | 满足（收尾后） | 七个扩展点逐条写进 M5、M6、M7、M9 的移交（A-I1、C-I3，见下"handoff"） |
| 权限矩阵覆盖每个新操作，用笔记本级的列 | 满足 | `permission_matrix_page_test.go` 12 个操作都用 `notebookColumns()`，心跳与结束另有"别人的会话"一种目标 |
| 12.5：用 PAT 完整操作 | 满足 | `TestEveryOperationAcceptsAPersonalAccessToken` 由契约推导，覆盖全部页面操作；PG 的接口版本都用 PAT |
| 12.5：先写描述，再写代码 | 满足（修复后） | `api/modules/page.yaml`；`gen-check` 无差异。慢请求体的 400 原来不在描述里（A-M3），已补 |
| 12.5：架构测试、depguard、前端静态检查 | 满足 | 都为零 |
| 12.5：本 M 没有 `open` 的 handoff | 满足（收尾后，一项待人工验收） | 收到的四份：三份改为 `done`；M0/P1 编辑器的一份第 1 项前半待清单，后半转给 M5，清单通过之后改为 `done`（C-I1） |
| 文档约定的"M 完成" | 满足（收尾后，待人工验收） | 六个 Phase 各有审查记录；本记录、第 13 节、M4 总设计、README 在收尾时完成；M4 的状态在人工验收通过之后改为已完成 |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| A-I1 / C-I2 / C-I3 / B-I3 | Important | **M4 写给后面的移交没写**：M4 自己的七个扩展点都没有注册者，组合根交空集合，"最后一跳"只写在 M4 文档里；P4 审查 Q1–Q5（其中 Q1 是加锁次序上的死锁约束）、P6 第 7 节的编辑器事项、给 M6 的两条、给 M8、M9、M12 的事项都还没进目标 M 的 `handoffs/`。M5 开工时只看自己目录里 `open` 的项 | 新写 M5 的[编辑会话](../../M5-collab-editing/handoffs/M4-P4-edit-sessions.md)、[编辑器](../../M5-collab-editing/handoffs/M4-P6-editor.md)、[Markdown 扩展的约束](../../M5-collab-editing/handoffs/M4-P3-markdown-extensions.md)，M7 的[两个扩展](../../M7-assets-transfer/handoffs/M4-extensions.md)，M8 的[历史与恢复](../../M8-history-search/handoffs/M4-history.md)，M12 的[性能](../../M12-release/handoffs/M4-performance.md)；补 M5 的[页面事件](../../M5-collab-editing/handoffs/M4-P1-tree-refresh.md)（第 4 项）、M6 的[Markdown 扩展](../../M6-links/handoffs/M4-P3-markdown-extensions.md)（第 8–11 项）、M9 的[客户端](../../M9-mcp/handoffs/M4-P1-client-check.md)（第 3、4 项）。每个扩展点的最后一跳逐条写到"经哪条路径、断言什么、组合根交空时失败" |
| C-I1 | Important | 收到的四份移交都还是 `open` | 逐项对照代码：M2/P4 清理顺序、M3/P1 笔记本删除、M3 笔记本留给 M4 的五项都已落实，改为 `done` 并写明处置；M0/P1 编辑器的第 2、3 项已落实，第 1 项前半待人工清单、后半转给 M5，清单通过之后改为 `done` |
| A-I2 / C-I6 | Important | 第 13 节没有补进 M4 的约定，13.1 现有几处与代码不符（第 21 条的 `notebookRegistrants()` 等） | 见下"第 13 节" |
| C-I4 / A-M4 | Important | README 在 M4 一次都没改：没有页面一节，软删除、删除笔记本、无主清单的大小与活动、后台任务、配置的交叉校验、前端都没写 M4 | 加"页面"一节（树、正文、编辑会话、阅读视图、解析预算）；前端加"页面""编辑"两条；改正上面几行；配置一节写明两条交叉规则；部署一节写明解析预算与内存 |
| B-I1 / B-I2 / C-I5 / C-M1 | Important | **故事表的句子与页面版本的落库断言不全**：PG4 两个版本都没断言编辑会话被删除；PG5 接口版本的 `revision` 恒为 1，页面版本没有删除线、自动链接、"frontmatter 不成分隔线或标题"；PG6 接口版本没有外部图片，页面版本没有 `//外站` 链接与没闭合的标签；PG7 接口版本保存之后没读阅读视图；PG3 没断言条目的 `before_sort_order`；PG8、PG9、PG10、PG5 的页面版本没有调用接口版本的断言；PG10 的页面版本没断言 409 `page.edit_session_ended` 与新会话，标题写"完成"却按 Ctrl+E；PG12 的页面版本没有"看不到笔记本的人"；PG13 的页面版本只有一条删除路径；PG14 的页面版本没断言最后活动 | 新的夹具：`wiki-pages.ts` 的 `contentWrites`（编辑器的每次写的答复）、`assert/page.ts` 的 `placeOf` 与 `sessionsOf`；`expectSubtreeDeleted` 断言会话已删，`expectMoved` 断言移动之前的位置。逐个故事补上上面每一句：PG4 在子树与兄弟上开会话（兄弟的会话还在）；PG5 写到 revision 2 再读阅读视图；PG10 点"完成"并断言会话已删；PG13 在页面上删无主笔记本与删工作区；PG14 把各处的时刻改到不同的日子，断言列表显示最晚的页面写入的那一天。e2e 反向对照见下 |
| A-M1 | Minor | Markdown 扩展的约束只在 M6 的目录里（`to: M6`），第一个注册者却是 M5 的任务项 | M5 加一份指向它的[移交](../../M5-collab-editing/handoffs/M4-P3-markdown-extensions.md)，写明 M5 先碰到的三点（架构规则、耗时与输出、最后一跳） |
| A-M2 | Minor | 会话的否决者与订阅者没有"两个注册者、按次序、第一个错误即停"的测试：倒序调用（S01）、订阅者出错之后继续（S02）都让全部测试通过 | `page/app/session_test.go` 加 `TestTheSessionRegistrantsRunInTheirOrder`：两个否决者、两个订阅者按次序，删子树结束两个会话时每个会话都告诉两个订阅者，第一个拒绝或错误即停。S01、S02、否决者拒绝之后继续（S03）三项反向对照现在失败 |
| A-M3 | Minor | 请求体在 `read_timeout` 之后才到齐的 400 不在接口描述里，detail 与截断的 JSON 相同（"could not be decoded"），调用方会以为自己的 JSON 写坏了 | `httpserver.APIErrors.BodyError` 认出连接的期限（`os.ErrDeadlineExceeded`），detail 写明请求体没有在读超时之内到齐，码仍是 `bad_request`；`server_test.go` 的 `TestABodyPastTheReadTimeoutIsSaidToBeLate` 在真实的服务器上钉住；两个正文路由的描述写明，重新生成。前端编辑器的写不会是坏 JSON，`bad_request` 照旧说"网络太慢" |
| B-M1 / C-M2 | Minor | P6 文档写"别处删页之后外壳去父页"，代码只对本标签页删的页去父页，别处删的显示 404；"（3.12）"应为"（3.10）" | 照代码改写；M5 的编辑器移交第 6 项按实际行为写 |
| B-M2 | Minor | 编辑模式读不到正文时只有"重试"，没有回到阅读视图的按钮（Mod+E 可以） | 失败时加"完成"，回到阅读视图、结束会话；`page-edit.test.tsx` 断言。反向对照 BM2-NO-DONE 失败 |
| B-M3 | Minor | "移动到…"没有走 `useForm`（13.2 第 12 条） | 改走 `useForm`：拒绝显示在表单上方，发送中禁用；422 的 `parent_id`、`after_id` 显示在各自的选择框下、焦点移过去（修复核对 MN-2） |
| B-M4 | Minor | 不同父页下的同名页面，树里的"子页面""操作""删除"与改名、移动对话框的名称一模一样（13.2 第 17 条） | 同名（按标题键，大小写与 NFC 不计）时带上所在的位置："Notes (in Guide)"，根下的带笔记本名：`stores/page-tree.ts` 的 `placeOfTitle`（索引里按标题键计数），`page-tree-item.tsx` 的 `useDistinctName`。四项反向对照失败 |
| B-M5 | Minor | 审查的反向对照有六处存活：保留我的之前再读一次（E15，正是总设计第 4 节放弃的方案）；重叠的读与写的答复（T1、T8）；新建的答复在地址变了之后仍去新页（N2）；移动对话框的父页失效时回到顶层（V1）；不按 `Retry-After` 等（E6）；Mod+B 只有一侧有 `**`（C1） | 各补测试：冲突读到 5 之后服务端变成 6，"保留我的"仍用 5、冲突区换成 6；重叠的读用改名；导航落地之后再断言地址；被移动的页不在顶层；在 1999 与 2000 毫秒处数请求；Mod+B 一侧的两种情形。六项反向对照现在都失败 |
| B-M6 | Minor | 13.2 第 7 条的"再读一次时显示别处的变化"，左栏的树、笔记本首页、页面的标题、面包屑与子页面都没有测试；首页的读取失败也没有单独断言 | `page-tree.test.tsx` 与 `page-layout.test.tsx` 各加重读的测试；首页的"重试"单独断言。反向对照（重读的树不替换）让两处失败 |
| C-M3 / A-N6 | Minor | M6 移交里的耗时判据过时（256 KB、两倍） | 改为代码的判据：512 KB，先量四分之一，全尺寸不超过它的 8 倍加 1 毫秒 |
| C-M4 | Minor | 给 M8 的三件事没有移交（重排不记条目、回收站里的历史、恢复清空跟随的行） | 新的 M8 移交，连同 B-Q2 的 `removedTo` |
| C-M5 | Minor | 给 M12 的性能与部署事项没有移交 | 新的 M12 移交（共享锁与树写、大正文的内存与预算、先到先得、删大子树、活动的索引）；README 的部署一节写明内存 |
| C-M6 | Minor | 总体设计 12.2、12.6、4.3、第 14 节没跟上 M4 | 见下"文档与代码的不一致" |
| A-N1 | Nit | `harden/export_test.go` 的 `Hardened()` 没有调用者 | 删掉 |
| A-N2 | Nit | `harden/links.go` 465 行，是唯一超过约 400 行的生产文件，没写理由 | 保留一个文件：它改写自 goldmark 的 `parser/link.go`，与上游一个文件对应，升级 goldmark 时逐段对照；理由写进文件开头的注释 |
| A-N3 / C-N3 | Nit | `page/domain/session.go` 的注释说网页"持有同样的两个数"，没指回前端 | 改为指向 `web/apps/web/src/stores/page-editing.ts` 的 `editSessionHeartbeat`（前端只有心跳一个数）；`errors.go` 的 `ErrEditSessionEnded` 补上"别的客户端开的" |
| A-N4 / A-N5 / C-N1 | Nit | M4 总设计与代码的出入：`page.edit_session_ended` 的情形、5 MiB、`edit_sessions` 的外键、树的不变量、交错 43 与"两种先后"、耗时预算的门禁、命中区、`removedTo`、`@lezer/markdown`、"会话的状态"、心跳"不读成员行"、"只改本单元自己的行" | 照代码改写 |
| B-N1 | Nit | PG9 的组合用 `waitForTimeout(300)` | 去掉：只有一次写、写的是确认之后的文字，已经证明组合中没有保存；e2e 反向对照（Ctrl+S 不等组合结束）失败 |
| B-N2 | Nit | 快速切换滚动用的是没夹过的 `active`，树重读、列表变短之后去滚一个不存在的选项 | 改用 `current`；测试记下滚进视野的选项。反向对照失败 |
| B-N3 | Nit | 人工清单的 `$BASE`、`$PAT`、`$PAGE` 没说从哪来，第 9 步没写在哪做 | 写明 |
| B-N4 | Nit | 反向对照的较小缺口：只换查询或锚点也拦；左栏的树不按笔记本重新挂载；高亮的类名白名单放过任意小写类名；"还没有列表时再读一次"没有测试；离开编辑时先读阅读视图没有断言；构建插件没有自动测试 | 各补测试：查询与锚点、换笔记本之后失败提示不跟过去、`hidden` 与三个下划线的类名被拒（白名单收紧为一到两个下划线）、写的读取失败之后的第一次读、读阅读视图的答复扣住时还停在编辑器；构建插件抽成 `build/editor-out-of-main.ts` 的 `editorLeak`，五个用例。各项反向对照失败 |
| B-N5 | Nit | `page-tree.tsx` 一个文件里有树、拖拽、菜单与三个对话框；`page-editing.ts` 会话与保存混在一起 | 拆出 `page-tree-item.tsx`（行、拖拽、菜单）；会话的拆分写进 M5 的编辑器移交第 8 项，加锁之前做 |
| B-N6 | Nit | PG10 页面版本的标题写"Done ends the session"，按的是 Ctrl+E，没有 e2e 点过"完成" | 改为点"完成" |
| C-N2 | Nit | Phase 文档、计划与注释的残留 | 照代码改写；`markdowntest/costs.go` 的注释写的实测数（约 6 与 11）改为 P3 文档的约 4.4 与 10.3 |
| C-N4 | Nit | P6 的状态"已合并，待人工验收"不在约定的取值里 | "进行中"加备注 |
| C-N5 | Nit | `docs/README.md` 的目录结构没有 `manual/` | 补上 |

## 疑问与判断

| # | 疑问 | 判断 |
|---|---|---|
| A-Q1 | 解析预算（P4 的决定，负责人可改判）与文档一致；三点风险：① `parse_max_wait` 不短于请求期限时，阅读视图先到期限、答 500 而不是 503；② 下限 5 MiB 最坏约 1.5 GB，内存小的机器靠配置压不下来；③ 预算只管 page 模块，M6 参与者的解析不经它 | ① 加交叉规则 `page.parse_max_wait < server.request_timeout`（`config/validate.go`，表格测试的边界两行；反向对照 `>` 失败），README 与样例配置写明；先例 `auth.password.max_wait` 原来也没有这条，修复核对之后一并补上；② 接受，README 的部署一节与 [M12 的移交](../../M12-release/handoffs/M4-performance.md)第 2 项写明内存；③ 写进 [M6 的移交](../../M6-links/handoffs/M4-P3-markdown-extensions.md)第 9 项 |
| A-Q2 | 笔记本删除不发页面事件，M6 的索引怎样跟上 | 由 M6 二选一（注册笔记本删除并软删，或读取时按 `nodes.deleted_at` 过滤），并修订 12.4：M6 移交第 10 项 |
| A-Q3 / B-Q3–B-Q6 / C-Q1 | M4 总设计第 8 节"收尾的待定项"四项 | 三位的建议一致处照办：<br>• `document.title`：收尾时做。`app/document-title.ts` 的 `useDocumentTitle`，每个页面在主标题处调用（标题、所在的位置、"Nerve Wiki"），页面离开时恢复为应用名；`document-title.test.tsx` 走一遍页面、笔记本首页、笔记本与工作区的设置、账户设置、404、登录，七项反向对照失败（WCAG 2.4.2）。<br>• 同站链接的应用内跳转：转给 M6（"链接跳转"的增强），移交第 11 项。<br>• 删除与新建之后的导航：转给 M5，与"别处删掉正在编辑的页"一起设计，编辑器移交第 6、7 项。<br>• 宽表格：先做过渡，阅读视图与代码块在比显示的宽时可以聚焦（`reading/scroll-focus.ts`，WCAG 2.1.1），焦点有可见的轮廓；PG5 的页面版本用一个宽表格断言阅读视图可以聚焦（不注册这个增强时失败）。每个表格自己滚动、可以聚焦的外包层要改渲染器、`Markup` 与 `CheckHTML`，转给 M6，移交第 11 项 |
| B-Q1 | 扩展点的上下文够不够 12.4 的注册者用：`ReadingContext` 没有应用内导航与写入，`EditorContext`、`EditorControls` 没有会话的状态、页面树或上传 | 按需要的 M 各自加：M5 的编辑器移交第 5 项（会话与锁的状态、阅读视图的写入），M6 的移交第 8、11 项（补全要页面树、跳转要导航），M7 的移交第 2 项（上传）；M4 总设计第 8 节原来写的"会话的状态"不存在，已改正 |
| B-Q2 | `PageTreeStore.removed` 一代之内从不清除 | M4 没有恢复，不清除是对的；M8 的恢复要从表里去掉，M8 移交第 4 项 |
| C-Q2 | P4 审查 D-d"`testConfig` 不经校验"没有去处 | 不做：`validate` 是 config 包的私有函数，`testConfig` 是只填测试要用的键的最小配置（没有签名私钥、日志等），过校验要么为测试导出 `validate`，要么补全全部键。新的定时任务间隔为 0 时测试会立刻暴露（River 不停地排它，D-d 正是这样发现的），代价可控 |

## 第 13 节

由一位 Opus 起草、作者逐条对照代码核对，写进[总体设计](../../v0.1-design.md)第 13 节（第 15 节记一行修订）：

- **13.1 后端**：第 1 条加"页面的写入单元"（`Writer.Run` 的次序，M6–M9 的写都是建 `UnitSpec` 加操作）与"不经写入单元的"（心跳、结束、清理、开启会话、与当前正文相同的保存）；第 2 条加客户端与变更集；第 3 条点名 `TestTheTreeIsWhatEachReadAllows`；第 4 条写带正文的写的码的次序（含 503）与两种会话码的划分；第 5 条改正会话行的锁（`FOR UPDATE`，锁下核对四项）、否决者与订阅者的锁、外键检查与交错 30–44；第 6 条点名自引用外键的三个测试，写明 `edit_sessions` 不是软删除，注明模块内的 `CASCADE` 只作兜底；第 9、10 条加会话的时钟与页面的日志字段；第 11 条加 `BodyLimits()` 与 page、notebook 给组合根的构造；第 14 条加按路由放宽请求体上限与慢请求体的 400；第 15 条加两边各一份的心跳与三条交叉规则；第 19 条加"判定之后才做昂贵的事"与解析预算；第 21 条加参与者与观察者、两个"两个注册者"测试、组合函数（改正原来的 `notebookRegistrants()`）、会话订阅者的两处接线与 M4 没有注册者的七个扩展点；第 23 条加 `page.cleanup_expired_edit_sessions`；新增第 28 条标题键。
- **13.2 前端**：第 1 条写页面树写队列与编辑保存的两处例外；第 6 条写 `editor/`、`reading/` 的依赖方向（收尾时加进 oxlint）；第 7 条写阅读视图只在 SWR、正文不缓存、保存自己的重试；第 8 条写编辑器不进主包的构建检查与 Worker 的 CSP；第 10、16 条加页面的外壳与 `removedTo`、新建之后核对 location 的 `key`；第 15 条加页面树与阅读视图的键；第 17 条加同名页面带位置；新增第 20–24 条：快捷键、未保存的提醒、标签页的标题、前端的扩展管线（上下文按需要的 M 加字段）、横向滚动要能用键盘。
- **13.3 Markdown**：第 1 条加编辑器的换行记录；新增第 3–5 条：解析时机、渲染器的标记与用户 HTML 的分界、扩展的约束。
- **13.4 测试**：第 3 条加 `page.ts`；第 4 条加页面交错持的行、`interleaveBehind`、定时任务的交错与 `checkPages`；第 6 条加"别人的会话"这种目标；新增第 8 条耗时与分配的检查。
- 同时改了 4.3（耗时与内存按 M4 的结果）、8.4（交叉规则）、12.2 的 M4 一行、12.6 的状态（"进行中（待负责人执行输入法清单）"）与第 14 节的风险表（输入法、耗时与内存两行改写；新增 MultiXact 与树写的延迟、正文写入单元的 `lock_timeout` 两行，都链到 M12 的移交）。

起草时按代码改正了审查者的几处说法：class 不只 `nw-` 前缀（还有 `language-*` 与 goldmark 脚注的）；`PageTreeStore` 对外只有 `removedTo`，没有 `wasRemoved`；页面的交错还持变更集行；保存除了 B 写的还有 503 的重试与 409 之后重开会话；编辑器的扩展现在只能在 `source-editor.test.tsx` 里注入。12.4 笔记本删除事件那一行留给 M6 决定之后修订（M6 移交第 10 项）。

## handoff

- **收到的**：[M2/P4 清理顺序](../handoffs/M2-P4-purge-page-tree.md)、[M3/P1 笔记本删除](../handoffs/M3-P1-notebook-deletion.md)、[M3 笔记本留给 M4 的部分](../handoffs/M3-notebooks.md)逐项对照代码都已落实，改为 `done`；[M0/P1 编辑器](../handoffs/M0-P1-editor.md)第 2、3 项已落实，第 1 项前半待人工清单、后半转给 M5，清单通过之后改为 `done`。
- **M4→M5**：新的[编辑会话、写入守卫与锁](../../M5-collab-editing/handoffs/M4-P4-edit-sessions.md)（P4 审查 Q1–Q5、后台标签页的心跳、否决者与订阅者与守卫的最后一跳）、[编辑器](../../M5-collab-editing/handoffs/M4-P6-editor.md)（输入法与自动保存、推送与撤销、人工清单的后半、扩展管线与它的最后一跳、别处删页、删除与新建之后的导航、先拆出会话）、[Markdown 扩展的约束](../../M5-collab-editing/handoffs/M4-P3-markdown-extensions.md)（指向 M6 的那份，任务项先到）；[页面事件与树的重读](../../M5-collab-editing/handoffs/M4-P1-tree-refresh.md)加第 4 项（观察者的最后一跳，连同不触发的路径）。
- **M4→M6**：[Markdown 的扩展](../../M6-links/handoffs/M4-P3-markdown-extensions.md)改第 4 项的判据，加第 8–11 项（参与者、观察者、扩展与补全的最后一跳；P4 留下的两条与多页加锁；笔记本删除不发页面事件；同站链接与宽表格）。
- **M4→M7**：新的[附件的 Markdown 扩展与粘贴上传](../../M7-assets-transfer/handoffs/M4-extensions.md)。
- **M4→M8**：新的[页面的历史、回收站与恢复](../../M8-history-search/handoffs/M4-history.md)；[M9 的单元合并](../../M9-mcp/handoffs/M4-P2-unit-merge.md)的末句改为链过去。
- **M4→M9**：[客户端名称的检查](../../M9-mcp/handoffs/M4-P1-client-check.md)加第 3 项（`mcp:<名>` 的最后一跳）、第 4 项（写入选项 `UpdateLinks`）。
- **M4→M12**：新的[页面的性能与部署](../../M12-release/handoffs/M4-performance.md)。

`M4-pages/handoffs/` 只剩 M0/P1 编辑器一份 `open`，等人工清单。

## 文档与代码的不一致

照代码改写（一位 Opus 起草、作者核对；C-M6、A-N4、A-N5、C-N1、C-N2、B-M1、C-M2）：

- **[M4 总设计](../00-M4-design.md)**：第 4 节耗时的门禁是 `markdowntest.CheckCosts`（基准手动运行）；只有本标签页删除的页去父页（`removedTo`），别处删除的显示 404；拖拽的命中区用 `list-item`；编辑器用 `markdownLanguage`；`edit_sessions` 不建指向页面各表的外键；树的不变量加上已删节点的跟随行。第 5 节 `page.edit_session_ended` 加"别的客户端开启的"、5 MiB。第 7 节心跳与结束不锁成员行。第 8 节编辑器的控制只有 `saving()`、没有"会话的状态"；阅读视图的增强加上横向溢出的聚焦；变更集、条目与版本的改写范围；交错 43、44 各只有一种先后；"收尾的待定项"写成结论；"写进移交"都链到移交。
- **Phase 文档**：P1 的排序下标（`len-1`）与 P4 起 `NewNotebookDeletion` 的签名；P3 的标题 id 在 `parse.go`、耗时先量四分之一；P4 的活动查询、参与者测试的位置、交错 43、44 各只有一种先后、给 M5 与 M6 的事项链到移交；P5 的已知差异记在本节（还没有帮助文档）；P6"别处删页之后外壳去父页"照代码改写、"（3.12）"改为"（3.10）"，状态"进行中"加备注（C-N4）。
- **计划**：P1-S1（那句注释所在的文件）、P1-S5（夹具的名字与返回）、P2-S1（`Subtree` 的范围与次序）、P5-S2（`wasRemoved` 改为 `removedTo`）、P6-S1（`@lezer/markdown` 改为 `markdownLanguage`）。
- **[人工清单](../manual/P6-ime-checklist.md)**：`$BASE`、`$PAT`、`$PAGE` 从哪来，第 9 步在哪做（B-N3）。
- **[P5 审查记录](P5-tree-reading-review.md)** 的一处节号（3.7 改为 3.6）；`docs/README.md` 的目录结构加 `manual/`（C-N5）。
- **README**（随代码提交）：页面一节，软删除、笔记本删除、无主清单的大小与活动、后台任务、配置的交叉规则、部署的内存，前端的"页面""编辑"两条（C-I4、A-M4，修复核对 NT-1、NT-4）。
- **代码注释**：`page/domain/session.go` 指向前端的 `editSessionHeartbeat`，`errors.go` 的 `ErrEditSessionEnded` 补"别的客户端开的"（A-N3、C-N3）；`harden/links.go` 写明为什么是一个文件（A-N2）；`markdowntest/costs.go` 的实测数（C-N2）。

## 反向对照

审查者做的：
- **A**：36 项中 34 项失败：笔记本删除的三条路径与工作区删除的接线、软删除的四类行、删会话、清理器的有无与先后、`nodes` 的循环、活动的接线与取值、规则表六条、删子树删会话（R01–R24）；加锁（L01–L06，其中 L03 只被 page/app 的替身测试测出）；会话（S03–S06）。存活的 S01、S02 见 A-M2。
- **B**：vitest 75 项中 62 项失败，存活的 13 项见 B-M5、B-N4（其中一项让 vitest 挂起，见下）；构建插件 3 项；e2e 8 项中 7 项失败，存活的一项（Mod+O 不阻止浏览器的默认动作）由 vitest 守住。
- **C**：笔记本删除与工作区删除的注册者、活动的来源换成空，3 项全部失败。

作者在修复中做的（全部按预期失败，改动全部还原）：
- **Go 与前端 37 项**：会话的否决者倒序、订阅者出错之后继续、否决者拒绝之后继续（A-M2）；慢请求体的 detail（A-M3）；`parse_max_wait` 的边界（A-Q1）；标签页标题七项（页面不带笔记本、离开不恢复、三处设置不带分节、笔记本首页不带工作区、登录不设）；读不到正文时没有"完成"（B-M2）；移动对话框不显示拒绝（B-M3）；同名不带位置、计数的阈值、不按标题键计数、改名对话框的标题（B-M4）；快速切换滚 `active`（B-N2）；B-M5 的六项与"还没有列表时再读一次"；重读的树不替换（B-M6）；查询变化也拦、树不按笔记本重新挂载、高亮白名单的两种放宽、离开时不先读阅读视图、构建插件只沿静态 import、不跳过编辑器的入口（B-N4）；横向滚动的聚焦恒为真、不观察尺寸。离开时先读阅读视图、查询变化两项第一次存活：测试在重新渲染之前就导航了、在轮询的间隔里看不出闪烁，改成等"未保存"之后再导航、扣住阅读视图的答复再断言，之后失败。
- **e2e 12 项**（每项重建二进制，只跑对应的故事）：删子树不删会话（PG4）；条目的 `before_sort_order` 改掉（PG3）；阅读视图的 `revision` 恒为 1（PG5 接口）；不渲染删除线、不注册横向滚动的聚焦（PG5 页面）；放行 `//外站` 的地址、不补闭合（PG6 页面）；编辑器的写不带会话（PG8 页面）；Ctrl+S 不等组合结束（PG9）；笔记本 404 不显示（PG12 页面）；笔记本删除不删页面（PG13 页面）；活动取最早的写入（PG14 页面）。
- **修复核对之后**（全部按预期失败）：前端 8 项：新建的答复在走开之后仍去新页（换成记录导航之后重跑）、嵌在邀请页里的也设标题、整页替换时不设标题、移动对话框不列字段、选择框不标出错、选择框不带描述、邀请表单仍列 `role`、阅读视图没有名称；Go 2 项：哈希等待的边界放宽为 `>`、去掉这条规则；oxlint 1 项：`reading/` 导入 store；e2e 2 项：阅读视图不能横向滚动（`overflow-x: hidden`，PG5 页面）、会话的写不记进会话（PG10 页面）。

审查期间的一件事：B 的子审查跑反向对照时，一项变异（会话结束的 409 无限重开）让 vitest 挂起，留下三个进程（82440、82441、82460），子审查自己结束它们被权限分类器拒绝，B 随后结束了这三个进程；作者核实它们都已不在、B 的目录下没有残留进程。

## 核实修复

- 修复在分支 `m4-closeout`：`5a4a450`，以及修复核对之后的 `e673a3b`（两个提交共 80 个文件，+1533/−472），合并 `af134f8`。每个提交之后本地 `make check`（最后一次 vitest 1379 个）、`make gen-check`、`make e2e`（152 个）为绿，上文的反向对照都按预期失败，改动全部还原、重建干净。
- 合并提交上 `make image-smoke` 为绿。
- 持续集成：分支的两个提交（run 37109005941、37111005570）与合并提交（run 37111535721）四个任务都为绿。

修复的差异（`5a4a450`）另经一位独立审查者核对，没有 Major，Minor 四项、Nit 六项、疑问一项，处理在 `e673a3b`：

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| MN-1 | Minor | 邀请页里嵌着的"会话暂时不可用"也设标签页的标题，盖掉邀请页的，恢复之后标题成了应用名 | 标题移到整页替换时用的 `SessionUnavailablePage`（两处守卫），嵌在邀请页里的不设；`guards.test.tsx` 断言整页时的标题，`invitation.test.tsx` 断言不可用时与恢复之后都是邀请页的标题 |
| MN-2 | Minor | 移动对话框走了 `useForm` 却没列字段：422 的 `parent_id`、`after_id` 只在表单上方显示通用的话，选择框不标出错，焦点也不移过去（13.2 第 12 条） | 列出两个字段，问题显示在各自的选择框下（`aria-invalid`、`aria-describedby`，新文案 `field.parent_id.not_allowed`、`field.after_id.not_allowed`），焦点移到出错的那个；`page-tree-writes.test.tsx` 两个字段各一例。顺带发现邀请表单把 `role` 列为显示的字段，选择框下却不显示，`role` 的 422 什么也不显示：改为不列，落到表单上方，加测试 |
| MN-3 | Minor | 可以聚焦的横向滚动区域没有名称 | 阅读视图以页面的标题为名称（`aria-label`），`page-layout.test.tsx` 按名称找它；代码块的名称随表格的包裹层转给 M6（[移交](../../M6-links/handoffs/M4-P3-markdown-extensions.md)第 11 项） |
| MN-4 | Minor | PG10 的页面版本没证明重发的保存在新会话里 | 点"完成"之前 `expectOneSessionRevision(db, 新会话, 页, 2, 3)` |
| NT-1 | Nit | README 三处与代码不符：会话"不是这个客户端（网页或某个令牌）开的"，实际要求本人、这一页、同一种客户端（网页，或任一令牌）；"期间有别人的写"，实际是任何别的写，含本人在别处的；无主清单的"最后活动"，实际取笔记本自己的修改与页面写入中较晚的 | 照代码改写 |
| NT-2 | Nit | `tsconfig.json` 的注释说 Vite 的原生加载器"需要"扩展名，实际是不带时警告 | 改写注释 |
| NT-3 | Nit | "新建的答复在用户走开之后不跳转"的测试用 50 毫秒的固定等待 | 改为 `router.subscribe` 记下每次导航（导航一开始就在路由的状态里），断言按钮恢复之前没有去新页 |
| NT-4 | Nit | `parse_max_wait` 只写"必须短于"请求期限，认证与判定也要时间 | README 与样例配置写"且应明显短于它" |
| NT-5 | Nit | PG5 只断言阅读视图带 `tabindex`，没有真的用键盘滚动 | 聚焦阅读视图、按右方向键，`scrollLeft` 变大 |
| NT-6 | Nit | 慢请求体的测试用裸 handler，没经过请求体上限与形状检查的中间件 | 不补：核对者读过这条链，`MaxBytesReader` 与 `io.ReadAll` 原样传出 `os.ErrDeadlineExceeded` |
| Q-1 | 疑问 | 反向对照的日志与清单有两项不一致 | 那两项第一次存活、补测试之后失败，日志是第一轮的；本记录按最新一轮写 |

起草第 13 节时又对照代码定了几处：
- 13.1 第 6 条"父行的删除不经 `ON DELETE CASCADE`"与迁移里模块内的 `CASCADE`（`page_contents`、`page_revisions`、`changeset_items`）字面不符：行为一致（清理器先查 `NOT EXISTS`，级联不触发，P1 文档写明"级联只是兜底"），在约定里注明，不改迁移。
- `editor/`、`reading/` 原来没有 oxlint 的目录规则：加上（不导入 `app/`、`stores/`、`pages/`、`components/`、`onboarding/`、`session/`），临时加一个导入 store 的语句，oxlint 报错。
- A-Q1 提到的先例 `auth.password.max_wait` 也没有"短于请求期限"的交叉规则，等哈希名额同样会先撞上期限、答 500：一并补上（`validate.go`，边界两行；反向对照 `>`、去掉规则都失败），README 与样例配置写明。

## 没能验证的风险

- 只用 macOS 上的 headless Chromium：Firefox、Safari 的行为（含宽表格在 Safari 上的键盘滚动）、窄屏布局、读屏的实际朗读没有验证；真实输入法由人工清单覆盖。
- 交错与清理的确定性只在本机、Docker Desktop 下验证；审查期间别的会话同时在跑测试容器，testcontainers 偶有启动超时，重跑干净，是环境问题。
- 性能：大页面的并发阅读与保存、共享锁下树写的延迟、删大子树持锁的时长只做了推理或单机实测，见 [M12 的移交](../../M12-release/handoffs/M4-performance.md)。
