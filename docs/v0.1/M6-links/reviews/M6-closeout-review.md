# M6 链接与 Obsidian 方言：收尾审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | `main` 的 `557b03f`（M6 的七个 Phase 全部合并、P7 文档写好之后），M6 的改动是 `de53433..557b03f`（167 个提交，497 个文件，+40948/−1181）。对照[文档约定](../../../README.md)的"M 完成"、[M6 总设计](../00-M6-design.md)、七个 Phase 文档与审查记录、收到的移交、[输入法清单](../../M4-pages/manual/P6-ime-checklist.md)第 16–18 步与[总体设计](../../v0.1-design.md)第 12、13 节 |
| 审查方式 | 三位独立审查者（Opus）并行（A 后端、B 前端与端到端、C 完成标准与文档），各在自己的 `git archive` 快照上做探针，仓库与别的 Docker 容器都没动过：<br>• **A**：`make lint-go`、`go test -race`（64 个包）、`make gen-check`、样例集自检、耗时测试、四个模糊测试各 180 秒、随机测试加种子、交错 `-count=10`；在快照的副本里写探针量链接很多的页、很多同名的页、没有统计的查询、预算与 panic；组合根的注册者与最后一跳 18 项反向对照<br>• **B**：`lint-web`、knip、vitest 连跑 3 遍（1,872 个）、`make build`、`make e2e` 两遍（199 个），L 系列 `--repeat-each 3`；CDP 降速 4 倍与 8 倍、同时跑 vitest 时重复 M6 的 34 个故事；在 Chromium 里量读者与编辑者的代价（一段 20 万条链接、20 万个标签、4 MB 的标题、2 万个带公式的标题）；mermaid 标签与写者 HTML 的伪造；vitest 110 项、e2e 12 项反向对照<br>• **C**：样例集自检、`gen-check`、`lint-web`、knip、`lint-go`、架构测试、L 系列 16 个故事；逐条对照完成标准、故事表、扩展点表与第 13 节；七份审查记录里 60 条"已修、已加测试"抽查 |
| 日期 | 2026-10-07 |
| 结论 | 处理完链接很多的页（A-I1）、编辑器解析的平方耗时（B-I1）、移交（C-I1、C-I3）、附件嵌入的接缝（C-I2）、第 13 节（C-I4、A-M6、B-M4）、README（C-I5）与 L 系列的对等验收（B-I2、C-I6）之后，**等负责人执行完输入法清单即可收官**：<br>• 门禁、全部故事、持续集成全绿；M6 的完成标准与 12.5 逐条满足，人工验收一项待执行（见下）；<br>• 两条读者与编辑者的代价（A-I1：一页 87 万条链接，保存 11–15 秒、同一笔记本的保存都在等；B-I1：一段 5 万个 `[[x]]` 进入编辑 9.5 秒）收尾时修掉，深层嵌套与更大的规模写进 M12 的移交；<br>• 负责人可以推翻的决定汇总在下面一节 |

## 完成标准核对

