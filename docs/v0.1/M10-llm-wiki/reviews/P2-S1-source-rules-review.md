# M10/P2 S1（来源规则）：审查与实施记录

| 项 | 内容 |
|---|---|
| 对象 | 分支 `m10-p2-s1`；实施基线 `4755b4c99f99e7ef4b1ebbc37fd55febe488acea`；初审头 `6facccde054512bdb5d6684c21a2f05a4219ff83`，tree `a2b7861d45020191bd2fecb0d61dabd51501f4a3`；修复后最终代码头：`fd00ee670c24db3ad4d9478419acd8340f1e6d91` |
| 权威与范围 | [P2 设计](../02-P2-source-area.md) §0、S1 服务端/契约/Web fake、§3.12–3.13、§5–8.1；[00 总设计](../00-M10-design.md)；[总体设计](../../v0.1-design.md)；[工作流](../../claude_to_codex/02-workflow.md) §3。S2 去重/hash/有限读取/迁移与 S3 完整网页仍 pending。 |
| 日期、状态 | 2026-10-27；S1已合并；记录CI与两main Test均值待root验收，S2未释放 |
| 初审与补充 | 三位独立审查者在各自冻结 6fac 工作树读需求、diff、源码并执行反例；初审A服务端正确性0 I/M/N/Q，B数据/迁移/历史/契约0 I/M/N/Q，C Web/API/MCP/E2E/证据0 Important / 1 Minor / 0 Nit / 0 Q。三份初审由主持者核验接受；原C另独立补充确认C-M2 Minor 1条，后在b750修复准备诊断确认C-M3 Minor 1条，均由主持者接受。共三个Minor；初审、补充与诊断统计分开。C-M1/M2为第一组，C-M3为第二组，各ID分别验收。 |
| 修复与复核 | 第一组C-M1/M2已提交 `b750631362a468a01a18dfc4faa065b7d88af4db` 并增量核验；第二组C-M3提交 `fd00ee670c24db3ad4d9478419acd8340f1e6d91`。最终全量验证与原C逐项复核状态：原C在fd00正式复核分别关闭C-M1/C-M2/C-M3，未关闭Important=0、Minor=0，无新增已核实I/M/N/Q；主持者核96件/740,998字节、17独立执行及五个0→1→0。报告review-c/final-fix-recheck/C-final-fix-recheck.md，SHA256 82f06e259ce3da1e3a370cc8385d9f74e7c4a402fd861d28d34097334b167aab；完整证据索引见下文；主持者已接受fd00代码、完整本机验证、精确分支CI/成本及原C正式复核，三个Minor分别关闭。A/B初审无发现；6fac后仅两TS测试设施变化，server、Makefile与CI workflow对象未变，原范围结论保持；C独立执行三项修复与相邻回归并审计v4/CI。v4交付1594件/8,132,996字节、350外层执行与24份冻结程序/fixture已核。文档及合并main CI已核；本次记录CI与两main均值待root外部验收，S2未释放，P2/M10继续进行中 |
| 文档与合并 | 本记录与 S1 投影由同一个带标签、exact-old-once 脚本生成，先在固定提交的 archive 副本演练，主持者全文审；文档 exact CI：已核通过，见[合并与后续验收](#合并与后续验收)。合并见 00 号文档第 12 节。 |

## 实施提交与实际差异

| 提交 | 内容 |
|---|---|
| `08dfaf5fd7311c5c825de6776e346cb9c623950d` | 00049、rewrite/Bytes 持久与同步/后台历史全链 |
| `a0dfe443ce7265389bc75ab80c748141ab2def92` | 来源事实与全部生产者、后台实际结构动作与名义批界 |
| `93395bd5b81861492dd5d8ffc2dca53fa05254dc` | 来源正文/管理/session 规则、后台预检、REST/MCP/文字与 fake |
| `3be31d583d1de622a352076a335f655c4ff0a913` | LW4/LW5 API/MCP、SQL rewrite 与有界持久状态断言 |
| `08fa1d162fb6906b339f7c33188e7ad3622ff512` | test-only：真实降级后同一来源的 REST/MCP 写恢复 |
| `6facccde054512bdb5d6684c21a2f05a4219ff83` | test-only：来源矩阵独立垃圾组与后来整本删除的时间分离 |
| `b750631362a468a01a18dfc4faa065b7d88af4db` | 第一组C-M1/C-M2：仅两份Web测试设施，正文写回位置与删除管理位置修复；增量检查及精确gen-check已核，该增量阶段未作整步接受 |
| `fd00ee670c24db3ad4d9478419acd8340f1e6d91` | 第二组C-M3的最终实际修复：第二组C-M3仅修改Web历史测试设施：现活简单placement按node UUID排序，每步在当时node/树上先管理检查后移动，保留本动作Before权限；不按历史数组、不用原树OR最终树。第一遍不提前移除created/restored，placed避免第二遍重做原树检查；删除/正文保留第一组已证阶段位置，blocked残余保留原管理后备，不模拟完整key/depth/aside/conflict scheduler。第一组C-M1/M2提交b750保持历史，三个ID已由原C正式复核分别关闭 |

六个初审提交的增量检查/生成核对由主持者验收，C 又按 git 对象、原始输入与日志核过；没有声称六棵历史树各自跑过全量 CI。`93395bd5` 的早期 Go 检查后只有未跟踪的 Web 测试做 lint 调整，最终 Web 检查对应提交字节；不是从第一条检查起整个目录均未变。

S1 已有严格来源正文不可变、首次实际正文写、系统 rewrite 及相应撤销、管理员来源树操作、session/version 禁写、no-op 窄预检与按项 batch 正文检查。`page.NewModeGuard(pool, authorizer)` 在组合根同时登记写守卫和会话否决者，均在 lock 之前；`Unit.withSourceLines` 准备/复用事实，写守卫消费事实，会话经最小 `SourceAncestors` 端口读当前祖先。普通模式零额外来源读/管理授权，不抹掉既有响应读取。

实际落名为 `StepContent`、`Step.Kinds`、`NodePlace.InArea`、`NodeFacts.Kind`；附件仍用 `Step.Asset`，其 `Step.Content` 为 nil。恢复跳过已活父节点后可能形成多个根，`StepLines.AfterParents` 为每个外父保留独立祖先线，步内 `Changes` 的当前父图优先；两条合法 RestorePart/continuation 模块执行见证与纯模拟 trace 分开。content-only 撤销只补相关父线到 `RevertState.Lines`。新 helper 实际在 `revert_actions.go`、`revert_job_contract.go`、`job_batches.go`、`revert_source_lines.go`，不是设计旧名 `revert_job_actions.go`。

后台在 S1 已按 compact→生产未缩减名义 cuts→本批同 group 合并的结构动作构造事实；执行保留既有锁下状态、Joined、continuation/skips、retimed/frontier 与合法 halving。同步仍按原 Ops；S1 trace 证明结构/事实差异，S2 真实 duplicate 情形仍待完成。`AfterParents` 与 helper 落名是内部事实表达修正，不新增产品决定。

本 batch 新建空来源的无 base 首正文是窄例外，既存空页/标准/区外原 PathTaken 不放宽；逐项 Merged 只计当项在来源内的实际改变，最终合成写保留累计后备。六个来源 immutable 操作、delete 的 admin_only、两 scope MCP、双语与已有 fake 随 S1 同步；初审/补充的C-M1/M2及b750准备诊断的C-M3分列下文，不用各阶段Web绿灯掩盖已核实的fake偏差。

## 审查发现、主持者处置与修复核对

| 编号 | 级别与发现时位置 | 已核实情形 | 处置与状态 |
|---|---|---|---|
| C-M1 | Minor；`web/apps/web/src/test/history-server.ts:515`，同步与 `jobs-server.ts:367` 共用 | 合法管理员 MCP batch：来源移出后普通 edit，撤销先移回再写回，真实同步/start 应 immutable，fake 却放行；区外先 edit 再移入，撤销先移出再写回，真实同步/后台成功，fake 却拒。6fac fake 以撤销前位置判正文。 | 主持者接受合并前修复，fake 按实际写回位置判；不能用“当前位置或目标任一在来源就拒”误拒反向。不在 TS 重建完整 Go scheduler/cuts/halving。第一组已在b750修复并增量核验，最终全量与原C验收状态：原C在fd00正式复核分别关闭C-M1/C-M2/C-M3，未关闭Important=0、Minor=0，无新增已核实I/M/N/Q；主持者核96件/740,998字节、17独立执行及五个0→1→0。报告review-c/final-fix-recheck/C-final-fix-recheck.md，SHA256 82f06e259ce3da1e3a370cc8385d9f74e7c4a402fd861d28d34097334b167aab；完整证据索引见下文 |
| C-M2（补充） | Minor；`history-server.ts:505`，管理helper `page-server.ts:1080` | 同一公开MCP batch新建区外A再把source root移入A；撤销先把root移出再删A，真实两scope×同步/worker四例成功。fake在删除A时仍按请求前树判carriesRoot，同步/start、两种changes数组顺序均误报protected。 | 与C-M1同组修复，删除管理判断使用此前合法逆移动已生效的树；保留root移动权限和直接删除root/仍携root祖先的保护，不跳过所有created保护，不按数组顺序绕过。第一组已在b750修复并增量核验；最终单独回归/负控/原C复核：fd00永久fake定向61项通过，含C-M3的16格及单步Before4格；Folder先时editor拒绝、child先时editor允许，两种历史数组顺序与sync/start均覆盖。fix2首红为Folder先editor四格错误成功，其余57通过；后续lint失败不计业务红。最终v4两组8个单点控制均按同argv 0→1→0，另11个真实/fake对照exit0，C-M1/M2、deferred-parent与cycle边界保留。真实C-M3由HTTP与SQL证明两序，fake后台只核start；deferred预期校正/cycle自审来源不增列C发现。原C最终另独立执行17段：Go9顶层/31含子测试、Web13文件/1303测试，五个同argv 0→1→0负控业务失败数16/10/6/12/10；未重跑作者13门禁，后者为原件审计。第四负控前唯一锚点预检中断与清理、收窄后续跑均保留，不计产品失败或有效负控；原C在fd00正式复核分别关闭C-M1/C-M2/C-M3，未关闭Important=0、Minor=0，无新增已核实I/M/N/Q；主持者核96件/740,998字节、17独立执行及五个0→1→0。报告review-c/final-fix-recheck/C-final-fix-recheck.md，SHA256 82f06e259ce3da1e3a370cc8385d9f74e7c4a402fd861d28d34097334b167aab；完整证据索引见下文 |
| C-M3（修复准备诊断） | Minor；b750 `history-server.ts:546–547` | 合法管理员公开MCP batch移出Folder再改名child，Folder UUID先时，实际撤销先把Folder移回来源再逆改名child；真实editor同步/start均403 source_admin_only且无写/无job，fake却同步changed/后台queued。child UUID先时，真实editor同步/worker均成功，SQL先在区外逆改名后移入；两例最终树相同。 | 第二组修复管理动作发生前的逐步位置，保留本动作自身移动前的来源权限；不能一律看原树、最终树或两者OR。仅测试fake偏差，不是后端权限漏洞，不复制完整scheduler；提交 `fd00ee670c24db3ad4d9478419acd8340f1e6d91`，第二组C-M3仅修改Web历史测试设施：现活简单placement按node UUID排序，每步在当时node/树上先管理检查后移动，保留本动作Before权限；不按历史数组、不用原树OR最终树。第一遍不提前移除created/restored，placed避免第二遍重做原树检查；删除/正文保留第一组已证阶段位置，blocked残余保留原管理后备，不模拟完整key/depth/aside/conflict scheduler。第一组C-M1/M2提交b750保持历史，三个ID已由原C正式复核分别关闭；原C在fd00正式复核分别关闭C-M1/C-M2/C-M3，未关闭Important=0、Minor=0，无新增已核实I/M/N/Q；主持者核96件/740,998字节、17独立执行及五个0→1→0。报告review-c/final-fix-recheck/C-final-fix-recheck.md，SHA256 82f06e259ce3da1e3a370cc8385d9f74e7c4a402fd861d28d34097334b167aab；完整证据索引见下文 |

C-M1的双向真实库对照exit0，fake四个同步/开始后台断言exit1，失败为两次错误放行、两次错误拒绝；真实服务不属于本项缺陷。原始复现为`review_c_history_boundary_test.go`、`review-c-history-boundary.test.ts`与`history-both-directions-{real,fake}.{json,log}`。C-M2的`real-management-v2`四例exit0，`fake-management`四例业务失败；真实worker均等到succeeded并核root回顶层、A回收、Evidence正文/revision及四条原始内容/历史行不变。第一次`real-management`漏message而在schema层失败，不算产品失败或可达性证据，旧记录保留。补充是原代码上的真实/fake对照，没有生产变异，不冒称0→1→0。修复回归与实际选择数：fd00永久fake定向61项通过，含C-M3的16格及单步Before4格；Folder先时editor拒绝、child先时editor允许，两种历史数组顺序与sync/start均覆盖。fix2首红为Folder先editor四格错误成功，其余57通过；后续lint失败不计业务红。最终v4两组8个单点控制均按同argv 0→1→0，另11个真实/fake对照exit0，C-M1/M2、deferred-parent与cycle边界保留。真实C-M3由HTTP与SQL证明两序，fake后台只核start；deferred预期校正/cycle自审来源不增列C发现。原C最终另独立执行17段：Go9顶层/31含子测试、Web13文件/1303测试，五个同argv 0→1→0负控业务失败数16/10/6/12/10；未重跑作者13门禁，后者为原件审计。第四负控前唯一锚点预检中断与清理、收窄后续跑均保留，不计产品失败或有效负控。两个方向或数组顺序不拆新发现；作者deferred-parent预期校正和局部图cycle自审回归不记作6fac新产品缺陷或新增C发现。

C-M3确证轮绑定b750：notebook/account公开MCP各形成真实历史，editor同步/start均拒绝；admin同步/真实worker均成功，独立SQL审计恰为Folder先移动、child随后改名时祖父已raw。real四子例exit0（5.844s），fake四例中editor两红、admin两绿（exit1，0.943s）。必要正对照仅改变真实创建/UUID先后：child先时，editor两scope×同步/worker四子例成功，SQL恰为区外child改名后Folder移入；real exit0（5.817s）、fake四例exit0（0.942s）。fake后台只观察start，不冒称执行worker。这两轮是原C独立诊断与正对照，既不是生产变异，也不是最终正式复核；原始红/绿与各17件清单分开冻结。最终修复必须同时保留两种顺序及C-M1/M2既有边界，不能用全局最终树或current/final OR抹平区别。

主持者实施期间另核两项，均不是正式三审发现：root-I1 是 P1 §9 的真实 downgrade 覆盖缺口，由 08fa 补同一非空页、同父位置 REST revision 2 / notebook MCP revision 3 及 SQL 三版本 false，首次补测即绿；root-I2 是 08fa 全门禁发现的测试 fixture 时间碰撞，由 6fac 修为创建 T−2h、独立 trash T−1h、外层整本删除 T。新 `TestSourceMatrixSeedKeepsItsTrashGroups` 断严格时间序、原 root/计数和非空已删 Wiki 集合，并调用原 `checkPages`。两项均不改生产规则；sourceTrash 保留期起点前移一小时，不声称任意 purge cutoff 结果相同。

初审的无问题项目与限制分别保留：A 核所有 producer、末跳、锁/会话、batch、真实 worker/前批保留；B 核真实 migration、SQL/fake AND、两个 reader、恢复森林、299 个 REST 矩阵格；C 核码表/双语/11 项 API-MCP E2E及原始证据。A/B 没有重跑全部 Web/门禁，C 没有重跑完整 288 次变异/13 门禁，其全量部分是独立哈希/输入/日志审计。三者均未将 S2/S3 待办判作 S1 缺陷，也未提出有实证的过度设计项。

### 两组修复与最终核对

第一组C-M1/M2的b750提交与增量核验是已发生历史，该增量阶段未作整步接受；第二组C-M3提交为`fd00ee670c24db3ad4d9478419acd8340f1e6d91`。最终全Step、CI与原C逐项复核状态见下列记录，各项按实际回执列明；以下最终记录不沿用b750增量结果。

- 最终代码与原审查者核对：`fd00ee670c24db3ad4d9478419acd8340f1e6d91`；原C在fd00正式复核分别关闭C-M1/C-M2/C-M3，未关闭Important=0、Minor=0，无新增已核实I/M/N/Q；主持者核96件/740,998字节、17独立执行及五个0→1→0。报告review-c/final-fix-recheck/C-final-fix-recheck.md，SHA256 82f06e259ce3da1e3a370cc8385d9f74e7c4a402fd861d28d34097334b167aab；完整证据索引见下文
- 新增/保留反例、全部 Step replay、A/B/C 反例复跑和范围：fd00新目录whole-step-final-v4：109 variants /112 checks /336同argv执行，112 baseline与112 restore均0，mutant110次exit1、2次已知exit0。108项至少有一个预期catcher；test-input、test-fixture及cycle结构观察单列，不全部称产品语义kill。Of/bootstrap存活而direct of/of_subset捕获；active-background→sync继续存活；marker barrier、trace与用户结果边界保持。原93控制288份失败测试名集合与6fac轮一致；A/B/C八控制、fix1五控制、fix2三控制均重放。另11真实/fake对照exit0，原C-M3 Folder先拒绝与child先允许由真实HTTP/worker/SQL次序验证，主持者root-replay-integrity及root-counterexample-integrity均接受
- 最终门禁/分支 CI：fd00完整13阶段串行、全exit0，总wall3155.474s；static-build7.951、race300.399、bootstrap1/2/3为288.549/308.360/262.781、costs331.525、guards469.401、web103.541、short54.698、e2e139.401，三片第二次308.944/312.807/267.028s。完整Web169文件/3492测试通过，完整E2E302测试通过；root-gate-acceptance核13条命令/日志与121份提交文件未变，未提高阈值；[CI40492563209](https://github.com/open-nerve/NerveWiki/actions/runs/40492563209)，exact fd00ee670c24db3ad4d9478419acd8340f1e6d91、attempt1，十作业completed/success，主持者与原C各核实时API原件。Test步骤：web219、race625、bootstrap1/2/3为365/441/396、costs379、guards745、short180秒；E2E182、image smoke66秒。仅一个代码分支样本，不能替代两次main Test均值
- 主持者最终接受与未完成项：主持者已接受fd00代码、完整本机验证、精确分支CI/成本及原C正式复核，三个Minor分别关闭。A/B初审无发现；6fac后仅两TS测试设施变化，server、Makefile与CI workflow对象未变，原范围结论保持；C独立执行三项修复与相邻回归并审计v4/CI。v4交付1594件/8,132,996字节、350外层执行与24份冻结程序/fixture已核。文档及合并main CI已核；本次记录CI与两main均值待root外部验收，S2未释放，P2/M10继续进行中

## 主持者处置与决定序列

决定 36–43 已在 [P2 设计审查](P2-design-review.md) 接受，与 00 §11 的总设计编号独立。C-M1/C-M2/C-M3修复已有契约、root-I1/root-I2和事实表示/文件落名修订不改变产品选择；本记录不新编决定44，下一产品决定仍从44接续。设计审查决定42的`K+\|X\|`只修Markdown管道符，不改原语义。

## 变异、真实 catcher 与保留的存活

6fac 的冻结 replay 为 **93 variants / 96 checks / 288 commands**：S1-1 14/17 checks、S1-2 22、S1-3 53、S1-4 SQL reader 2，另 test-input 1、test-fixture 1。91 个生产/观察器变体中 90 个有语义 catcher、1 个 active-background→sync 存活；Of/bootstrap 是另一个 surviving check，Of 的 direct reader 已捕获，不能按 check 数推 variant 数。每一 check 同 argv baseline→单点破坏→逐字节恢复重跑；编译、panic、timeout、skip/无选择不算 kill。最终修复头重新执行情况见上节，不能沿用 6fac 为修复头结果。

| 项 | 实际 catcher 与证据边界 |
|---|---|
| #8 full 与 Of | `page/adapter/postgres.TestSourceHistoryReadersKeepTheRevertedVersionsRewrite/full` 和 `/of`、`/of_subset` 分别读真实 SQL。full 改读 base 由 direct/真实 bootstrap 捕获；Of 在原 bootstrap 确实存活，由 direct of/of_subset 捕获；保留两条观察。 |
| #8 compact/ops/writeback | `page/app.TestSourceRevertsCarryRewriteAcrossBatches` 与 `bootstrap.TestTheSourceRewriteSurvivesUndoAndUndoAgain` 分对应轮；background writer marker 的 bootstrap 只到第二批 barrier 等待失败，不算任务终态 guard 证明，独立 app catcher 断实际存储 Rewrite=false 与期望 true 不符。 |
| #38 active branch / trace | active-background→sync 在三个真实 S1 场景同判而存活；`TestSourceJobFactsUseNominalCutsAndActualActions` 的事实次序变异由 trace 捕获。不能声称 S1 用户结果已区分这两条，S2 duplicate 场景仍待兑现。 |
| #39 cross-group / Merged | 只看首恢复成员的旧 oracle 曾存活，后补 literal 全组成员捕获；旧 create Merged catcher 曾存活，区外非空 create→move 入来源的 producer catcher 断实际 Merged=1/期望0。后者不冒称用户拒绝结果已变化。 |
| 补充 test-input | downgrade 请求正文改回原 bytes，合法 no-op 仍 200；`TestTheDowngradeEndsTheContract` 在 readback 正文/revision=1≠2 失败。它证明观察器要求真实变化，不是产品功能红→绿，不声称同轮逐条击中后续所有 SQL。 |
| 补充 test-fixture | `trashedAt=now+1h` 改成 `now+2h` 与外层 T 相撞，`TestSourceMatrixSeedKeepsItsTrashGroups` 失败；测试数据合法性反例单列，不并入生产语义 killed 数。 |
| A 独立反例 | `TestReviewAEmptySessionCannotWriteAfterAncestorMove`：移入来源前开的有效会话对空来源 changed save，单去 session 拒绝条件让真实 PUT 错误写入 revision2；0→1→0，非 no-op/预检证据，也不是删除 guard 注册轮。 |
| B 独立六轮 | SQL AND→OR、full 读 base、Of 读 base、fake AND→OR、compact 丢 rewrite、忽略 AfterParents，各同 argv 0→1→0；真实 SQL、模块 fake、森林各自标层。 |
| C 独立注册轮与对照 | `TestReviewCSourceIdentityAcrossPublicEntries` 的HTTP/notebook MCP/account MCP×writer/admin六格，单去write mode guard、保留veto，严格v2三段0→1→0，真实错误正文/revision；C-M1双向、C-M2删除管理位置及C-M3两种管理次序对照另列，均非该生产mutation。 |

完整实际选择名在 [P2 §7](../02-P2-source-area.md)；完整展开 argv、exit、输入/恢复 SHA、失败断言归冻结 `variants.json`/`results.json` 与各 reviewer 原件。旧记录均保留：3be 第一轮 91/279 加 marker supplement 3 次，发现 downgrade 缺口后没跑该头全门禁；08fa 第二轮 92/285 与两 focused race 后，在第五阶段 bootstrap-3 exit 2 / 273.494s，两条既有可见性测试报独立垃圾组不变量，前四阶段成功、后八阶段未执行。6fac 是新目录完整重跑，不覆盖前轮。S1-4 rewrite-false 早轮缺同 argv 前置 baseline，之后严格补轮；Web lint、fixture/编译等无效失败不算语义红。

B 矩阵首次漏 prepare（exit1、0格）及随后只选 prepare（exit0、0格）均不算矩阵验证；有效独立选择为 23 行×13角色=299 格，和作者原 source 增量21×13=273格口径不同。C 最初独立恢复因并行加入临时 helper 编译失败，不算恢复绿或产品问题；保留旧轮，仅严格 v2 输入稳定三段算有效证据。

## 门禁、CI 与分片样本

设计提交 4755 的精确 main [CI 40413727596](https://github.com/open-nerve/NerveWiki/actions/runs/40413727596) 十项成功，S1 由此进入。以下表格只记录 **6fac 初审候选**，不宣称后续修复头已完成这些项目。

| 项 | 6fac 已核记录 |
|---|---|
| focused race 两轮 | wall 37.803/37.871s；每轮74顶层/286含子测试，8实际测试包/9请求包；page/http 未匹配测试。新seed .83/.84s、真实downgrade .83/.82s、真实rewrite1.47/1.47s。 |
| 串行13门禁 | 全 exit0，wall3146.177s。static-build 9.307、race315.435、bootstrap1/2/3 305.453/308.156/263.434、costs317.001、guards452.314、web102.616、short55.602、e2e140.103；三片第二次294.761/308.340/273.604s。完整Web169files/3436tests，E2E302。 |
| bootstrap本机Go包 | 1：304.675/293.997，均299.336；2：307.397/307.670，均307.5335；3：262.664/272.802，均267.733秒。与make wall、CI Test不混算。 |
| 精确代码分支CI | [40468800650](https://github.com/open-nerve/NerveWiki/actions/runs/40468800650)，6fac attempt1，十作业success。bootstrap Test441/309/371秒；race464、costs482、guards760、web211、short147、E2E230、image64。仅分支单样本，第一份距450秒9秒。 |
| 既有成本界 | CI20metrics均在原界内；IndexItems三中位17.511520/17.976553/17.526051ms是每样本20次调用总量，对30ms界；GrepCandidates184.718762/185.740306/191.332950ms，对600ms界；完整app grep427.096100ms。不改阈值、不由本样本推长期稳定或原因；原3个失败仍保留，累计24个workflow、23个modern；IndexItems与GrepCandidates各自69组、207个样本，非两类合计。 |

最终修复头各项记录如下，未完成项明确标为pending：

| 项 | 最终记录 |
|---|---|
| 精确代码/原始证据 | `fd00ee670c24db3ad4d9478419acd8340f1e6d91`；whole-step-final-v4/S1-Step-report.md SHA256 8f3f3987790fd8f2217270b31512a4580920a9eaadfff02bd81730062b2288d7；delivery-manifest a5dac66bfd534f8dcd0c272f924e7ee0b9e941f87ebf7beae15a2005bb33e1ae，root-delivery-acceptance已核1594件/8,132,996字节。最终派生摘要final-validation.json SHA256 6684c25c20921786ad4807a9a52367c8c98190e70a9f971e89ce07ad0533f08c，原执行log/meta/diff/backup未变，整理说明保留在冻结报告。schema49候选已另存schema49-baseline/fd00ee670c24db3ad4d9478419acd8340f1e6d91/；binary SHA256 f5f08a63a409229dc17d8a9470b621a629d6425738a3868095887c8d8568e771、39,348,378bytes，version绑定fd00/modified=false、max49。精确代码CI与原C正式复核原件已另存；原C报告SHA256 82f06e259ce3da1e3a370cc8385d9f74e7c4a402fd861d28d34097334b167aab，清单23813b0f66f101cc2dcd38de6e192c329d9034385fb6106524707f841edff2f9，root复核回执p2-s1-original-c-final-recheck-acceptance.json；代码CI回执p2-s1-final-code-ci/root-ci-acceptance-and-fix-review-release.json，成本报告p2-s1-final-code-ci-cost-supplement.json及root接受回执均保留。本次记录CI与两main均值待验收；保存构建仍按schema49候选标识 |
| focused race | fd00两轮同argv -race -count=1 -json -timeout 15m -p 1，均exit0、输入稳定、无fail/skip；wall36.692/37.655s，各74顶层/286含子测试，8实际包/9请求包，page/adapter/http无匹配且不计focused覆盖。rewrite1.40/1.53s、downgrade0.86/0.86s、seed0.79/1.87s；主持者root-focused-races已核 |
| 完整门禁 | fd00完整13阶段串行、全exit0，总wall3155.474s；static-build7.951、race300.399、bootstrap1/2/3为288.549/308.360/262.781、costs331.525、guards469.401、web103.541、short54.698、e2e139.401，三片第二次308.944/312.807/267.028s。完整Web169文件/3492测试通过，完整E2E302测试通过；root-gate-acceptance核13条命令/日志与121份提交文件未变，未提高阈值 |
| bootstrap两轮/均值 | fd00本机三片各两轮：make wall 1=288.549/308.944（均298.7465）、2=308.360/312.807（均310.5835）、3=262.781/267.028（均264.9045）秒；Go包1=287.873/308.122（均297.9975）、2=307.708/311.993（均309.8505）、3=261.978/266.243（均264.1105）秒。两种计时分开；最终两main Test均值待记录CI后核验，不用本机值替代 |
| 代码分支 exact CI/成本 | [CI40492563209](https://github.com/open-nerve/NerveWiki/actions/runs/40492563209)，exact fd00ee670c24db3ad4d9478419acd8340f1e6d91、attempt1，十作业completed/success，主持者与原C各核实时API原件。Test步骤：web219、race625、bootstrap1/2/3为365/441/396、costs379、guards745、short180秒；E2E182、image smoke66秒。仅一个代码分支样本，不能替代两次main Test均值；本次fd00代码CI20项均在原界内：IndexItems每样本为20读总量，三阶段中位12.092768/12.053564/13.592351ms，对30ms；GrepCandidates三中位197.394567/184.531791/196.978013ms，对600ms；完整grep调用中位402.851941ms，对1000ms。新累计25个observed workflows/24个modern，IndexItems与Candidates各自72组/216原始样本，app call24组/72样本；三项历史失败保留，不用新界追改旧分类。6fac各自69/207继续作为旧历史，文档/main后续增加按届时实际回执更新；不从单轮绿色推稳定性。fd00本机21原始样本另列：IndexItems三中位10.201291/9.178542/10.357667ms、Candidates99.131791/93.979667/105.028250ms、call214.983917ms，均在原界；不混作CI样本。最终代码成本补充SHA256 abc63563b93aa5410aead1e037d479e28286a207e275b146c3ee9f3301970241 |
| 文档分支 exact CI | 已核通过，见[合并与后续验收](#合并与后续验收) |
| main 合并 exact CI | 已核通过，见[合并与后续验收](#合并与后续验收)；合并见 00 号文档第 12 节 |
| main 记录 exact CI | pending：本记录提交后由root树外核验，见[合并与后续验收](#合并与后续验收) |
| 两个 main Test 样本及均值 | pending：记录CI完成后核最终两main样本对，见[合并与后续验收](#合并与后续验收)；每份须 <450 秒，不能替用本机或分支样本 |

### 冻结原件索引

主持者保管树外`m10/p2-s1-evidence/`；这些逻辑路径与哈希用于追溯，不是仓库相对链接。初审A `review-a/REPORT.md` SHA256 `3d7903a1b09d275052d12fd3e516b5e71b7585e707459a6f0c1bf05afdf08b04`，B `review-b/report.md` `07950247c3c450cb5c2b4bfee6dfdb6d9d430e5c817f160a89ebc00dbb80b7f9`，C `review-c/C-review-report.md` `cc07f3b73f64676ea1c63b6ced80836eb730a5fad9e483a789de665b305b8976`。原C的53件manifest冻结不变；独立补充`review-c/supplement-management/C-management-supplement.md` SHA256 `63cd36aae3361b067149c206488ec604dcd95a6ce37f3ea7ea7ddb193701565e`，另有23件manifest，不覆盖原报告。v3 release SHA256 `9bab8c6bf10367f00cc7916f8bd12c2c9c88bed1f4dd0b9676eca5d092f6f707`。

b750原C准备诊断`review-c/fix1-management-order-diagnostic/management-order-diagnostic.md` SHA256 `e0fcaa43b8eb7ab0ec6e75cc072f9bd4223fa3db8c24e443739a907751b68699`，独立17件manifest `88307da85d85a05290dceffff27706ccabb48399ce0b8f1c6e167ef189923be7`；必要正对照`review-c/fix1-management-order-earlier-child/earlier-child-positive-control.md` SHA256 `3bb5b7dfc39d49f7dfd5a24fb02e120a466fce5cec6a922604b50c111492dcb6`，另17件manifest `7037beed587351357546d0b42bec3231fa4366a5816479298d6fa84cd0c56f7a`。原53件、C-M2补充23件继续冻结；这两份诊断报告不替代最终原C复核。

6fac `whole-step-final-v2/S1-Step-report.md` SHA256 `9df6e86d3c91077ba64e8b74e1f1c1587a286568817c6b3584754b1c458ed06f`，delivery-manifest `183bdab24b0d5d6bf752cca1ef362505cc7a3946c3c69d5c757e09859d0bb5d5`，主持者与 C 独立核1322件/6,836,203字节及121个变更文件与6fac对象一致。原报告冻结时的“CI进行中/审查待放行”保留为当时记录，本页追加已核CI与初审后续，不改原报告。

08fa 失败 `whole-step-final/gates/test-go-bootstrap-3.log` SHA256 `31afe04c81e6eadf7cf9b7458dc50db6350367426cd18814e4835a743a382613`；`whole-step-final/complete-gates.json` `c81b6ce65e554e7ab00d864b172ed8e72017029c27cfad4f6c7538110d5df5ae`。6fac 的补充成本回执 `p2-s1-code-ci-cost-supplement.json` `6d3da21c998b0611cdb3bb3839b49e4b362ea4d22b226506c7f524e97d3240f8`。修复后的证据及复核另增索引：whole-step-final-v4/S1-Step-report.md SHA256 8f3f3987790fd8f2217270b31512a4580920a9eaadfff02bd81730062b2288d7；delivery-manifest a5dac66bfd534f8dcd0c272f924e7ee0b9e941f87ebf7beae15a2005bb33e1ae，root-delivery-acceptance已核1594件/8,132,996字节。最终派生摘要final-validation.json SHA256 6684c25c20921786ad4807a9a52367c8c98190e70a9f971e89ce07ad0533f08c，原执行log/meta/diff/backup未变，整理说明保留在冻结报告。schema49候选已另存schema49-baseline/fd00ee670c24db3ad4d9478419acd8340f1e6d91/；binary SHA256 f5f08a63a409229dc17d8a9470b621a629d6425738a3868095887c8d8568e771、39,348,378bytes，version绑定fd00/modified=false、max49。精确代码CI与原C正式复核原件已另存；原C报告SHA256 82f06e259ce3da1e3a370cc8385d9f74e7c4a402fd861d28d34097334b167aab，清单23813b0f66f101cc2dcd38de6e192c329d9034385fb6106524707f841edff2f9，root复核回执p2-s1-original-c-final-recheck-acceptance.json；代码CI回执p2-s1-final-code-ci/root-ci-acceptance-and-fix-review-release.json，成本报告p2-s1-final-code-ci-cost-supplement.json及root接受回执均保留。本次记录CI与两main均值待验收；保存构建仍按schema49候选标识。

## 合并与后续验收

2026-10-27，文档提交`78f0f906cf95a95fd8ab9dcdad92da2fc3a80888`经精确分支CI通过后，以`--no-ff`合并为`bf8d6625daba554ee467aef2003346459657a060`；父提交为`4755b4c99f99e7ef4b1ebbc37fd55febe488acea`、`78f0f906cf95a95fd8ab9dcdad92da2fc3a80888`。合并回执：`p2-s1-integration/merge-acceptance.json`。

| 已发生阶段 | 精确HEAD与CI | bootstrap 1/2/3 Test及E2E步骤 |
|---|---|---|
| 文档分支 | `78f0f906cf95a95fd8ab9dcdad92da2fc3a80888`；[CI40499346141](https://github.com/open-nerve/NerveWiki/actions/runs/40499346141)十作业成功 | 442/299/404s；E2E 228s |
| 首次main合并 | `bf8d6625daba554ee467aef2003346459657a060`；[CI40501936360](https://github.com/open-nerve/NerveWiki/actions/runs/40501936360)十作业成功 | 370/322/389s；E2E 224s |

文档CI回执：`p2-s1-docs-ci/root-exact-ci-acceptance.json`；首次main CI回执：`p2-s1-main-ci/root-exact-ci-acceptance.json`。成本报告分别为`p2-s1-docs-ci-cost-supplement.json`（SHA256 `e0f7d1c57e0bd45272695cd22c868987ce8ed00f3ea9d6738d9c1993edef4222`；root接受`p2-s1-docs-ci-cost-acceptance.json`）、`p2-s1-main-ci-cost-supplement.json`（SHA256 `52f1075840aa5a87432bbc5397fa1953585aecab4d5de25145c754d192047dea`；root接受`p2-s1-main-ci-cost-acceptance.json`）；截至首次main的实际摘要：文档CI累计26 observed/25 modern，IndexItems与GrepCandidates各自75组/225原始样本，app 25组/75样本；首次main后累计27 observed/26 modern，IndexItems与GrepCandidates各自78组/234原始样本，app 26组/78样本。两轮各20项均在原界内；首次main三阶段中位IndexItems 11.466056/11.755995/11.270382ms（每样本20读总量，界30ms）、GrepCandidates 189.153470/185.126985/189.900707ms（界600ms）、完整grep call 397.707290ms（界1000ms）。174旧文件未变、3个历史失败保留，已查询清单无缺口；记录CI后累计仍待实际回执。保留历史失败和各阶段样本，不由绿色推出长期稳定。

本合并记录提交自身的精确main CI、成本回执与第二次main Test样本尚未发生，保持pending；不把分支样本或本机读数充作第二个main样本。按既有[决定33](P1-S3-prechecks-refusals-numbering-review.md)，root在树外保存两次main的run、精确HEAD、作业与Test/E2E时间，核三份Test均值各<450秒及最新整套CI后才放行S2。最终一对在下一次已安排的P2后续文档转录，P2完成前落实；不在触发本次CI的提交预写自身结果，也不为转录额外制造CI轮次。S2/S3/P2done仍pending，M8仍open。

## 接受与后续范围

主持者已接受fd00代码、完整本机验证、精确分支CI/成本及原C正式复核，三个Minor分别关闭。A/B初审无发现；6fac后仅两TS测试设施变化，server、Makefile与CI workflow对象未变，原范围结论保持；C独立执行三项修复与相邻回归并审计v4/CI。v4交付1594件/8,132,996字节、350外层执行与24份冻结程序/fixture已核。文档及合并main CI已核；本次记录CI与两main均值待root外部验收，S2未释放，P2/M10继续进行中

S1 不去重，没有 source_hash/duplicate_of/来源10k读取/600万迁移增量；S2继续。已有两语文字与 fake 不等于完整来源 UI：编辑入口、会话失去和结束、添加来源、持续队列与富重复提示归 S3。LW4/LW5 本步是 API 与两 scope MCP 版本；MCP 没有 task/session/version_restore/undo/reorder 工具，相应观察由 API/Go 完成，rewrite 的 undo 明确走 API bridge。

[M8](../handoffs/M8-history.md)仍 open，S2 去重与 S3 网页解释验收后才 done；[M9](../handoffs/M9-P1-mcp-registry.md)仍 open，工具/虚拟/prompt 留 P3–P5；M5 done、M6与M7 open。P2/M10继续进行中；S1 的 main 与记录 exact CI 及两样本均值核对后才由主持者释放 S2，不开始 M11。IndexItems/grep 的稳定性仍归 M10 收尾。