| 标准 | 结论 | 证据 |
|---|---|---|
| 所有 Phase 完成，各有审查记录 | 满足 | P1–P7（P3、P6 各分 A、B）完成，各有审查记录与修复的核对 |
| 本 M 的故事全部通过（本地与持续集成）；之前各 M 的故事仍通过 | 满足 | 本地 `make e2e` 203 个；持续集成的 e2e 任务为绿 |
| 故事表每一行、每个版本都有端到端测试，并做到表里写的每一句 | 满足（修复后） | L1–L6 共 6 个文件。B、C 逐句核对：L2、L4、L5、L6 原来只有页面版本（B-I2、C-I6），L1 页面版本没点 Markdown 链接，L3 没核对改写的作者，L6 没有编辑时的右栏（B-M5），都已补上 |
| 对等验收：两个版本调用同一组断言；例外 | 满足（修复后） | 新的 `e2e/fixtures/assert/links.ts`（`expectIndexedLinks`、`expectIndexedTags`、`expectIndexedAliases`，索引按正文的当前版本）；接口版本经 PAT 调 M6 的六个读接口，`links` 事件经 `openEvents` 断言载荷。例外写进 M6 总设计第 9 节：只在浏览器里发生的（应用内跳转与焦点、对话框与键盘、KaTeX 与 mermaid、补全的列表与输入法、右栏的布局，以及大页故事）只有页面版本 |
| 人工验收：输入法清单第 16–18 步 | **待执行** | 结果记在下面"人工验收"一节；通过之前 M6 不改为已完成（M6 总设计第 11 节第 3 项） |
| 本 M 建立的扩展点已建好，有测试证明注册者能挂上 | 满足（修复后） | 链接解析（`obsidian.Options.Resolve`，组合根经 `markdownExtensions(resolve)`）、改写的参与者、索引的观察者、`links` 事件类型、前端的补全与增强各有经组合根的测试。附件嵌入的渲染原写"M6 建立、交空"，收尾时改为 M7 建立（C-I2，负责人可以改判） |
| 本 M 注册的扩展点：经每条触发路径各有整个程序上的行为测试，组合根交空时失败 | 满足 | `bootstrap/links_*_test.go`、`interleavings_links_test.go`、`interleavings_rewrite_test.go` 与 e2e；A 的组合根反向对照 18 项全部失败 |
| 权限矩阵覆盖每个新操作，用笔记本级的列 | 满足 | `listBacklinks`、`getPageProperties`、`listTags`、`getTag`、`listLinkTargets`、`getLinkLanding` 用笔记本级的列；落点只给能写的人 |
| 12.5：用 PAT 完整操作 | 满足 | `TestEveryOperationAcceptsAPersonalAccessToken` 由契约推导；L1–L6 的接口版本都用 PAT |
| 12.5：先写描述，再写代码 | 满足 | `api/modules/linking.yaml`；`gen-check` 无差异 |
| 12.5：架构测试、depguard、前端静态检查 | 满足 | 都为零 |
| 12.5：本 M 没有 `open` 的 handoff | 满足（收尾后） | 收到的移交都改为 `done`（C-I1） |
| 文档约定的"M 完成" | 满足（收尾后，待人工验收） | 七个 Phase 各有审查记录；本记录、第 13 节、M6 总设计、README 在收尾时完成；M6 的状态在人工验收通过之后改为已完成 |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| A-I1 | Important | **链接很多的页**：一页 5 MiB 的 `[[a]] ` 是 87 万条链接，一次保存 11–15.5 秒、堆 1.2–2.0 GiB，同一笔记本的另一次 11 字节的保存在索引锁上等 10.8 秒；八本笔记本错开保存时 6.1 GiB，45 秒时答 500。解析预算只管解析与提取结果（留十分之一），观察者把提取结果转成索引的行、写进数据库时的工作集不在预算里；只有标签与别名有上限 | 索引每页至多记前 10,000 条链接（`linking/domain.MaxLinks`，`ff6b042`）：超过的不进反链与"指向这一页"，阅读视图对索引里没有的即时解析，照样跳到它们的页；改写按索引找链接，所以它们在改名、移动时不改写（P3 文档 3.2 写明）。`TestThePageFactsKeepTheFirstMaxLinks`；整个程序上 `TestAPagesLinksPastTheIndexsBoundLeadWhereTheyResolve`（10,000 条之后的一条在阅读视图里仍解析）。配置注释、P2 文档、M12 的性能移交第 6 项随之改写（`a895bcf`）；最坏的保存约是 5,000 条时（70 ms）的两倍。负责人可以改判上限 |
| A-I2 / C-I2 | Important | 附件嵌入的渲染扩展点没建：`obsidian.Options` 只有 `Resolve` | 改由 M7 建立（负责人可以改判）：它的形状取决于附件能被解析，M6 的解析只答页面，现在建只能是一个整个程序上走不到的接口。M6 总设计第 8 节、总体设计 12.4 改写，接缝写进 [M7 的移交](../../M7-assets-transfer/handoffs/M6-links.md)第 1 项（`7957264`、`38ca3b8`） |
| A-I3 / C-I3 | Important | 写给后面的移交缺 M7、M8、M10，M9 缺反链与索引的工具，M12 缺几项 | 见下"handoff" |
| B-I1 | Important | **编辑器解析一段之内的行内语法按方括号数的平方增长**（`@lezer/markdown` 1.7.2）：每成一个链接都把前面的标记扫一遍。一段 5 万条 wikilink（300 KB）进入编辑 9.5 秒，20 万条 89 秒（一个任务 80.7 秒），每按一个键整段重新解析；阅读视图是线性的。M4 的 Markdown 链接就有，M6 让 `[[` 成了主要语法 | 给 lezer 打补丁（`2206325`，`patches/@lezer__markdown@1.7.2.patch`，理由写在 `pnpm-workspace.yaml`）：链接标记的失效从上次失效到的下标起扫；作者另找到同类的两处，配不上的 `]` 往回扫完整段、配不上的 `*`、`_` 同样，前者记下"之下没有链接开始标记"的下标，后者照 CommonMark 的 openers_bottom 按配对所依赖的（类型、是否也能开、长度模 3）记下限，开始标记被截短时把下限降到它。解析的树与原版逐字相同：360 万段随机的行内语法（各种语法的词元，以及只有强调的字母表）逐节点比较没有差异；普通文档的解析时间不变。`editor/markdown.test.ts`：一段 10 万个链接、链接与配不上的 `]`、配不上的 `*` 各在 1 秒之内（换回原版分别 20、25、15 秒）。深层嵌套仍不是线性的，原版本来如此，写进 [M12 的移交](../../M12-release/handoffs/M4-performance.md)第 9 项 |
| B-I2 / C-I6 / A-M7 | Important | L2、L4、L5、L6 只有页面版本，第 9 节没写例外；e2e 里没有一处查索引的表，M6 的六个接口从没经 PAT 调过，`links` 事件没断言过载荷 | 补 L1–L6 的接口版本与 `assert/links.ts`（`664b86f`）；B 的报告在这之前的快照上，之后补了 L5 两页同名时链接写成路径、L6 反链的分页（`2b907e0`）。例外写进 M6 总设计第 9 节 |
| C-I1 | Important | 收到的 [M4/P3 Markdown 扩展](../handoffs/M4-P3-markdown-extensions.md)、[M5 事件类型](../handoffs/M5-events.md)仍是 `open` | 逐项对照代码改为 `done`，写明落实（`38ca3b8`） |
| C-I4 / A-M6 / B-M4 | Important | 第 13 节与 6.1 没跟上 M6：预算的取法、`locks`、扩展管线、事件类型、交错的编号、窄端口、派生索引表、写者内容的代价、依赖的补丁等 | 见下"第 13 节" |
| C-I5 | Important | README 写"新建、改名、移动不受锁限制"（M6 起改名、移动要改写的页被锁时答 409）；缺六个接口、方言的标记、前端的功能、`reindex` 命令 | 照代码补上（`38ca3b8`） |
| A-M1 | Minor | 解析带路径的目标要扫过最后一段同名的每一页：一万页都叫 `x`、10 万条互不相同的 `[[gN/x]]` 保存 5.0 秒，新建一页 `x` 5.7 秒 | 有了 A-I1 的上限，一页至多 10,000 条，约 0.5 秒。更快的做法（候选按倒数第二段分组、对"目标数 × 同名页数"设上限）写进 [M12 的性能移交](../../M12-release/handoffs/M4-performance.md)第 8 项（负责人可以改判） |
| A-M2 | Minor | 读链接目标的递归查询在没有统计时是平方的：2 万个节点、没有 ANALYZE，31–39 秒（之后 18 毫秒） | 递归的一步按父节点的主键取（`CROSS JOIN LATERAL … WHERE p.id = c.parent_id LIMIT 1`，笔记本与 `deleted_at` 的条件放在子查询之外，`29e3eb1`）：没有统计时 19 毫秒。`TestLinkTargetsReadWithoutStatisticsInTimeAsLongAsThey`（2 万个节点、不 ANALYZE、3 秒之内）在中间一版（条件留在子查询里，仍走部分索引，13 秒）上失败。M7 的移交写明导入之后 `ANALYZE`（`a895bcf`） |
| A-M3 | Minor | 属性链接按值配对是"链接数 × 标量数"：9,990 项 114 毫秒 | 标量按路径建表（`scalarsByPath`，`ff6b042`）；`TestThePropertyLinksFindTheirStringsInTimeAsLongAsThey`（2,000 与 8,000 项之比小于 8）。反向对照（扫全部标量）失败 |
| A-M4 | Minor | `ParseNow` panic 时预算不还；改写在 `ParseNow` 与 `u.Defer(now.Release)` 之间有窗口 | `ParseNow` 用"取到了"的标记，panic 或出错时 `defer` 释放；改写在取到之后立即 `defer` 释放，交给单元之后才不释放（`ff6b042`）。`TestAParseNowThatPanicsHoldsNoneOfTheBudget`；反向对照见下 |
| A-M5 | Minor | 交错只核对 `checkLinks`，不核对"索引等于重建"；`TestReindexSkipsANotebookDeletedMeanwhile` 不调 `checkPages` | 每个链接与改写的交错结束时 `checkRebuilt`（重建一遍、比较前后，`e19ad68`）；那个测试补上 `checkPages` 与 `checkRebuilt` |
| B-M1 | Minor | 阅读视图增强的注册表去掉 `math`、`diagrams`、`scrollRegions`、`unresolvedLinks` 任一项，vitest 全过；P6 文档写"vitest，经组合根的表" | e2e 的 L2、L4、PG5 都抓住（B 实测）。改正 P6 文档：经表的 vitest 只有 `appLinks`、`taskToggle`，其余在表里的最后一跳由 e2e 守住（`2b907e0`） |
| B-M2 | Minor | 两页同名时反链里分不出是哪一页（13.2 第 17 条）；B 也指了被锁页的列表 | 反链改用 `distinctName`（`dfd5d3b`），新测试两页"Install"分别显示"Install (in Guide)"、"Install (in Plans)"。被锁页的列表的调用处本来就传 `distinctName`，不改 |
| B-M3 | Minor | 右栏缺 13.2 第 7 条的测试：反链与属性第一次读不到时说明原因、重试之后加载；属性再读一次时显示别处的变化 | 三个测试（`dfd5d3b`），反向对照（重试什么也不做、属性不在聚焦时重读）失败 |
| B-M5 | Minor | 故事与文档的几处：L4 第三个故事的名字写了没有的步骤（"重新布局立即完成"），也不计时；L1 文件头说 Markdown 链接会跳过去，页面版本只点了 wikilink，本站完整地址经路由没有 e2e；C4 的注释与 M5 总设计仍写"改名、移动照常"；L6 没有编辑时的右栏、属性的文字；L3 页面版本没核对作者 | L4 改名，P6 文档写明 1 秒的上限由 `math.test.ts` 以假时钟核对；L1 页面版本点 Markdown 链接与完整地址，各经路由、不重新加载；C4 的注释与 M5 总设计 C4 一行加上 M6 的条件；L6 核对属性的键与值、编辑时右栏仍有反链与属性、没有大纲；L3 核对 `content_updated_by` 是改名的人（`2b907e0`）。L5 页面版本两页同名时补全写成路径不加：vitest（`link-completion.test.ts`）覆盖，接口版本断言了 `link` |
| C-M1 | Minor | 文档写 Obsidian 1.13.7，隔离的实例报告的是 1.12.7 | 改为 1.12.7（`7957264`） |
| C-M2 | Minor | M6 总设计几处与代码不符：样例数 67、"模糊测试"实为固定种子的随机测试、`Extract(Tree)`、第 5 节缺 `getLinkLanding`、第 8 节缺 `Extension.Links`、`Linker`、`Hider`，迁移 00025、00026 | 照代码改写（`7957264`） |
| C-M3 | Minor | 总体设计 12.2、12.4、12.6、第 14 节、第 15 节、9.3、6.1 没跟上 M6 | 12.2、12.4、9.3、6.1、第 14 节在 `7957264`；12.6 与第 15 节见下"第 13 节" |
| C-M4 | Minor | 输入法清单第 16 步编辑的"IME 一"已在第 15 步改名；"转给 M6"的一步已撤销；折叠 callout 的锚点在别的引擎待确认 | 第 16 步改为"IME 四"并说明；写明撤销的那一步；Safari、Firefox 的锚点进 M12 的打磨移交第 13 项（`38ca3b8`） |
| C-M5 | Minor | "HTML 里的链接与提取结果一致"（总体设计 4.3）没有测试 | `TestTheFixturesRenderedLinksAreTheirExtractedLinks`（`660ae2a`）：78 个样例逐个渲染，HTML 里每个链接都是提取到的，提取到的都渲染了，注释里的与链接文字里的 wikilink 逐个列出。反向对照（不渲染引导的那一条、嵌入不带标记）失败 |
| A-N1 | Nit | `GET tags/%2F` 答 404 `not_found`，契约没写 | `linking.yaml` 的 Tag 参数写明（`e19ad68`） |
| A-N2 | Nit | `LinkTarget.aliases` 没有 `maxItems` | `maxItems: 1000`（`e19ad68`） |
| A-N3 | Nit | `links` 帧没有经契约的结构核对 | `links()` 经 `apitest.CheckSchema(..., "EventLinks", ...)`（`e19ad68`） |
| A-N4 | Nit | 改写算写法（`domain.Linktexts`）对同名的页是平方的：4 万个 2.9 秒 | 与 A-M1 一起写进 M12 的移交第 8 项 |
| A-N5 | Nit | `Rewriting.Kept` 只为测试导出 | 移到 `domain/export_test.go`（`e19ad68`） |
| A-N6 | Nit | `TestTheIndexIsItsRebuild` 的别名标记按种子计，个别种子没覆盖 | 按全部运行计数，结束时核对覆盖（`e19ad68`） |
| A-N7 | Nit | 文档里几处数字与代码不符 | 改正（`7957264`） |
| A-N8 | Nit | `page/link_targets.go` 在模块根做编排 | 接受：模块根实现给别的模块的窄端口，是第 11 条已有的做法 |
| B Nit 1 | Nit | mermaid 标签保留的 HTML 比服务端给用户 HTML 的白名单宽：`class`、`style`、`<img>`、表单与密码框（实测都限在图之内，没有 CSP 违规） | 禁掉表单与控件（`form`、`input`、`button`、`select`、`option`、`textarea`；图里问密码）；`class`、`style`、`img` 不禁（实测限在图里，`img` 只有 `data:`），写进 13.2 第 23 条的信任边界（`dfd5d3b`）。L4 的恶意标签加表单与密码框，断言它们不在；反向对照失败 |
| B Nit 2 | Nit | C8 只等阅读视图与锁各答两次，同一层还重读反链与属性 | 四个都等（`2b907e0`） |
| B Nit 3 | Nit | P3B 审查记录写 W12 的修法是"加 2.5 秒延迟"，代码是按行轮询 | 改正记录（`2b907e0`） |
| B Nit 4 | Nit | 编辑器扩展的 `load` 失败也被缓存：一次分包加载失败，这个页面里补全就一直没有 | 失败的不缓存，下一个编辑器再试（`dfd5d3b`）；测试，反向对照失败 |
| B Nit 5 | Nit | `app-links.ts` 的 `data-nw-node` 没编码就拼进路径 | `encodeURIComponent`（`dfd5d3b`）；测试，反向对照失败 |
| B Nit 6 | Nit | 反链的列表与属性的 `<dl>` 没有名称 | `aria-label`（`dfd5d3b`）；反链的由同名的测试经名称找到 |
| B Nit 7 | Nit | 反链"更多"的防重入没有测试 | 新测试：读取中再按，只读一次、`aria-busy` 留着（`dfd5d3b`）；反向对照失败 |
| B Nit 8 | Nit | 结构：`page-properties.tsx` 约 170 行配对、`PageTreeStore` 转发 6 个 linking 读、`guardLabels` 在 `math.ts`、`reading-view.tsx` 管四件事、`page-panel.test.tsx` 1,092 行 | 右栏的测试按节拆成三个文件，共用的取元素函数在 `test/page-panel.ts`（`dfd5d3b`）；其余写进 [M12 的打磨移交](../../M12-release/handoffs/M5-polish.md)第 14 项 |
| B Nit 9 | Nit | L5 的反向检查等固定的 500 毫秒；L1 用修饰键开的新标签页不经 `anotherTab`，控制台与 CSP 没人看 | L1 的新标签页开始看着之后重新加载，核对安静（`2b907e0`）。L5 的固定等待不改：反向检查只会漏、不会误报，周围的正向检查在同样的负载下；代码里不补全的规则由 vitest 确定地核对 |
| C-N1 | Nit | P4 :202 的 `verify.mjs`、P2 的状态、P7 :117 的停顿、M6 总设计 :615、:98 | 改正（`7957264`） |
| C-N2 | Nit | `docs/README.md` 的提交格式与实际不一致 | 改正（`7957264`） |
| C-N3 | Nit | 8.6 的 `reindex` 选项与重算标题键；9.3 补全的别名；根 README 的样例集；M6 总设计的核对脚本 | 改正（`7957264`、`38ca3b8`） |

## 疑问与判断

| # | 疑问 | 判断 |
|---|---|---|
| A-Q1 | 引用它的页正文合计超过预算时，改名总是 503 `server_busy`（带 `Retry-After`，重试也一样） | v0.1 不改（负责人可以改判）：默认 8 MiB 下要第七页 5 MiB 的才会；P4 文档第 9 节已写。写进 [M12 的性能移交](../../M12-release/handoffs/M4-performance.md)第 3 项，压测时看改名的 503 比例，必要时分批改写 |
| A-Q2 | 读回不成立、写出超过一页上限的页不改，只记日志，改名答 200 | 有意如此（P4 文档第 9 节，负责人可以改判）：拒绝整个改名会让一页坏掉的正文挡住所有改名；锁的预检在这之前，被编辑的页仍挡住改名 |
| A-Q3 | 参与者的 `TakeNow` 用 `semaphore.Weighted.TryAcquire`，只要有人在排队就失败，额度够也一样：阅读视图排着队时改名答 503 | v0.1 不改（负责人可以改判）：写进 M12 的性能移交第 3 项，压测时一并看，必要时让 `TakeNow` 只看额度 |
| A-Q4 | 文档写的 Obsidian 版本 | 1.12.7（C-M1） |
| B-Q1 | B-I1 现在用补丁修，还是交给 M12 | 现在修（便宜、可核对，与 P6B、P7 定为 High 的读者代价是同一类），见 B-I1 |
| C-Q1 | M10 的 lint 要的数据在索引里，只记在 M6 的文档 | 写成 [M10 的移交](../../M10-llm-wiki/handoffs/M6-links.md)：lint 规则与索引的列的对照、过时索引的处理、提升 `Extractor` |
| C-Q2 | 把各 Phase"负责人可以推翻的决定"汇总给负责人；嵌套属性的键序 | 见下一节；嵌套属性的键序不保留（P5 第 10 节），在汇总里 |

## 负责人可以推翻的决定

各条的理由在所指的文档里。负责人在 M6 开工时确认的三项（改写的页被锁时整个改名 409、点未建的链接确认之后新建、输入法清单不阻碍 M6）见 [M6 总设计](../00-M6-design.md)第 11 节，不在这里。

- **总设计**（第 11 节末）：本人的锁同样挡住改写；被改名、移动的那一页自己的正文要改写时同样答 409；改名、移动时"被抢走"的链接也改写，新建、删除只重新解析；编辑时右栏不显示大纲；M6 等负责人执行完整份清单才改为已完成。
- **P3**（[A 部分末](../03-P3-index.md)）：解析在两处跟 Obsidian（路径短按字符数、`.md` 只读作一种），文件夹自己的页算在它的子树里；标签照 Obsidian 标签面板的计法；键的上限 1024 字节，U+0000 记作 U+FFFD；`reindex` 遇到失败的笔记本继续。
- **P4**（[第 9 节末](../04-P4-rewrite.md)）：只在指向变了或新有歧义时改写；wikilink 的写法跟 Obsidian，Markdown 链接的相对与根路径保持；`aliases` 的值不改写，索引过时的出发页跳过；预算取不到立即 503（A-Q1、A-Q3）；读回不成立、超过上限的页不改而记日志（A-Q2）；Markdown 链接文字里有注释时不跟。
- **P5**（[第 10 节末](../05-P5-api.md)）：每个出发页至多 10 条上下文；长行的窗口从链接之前约 80 字节开始；不算这一页自己的链接；没进索引的页按索引回答、不即时解析；`getTag` 的非法输入答空列表；嵌套属性的键序不保留；`count` 至多 1000、一页至多 1000 个标签与别名、一次请求至多读 32 MiB；VACUUM 之前很少几个链接目标时反链仍可能走主键。
- **P6**（[第 15 节末](../06-P6-reading-view.md)）：全部标签的总览与 frontmatter 的 `tags` 链接交给 M12；附件那样的名称在 M7 之前新建为页；落点只给写者；mermaid 的上限（`maxEdges` 200、原文 20,000 字节）与公式 4,000 字节；属性表按值的身份对齐；父页在最深一层时答 `too_deep`；定义宏的公式不排；KaTeX 的补丁与 150 层、布局至多 1 秒；mindmap 至多 150 行，别的图画几秒；换语言重新增强整个视图。
- **P7**（[第 15 节](../07-P7-editor-panel.md)）：补全的数据每个 `[[`、`#` 读一次、10 秒内同一处复用；不做 `[[页面#标题` 的补全；`#` 补全不认紧跟行内元素与块开头的标签；没闭合的 frontmatter 到第一个空行；编辑器不认 frontmatter；补全的表格照 lezer；右栏在 `xl` 以上才在旁边；大纲至多 1,000 项；反链的重读读已读的页数、不设上限；属性链接只去页面、路径长于 1,024 的不配对。
- **收尾新加**：索引每页至多 10,000 条链接，之后的不进反链、改名时不改写（A-I1）；附件嵌入的渲染改由 M7 建立（C-I2）；很多同名的页、改名的预算交给 M12（A-M1、A-N4、A-Q1、A-Q3）；给 `@lezer/markdown` 打补丁，深层嵌套交给 M12（B-I1）；mermaid 标签只禁表单与控件（B Nit 1）。

## 第 13 节

由一位 Opus 逐条对照代码起草、作者核对，写进[总体设计](../../v0.1-design.md)第 13 节（第 15 节记一行修订，C-I4、A-M6、B-M4）：

- **13.1 后端**：第 1 条写 `nervewiki reindex` 不经写入单元（只重算标题键与重建索引，不记变更集，只发一条不列页的 `links`）；第 5 条补 M6 的八个交错（代码里没有编号，按测试名引用）、`interleaveOnIndex`、`checkPages` 含 `checkLinks`、`checkRebuilt`，以及 reindex 不取工作区行的例外；第 6 条新增"派生的索引表"（不带外键、没有 `deleted_at`、不登记清理器，删除的路径删行）；第 8 条加 `locks`（`ProblemLocks`、`shared.Error.Locks`）；第 11 条列 M6 的窄端口与两个例外（`linking.NewIndex(pool, pages, publisher)`、`linking.ResolveLinks(pool, pages)`）；第 19 条新增"持着锁时不排队"（参与者 `TakeNow`、`KeepFacts` 到单元结束、reindex 排队取的例外）；第 21 条写 `markdownExtensions(resolve)` 与 M6 的最后一跳测试；第 30 条加 `links` 类型与载荷；新增第 31 条"写者控制的数量都有上限"（`MaxLinks`、`MaxNames`、`MaxKey`、上下文与计数、`MaxEventPages`、YAML 的值与层数）。
- **13.2 前端**：第 7、15、16 条加 `tag-pages`、`backlinks`、`page-properties` 三个只在 SWR 里的键与连上时的重读；第 19 条写反链的分页（重读读回已读的页数）；第 23 条写注册表到 M6、编辑器扩展的 `load` 与 `editor/loaded/`、失败的下一次再试、`EditorContext` 的两个读、事件只为读过的键重读，以及 KaTeX 与 mermaid 的信任边界；第 24 条按滚动区改写；新增第 25 条"写者的内容在读者标签页里的代价有上限"（公式、图、大纲、属性路径、正则与参数、编辑器的解析）、第 26 条"读者动了就不抢焦点"（`watchReader`）。
- **13.3 Markdown**：第 2 条写三套样例与三个核对脚本；第 3 条改正"一次取够预算"（写入排队取，参与者逐页 `TakeNow`）；第 5 条的组合函数；新增第 6 条"提取规则的版本与链接的标记"（`Extractor` 与 reindex、过时索引的处理、`data-nw-*` 只由渲染器写、链接里没有链接、扩展的钩子）。
- **13.4 测试**：第 3 条 `assert/links.ts`；第 4 条链接索引的交错与 `WaitForAdvisoryLockWaits`、`interleaveBehind`；第 6 条 M6 的六个操作；第 8 条 Makefile 多的两个耗时测试；新增第 9 条"查询计划的测试"（`planOf`、没有统计时的耗时）、第 10 条"派生数据等于重建"。
- **13.5 工程**：新增第 5 条"依赖的补丁"（katex 与 `@lezer/markdown`，理由、升级时复核、改解析行为的要有差分与代价测试）。
- 同时：12.6 的 M6 为"进行中（待负责人执行输入法清单）"；第 15 节补 P4、P6 B 两行（C-M3），收尾一行连同 `7957264` 改写的 6.1、8.6、9.3、12.2、12.4 与第 14 节；P1 文档的 `scroll-focus.ts` 与表格包装的 `tabindex`。

核对时起草者报的五处文档与代码不一致，按代码处理：reindex 不取工作区行，写成第 5 条的例外（它第一条语句就锁笔记本行，此前什么也不持，不成环）；P1 文档写渲染器输出 `tabindex`，实际由前端给，改正；第 15 节没有 `7957264` 的行，并进收尾一行；改名的预检之后被心跳续活的交错结束时只核对 `checkLinks`，改为 `checkPages`；M5 的事件类型测试用 `links` 当替身类型，载荷是假的，与 M6 真的类型同名，服务端（events 模块的三个测试、`events_lab_test.go`）与前端（`events/hub.test.ts`）改名为 `later`。

## handoff

- **收到的**：[M4/P3 Markdown 扩展](../handoffs/M4-P3-markdown-extensions.md)、[M5 事件类型](../handoffs/M5-events.md)逐项对照代码改为 `done`，写明落实；`M6-links/handoffs/` 没有 `open` 的了（C-I1）。
- **M6→M7**：新的[链接与附件、导入、导出](../../M7-assets-transfer/handoffs/M6-links.md)：附件嵌入的接缝（C-I2）、附件进解析、落点拒绝附件、导入是多操作的单元（索引要新、`TakeNow`、导入之后 `ANALYZE`，A-M2）、导出没有正文的被链接页、最后一跳。
- **M6→M8**：新的[链接索引与恢复、历史、还原](../../M8-history-search/handoffs/M6-links.md)：恢复带提取结果、只重新解析，历史版本即时解析，还原与整组撤销碰上改名、移动时的 `linking.pages_locked`，行为测试。
- **M6→M9**：[改写的移交](../../M9-mcp/handoffs/M6-P4-rewrite.md)加第 4 项（反链与索引的工具）。
- **M6→M10**：新的 [lint 的数据在链接索引里](../../M10-llm-wiki/handoffs/M6-links.md)（C-Q1）。
- **M6→M12**：[性能](../../M12-release/handoffs/M4-performance.md)第 3 项（改名的预算，A-Q1、A-Q3）、第 6 项改写（链接很多的页，A-I1）、第 7 项（补全读整个笔记本的目标）、新的第 8 项（很多同名的页，A-M1、A-N4）与第 9 项（编辑器解析深层嵌套，B-I1）；[打磨](../../M12-release/handoffs/M5-polish.md)第 13 项（编辑时的大纲、全部标签的总览、别的引擎的锚点）与第 14 项（前端的几处结构，B Nit 8）。

## 文档与代码的不一致

照代码改写：

- **[M6 总设计](../00-M6-design.md)**（C-M1、C-M2、C-N1）：Obsidian 1.12.7、78 个样例、固定种子的随机测试、`Extract(Tree)`、迁移 00025 与 00026、第 5 节的 `getLinkLanding`、第 8 节的扩展钩子与附件嵌入改由 M7、第 9 节的对等验收与例外、风险表的预算取法、第 11 节的指向。
- **Phase 文档**：P2 的预算说明加上索引的工作集（A-I1）；P4 :202、P7 :117（C-N1）；P6 的 L4 第三个故事与"经组合根的表"（B-M1、B-M5）；P1 的 `scroll-focus.ts`（B-M4）；P3B 审查记录的 W12（B Nit 3）。
- **[M5 总设计](../../M5-collab-editing/00-M5-design.md)** C4 一行与 C4 的注释：改名、移动照常加上 M6 的条件（B-M5）。
- **[输入法清单](../../M4-pages/manual/P6-ime-checklist.md)**：第 16 步、撤销的一步（C-M4）。
- **README**、`tools/md-fixtures/README.md`、`configs/config.yaml` 的预算注释（C-I5、C-M1、A-I1）。

## 反向对照

审查者做的：
- **A**：组合根的注册者与最后一跳 18 项全部失败。
- **B**：vitest 110 项中 104 项失败，存活的写成了发现（B-M1 的四项、B Nit 7 等）；e2e 12 项中 11 项失败，存活的一项（反链的计数不显示）由 vitest 覆盖。
- **C**：七份审查记录里 60 条"已修、已加测试"抽查全部属实。

作者在修复中做的（改动全部还原）：
- **Go 8 项**，7 项失败：渲染按错的位置取解析、嵌入不渲染成链接（C-M5）；不设上限（单元测试与整个程序的测试各一项，A-I1）；`ParseNow` panic 时不还预算（A-M4）；每条链接都重建标量表（A-M3）；改写里新写法的那一份在没写成或 panic 时不还（A-M4）。存活的一项是等价的：改写里旧的那一份去掉 `defer` 释放之后，紧接着的显式释放照样还，`defer` 只为 panic。
- **lezer 的补丁**（随机差分）：10 个改坏的副本，7 个出现差异（下限设高一位、键不含"也能开"或长度、截短开始标记时不降下限或降得不够、takeContent 不降"未失效"的下限、`]` 找不到时把下限设高一位），3 个等价（未失效的下限设到链接本身之后：那个位置已是链接元素；takeContent 不降链接开始标记的下限：只有 `LinkEnd` 调它，切的位置不低于下限；`hasOpenLink` 的下限设高一位：自动链接随即追加在那个位置）。第一版差分没抓住"截短时不降下限"，加了只有强调的字母表之后抓住。三个代价测试换回原版各 15–25 秒，失败。
- **前端 vitest 9 项**全部失败：反链按标题、重试什么也不做（反链与属性）、属性不在聚焦时重读、去掉防重入、去掉列表的名称、失败的加载照旧缓存、页 id 不编码、mermaid 不禁表单。
- **e2e 3 项**（各重新构建）全部失败：mermaid 不禁表单（L4）、完整地址不经路由（L1）、编辑时仍显示大纲（L6）。

## 核实修复

<<FIXCHECK>>

## 没能验证的风险

- 只用 macOS 上的 headless Chromium：Safari、Firefox 的输入法与补全、折叠 callout 里已是地址锚点的链接、读屏激活"更多反向链接"之后焦点的去向，由清单与 M12 人工覆盖。
- 链接很多与同名很多的页的代价是单机实测；上限之内的最坏情形、多本笔记本同时保存、导入之后没有统计的查询，留给 M12 的压测（[性能移交](../../M12-release/handoffs/M4-performance.md)第 3、6–9 项）。
- 编辑器解析的补丁只核对了随机差分与几种形状的代价；lezer 升级时要重新核对补丁是否还需要、是否还对（`pnpm-workspace.yaml` 的注释）。深层嵌套的强调让编辑器建不起来，原版本来如此，交给 M12。
- 审查期间别的会话同时在跑测试容器。

## 人工验收

[输入法清单](../../M4-pages/manual/P6-ime-checklist.md)的第 16–18 步（补全与输入法），由负责人在 Chromium、Safari、Firefox 上用真实的输入法执行；第 1–9 步的结果记在 [M4/P6 审查记录](../../M4-pages/reviews/P6-source-editor-review.md)，第 10–15 步记在 [M5 收尾审查记录](../../M5-collab-editing/reviews/M5-closeout-review.md)。通过之后 M4、M5、M6 一起改为已完成。另请顺带看一眼：读屏（VoiceOver）激活右栏的"更多反向链接"之后焦点落在新加的第一页（P7 文档第 9 节）。**待执行。**

| 步 | 内容 | Chromium | Safari | Firefox |
|---|---|---|---|---|
| 16 | `[[` 之后用拼音输入，组合中不弹出，确认之后列出 | 待执行 | 待执行 | 待执行 |
| 17 | `#` 之后用拼音输入，确认之后列出标签 | 待执行 | 待执行 | 待执行 |
| 18 | 补全开着时，候选框里的回车只确认输入法 | 待执行 | 待执行 | 待执行 |
