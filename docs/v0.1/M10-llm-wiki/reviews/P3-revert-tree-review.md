# M10/P3 撤销规划完整树读取：修复审查记录

> 本记录写入时：产品78167b4的本地门禁、精确代码分支CI和三席独立审查已由主持者接受，已核实Important/Minor/Nit/Q均0。正式文档提交及其CI、修复合并、联合record/指定两main配对和P3done仍待后续实际验收；原13ed main CI失败保持。

## 对象与原失败

| 项 | 对象/状态 |
|---|---|
| S2原输入 | 产品`fa681cf697703bc951266aa85978b1f1d62bbffe`与文档`987d4433c7281859e0f202346ca5346cc1600e21`保留原身份；文档精确分支CI41245802670 attempt1已接受 [E29] |
| 原失败 main | `13edc125d5478285fbdf2750dd467f54bcc61a77` |
| 原 CI | [41247708831 attempt 1](https://github.com/open-nerve/NerveWiki/actions/runs/41247708831/attempts/1)，guards job `124674497913`：FAILED，原件保留 |
| 修复归属 | M10/P3 旁支 `m10-revert-tree`；不计作新的 S2 功能或 P4 开始 |
| 实际产品提交 | `78167b424e5911fbd072ab0a047d82a39526a10d`；tree `bea20fde2becdd3911c3017d61c95602d19ae88d`，parent为上述13ed；26路径（22修改、4新增）的提交blob逐件匹配原接受候选 |
| 集成 | 本记录写入时，正式文档提交及其CI、no-ff合并/main CI、联合record/CI与指定配对仍PENDING；合并见[00号文档第12节](../00-M10-design.md#12-phase-进度表) |

原 `TestSourceRevertAtTenThousandTakesItsTime/no_statistics` 三轮依次为 3.136115390s/200、3.495831081s/200、4.002198968s/500。应用报告 `API request deadline exceeded`，错误链为 `transaction abandoned after restore the blobs of nodes ... context deadline exceeded ... conn closed`。最后 blob 标签是取消被观察到的位置，不单独证明根因。原4s请求、2s提交、7.6s操作和10m target 合同保持；原失败不因本地绿被改写。[E1]

## 主持者的决定（负责人可以改判）70

采用独立 `app.RevertTrees.ListRevertTree(ctx context.Context, notebookID uuid.UUID) ([]domain.RevertTreeNode, error)`，以六字段 `ID / ParentID / Kind / Name / NameKey / SortOrder` 读取撤销规划所需的**完整活树**；独立端口、SQL/adapter、同步及后台接线已合成上述单个原子产品提交；本地门禁、代码CI与三席审查已接受，集成仍待验收。公共 `ListNodes`/完整 `Node` 合同不变，保留 assets、stored NameKey，保留原 SortOrder 值、nil未读/非nil读空、两次完整读取及锁下重规划、上下文事务与取消/错误链。SQL 没有 `ORDER BY`，不承诺查询返回行序。没有 schema/index、RestoreNodes/TrashHistory SQL、夹具、期限或预算调整。[E2][E3][E15]

理由是减少规划未消费的审计字段传输/解码及宽结构承载，而完整历史语义保持。真实计划显示历史更新输入随同 clone 的三轮增长为 20001/40001/60001，不能少恢复历史来换时间。原 diagnostic02 **两次 ListNodes 合计**为187.489209–215.776376ms；不是每次，该口径以主持者勘误为准。本地顺序对照及后续一次代码CI不保证长期稳定，也不证明全部收益由某一条SQL导致。[E4][E5]

## 实施与已有验证

新增 `page/domain/revert_tree.go`、`page/adapter/postgres/revert_tree.go` 及 adapter/app 各一份 `revert_tree_test.go`；其余改动接通端口、模块、同步/后台消费者和对应测试，完整26路径及实际 diff 见 E2。保留规划/事实模拟算法、name slots 和来源规则。两个生产依赖显式接同一 Store；查询仍是指定 notebook 全部 `deleted_at IS NULL` 行，不引入候选裁剪或缓存。原始 Name/NameKey 不重算。

主持者已审实际 TDD 与语义日志：adapter 红阶段4个语义失败，绿阶段含原 `TestNodesReadBack`；app 首次红的非法 fixture panic **不算有效 RED**，保留原件，以修正 fixture 后的 `app-red-02` 证明专用端口/空树/错误/锁下重读；app/domain 绿及后续语义组89顶层、345 PASS节点已核。此阶段的输入敏感性控制不冒充真实源码变异；后续已完成的7项minimum和26项实际源码变异见下节，各阶段身份分开记。[E2][E7][E8]

原未插桩完整 HTTP target 的一次本地对照为三状态、各三轮，九次全200、四个具名测试节点PASS，子进程实际 exit0 / 82.033831373s。来源与原HTTP测试、三样本历史增长、100k基树、9900个真实附件均保持；3346源文件和252 embed 的字节及native模式前后相同。两次对照都未开 profiler/SQL observer。其执行身份仍是原提交前候选，不冒称在78167b4上重跑；当前提交blob与该候选字节一致另核。[E5]

| 状态；sample0 / 1 / 2，ms（舍入到0.001） | 原13ed基线 | 六字段候选 |
|---|---|---|
| no_statistics | 1588.390 / 1741.216 / 2096.858 | 1328.443 / 1515.724 / 1786.118 |
| analyzed | 1571.139 / 1744.710 / 2055.252 | 1317.822 / 1447.912 / 1759.530 |
| force_generic_setting | 1615.579 / 1746.434 / 2052.540 | 1312.720 / 1436.532 / 1838.293 |

对应减少214.247–310.740ms。它是**顺序、非随机的本地 before/after**，不是 CI 预测或单因素因果证明；样本级整数纳秒、状态与原日志以 E5 为准。原失败和后续真实 CI 义务不变。

诊断插桩与 auto_explain 只用于定位，不算本轮门禁或原生性能对照。精确计划与参数不在此重抄。时间口径勘误：helper 第171行先赋 StartedNS、第172行才 typed copy；wrapper差值包含 typed copy、配置/恢复和窗口杂项，排除 StartedNS 前 phase匹配/SQL哈希等，仍不隔离目标内部 auto_explain 成本，也不是总开销校准。[E6]

## 已接受门禁与待验项目

**提交前7项minimum**：session73251实际exit0，原候选3346源/252embed及native模式前后一致；server build、server vet、tools build、tools vet、整个Go模块lint、server short、tools tests七条命令各exit0。server short为7292 PASS/1468 SKIP，包93 PASS/23 SKIP；其中新增4个真库测试按原short规则跳过，真实PG绿仍引用TDD，不称short跑过真库。tools为27 PASS。[E7]

**作者真实源码变异**：26units、78次Go调用，各unit baseline→mutant→restore为0→1→0，原3346源/252embed精确恢复。RT01–RT10/RT21覆盖adapter形状，RT11–RT13覆盖事务/取消/读空，RT14–RT16f覆盖tree读取/投影事实与见证，RT17锁下重读，RT18后台批界，RT19/RT20真实HTTP接线；具体故障与非空断言以E8逐unit原件为准，不把全部选定测试都算kill。RT18的group-name-taken JG仍PASS，实际失败来自property及7个批间HTTP情形。RT07首个observer exit11发生在target之前，没有首次Go运行；RT19/20各自只读checker exit11及同一测试拆行的原输出均保留，随后按每项10个同测试HTTP断言对独立核定，未重跑target。[E8]

**生成模式准备与提交后静态构建**：原生成路径76件非可执行文件仅将native `100664→100644`，源码字节和Git模式/状态不变，备份与逐件身份见E9；minimum/HTTP/变异继续保留原native模式归属，不改写历史清单。78167b4上原命令 `make -j1 lint gen-check knip build` 已实际执行，session75315已收exit0，38.46139235096052s，**postcommit gen-check包含其中，不另声称一次独立gen执行**。该次fresh build已核，251件web产物/252件embed，源码3346件及模式前后一致；Vite大chunk警告保留。该回执的接受范围是fullstatic；后来的native、Web、Cost02与EW分别按实际输入另列。[E9][E10]

最终fresh输入03 SHA `3384363097e6a1cb318186e1db6bdb95a3450fb885d54877db963381dab3135a` 的 `actual_head=78167b424e5911fbd072ab0a047d82a39526a10d`、`actual_tree=bea20fde2becdd3911c3017d61c95602d19ae88d`、`actual_status_porcelain=""`，记录该时点干净提交。其legacy `head=13edc125d5478285fbdf2750dd467f54bcc61a77` 和旧 `status_porcelain` 仅保留提交前候选来源、供冻结validator识别，**不是当前执行HEAD/状态**。最终native模式为3267件100664、76件100644、3件100775；03的embed来源为上述实际fresh build，字节恰与先前相同，不等于没有重建。[E11]

**完整native19与affected双轮**：主持者已按E12接受原8项inventory、五份bootstrap各两轮（10阶段），再加新执行的一个完整nonbootstrap恢复阶段。原run最后的nonbootstrap在客户端重启后发现进程和工具句柄均不在，session15569查询为UNKNOWN、实际退出码仍未知；1858 RUN/1857 PASS且一个测试无终结的部分日志没有completion，不能称原run完整成功，也不是已核实的产品失败。原18阶段与新恢复阶段分别保留真实release、结果、raw及审计引用，旧partial被排除，未把它与新partial拼接。[E12]

新恢复controller实际exit0；完整nonbootstrap为8053 RUN、8052 PASS、1个既有SKIP（`internal/platform/markdown:TestTheCostsAreAboutTheSize`），115个包终结，子进程计时1252.488s（原值1252.4882024307735）。恢复只改证据目的地与session trace输出路径，测试选择、夹具、期限、原heavy/timer/观测工具及源码保持。3346件源码、252件embed及native模式在该阶段前后核对一致。原affected首轮与本次完整nonbootstrap内的page adapter/postgres、app、domain三个包配成两轮，各1917 PASS、零SKIP、745顶层测试，实际节点/结局逐项一致；没有另跑第三轮affected。[E12]

五份bootstrap以下均为原18阶段内的**本地子进程计时**，秒数只为显示舍入到0.001；均值以未舍入值计算，均严格小于450s。它们不替代未来指定两次main CI中各份Test步骤的配对。

| bootstrap份 | 第一轮s | 第二轮s | 均值s | 每轮实际节点与结局 |
|---|---:|---:|---:|---|
| 1 | 295.671 | 299.415 | 297.543 | 690 PASS |
| 2 | 181.022 | 180.888 | 180.955 | 254 PASS |
| 3 | 238.561 | 239.528 | 239.044 | 797 PASS |
| 4 | 275.495 | 271.889 | 273.692 | 1754 PASS、4 SKIP |
| 5 | 236.936 | 233.909 | 235.423 | 413 PASS |

第2份的`TestSourceRESTKeepsRootAndStandardWrites`两轮各有3个实际UUID子名，原件分别保存且各自PASS，不宣称两轮raw子名逐字相同。affected三个包的节点逐项相同是另一个已核事实。[E12]

新nonbootstrap的session审计关联92条binary trace、21个Testcontainers session与42个容器；保存的本人UUID观测中容器最终为空、数值PID不在，最晚UTC见证位于timer退出后11.331488s，在原60s观测界限内。原单查询15s和轮询2s按源码/字段核对，未逐查询实测；没有startticks或exec后exe身份，也未留存临时binary供重新哈希，binary SHA来自执行前adapter记录。状态变化采样与UTC见证不等同逐轮单调时钟测量，只覆盖已记录的本人UUID，不推论整台主机或其它门禁。整份raw由审计器解析；此处不宣称人工逐行阅读。[E12]

**完整Web测试**：78167b4上的原`make -j1 test-web`已实际exit0，session84681已收退出；182个文件、3925个测试PASS，子进程107.502s，源码/embed/native模式前后不变。22条`TimeoutNaNWarning`原样保留。接受范围是完整普通Vitest文字报告及计数，不宣称逐case JSON身份或EW/CI已通过。[E13]

**Cost01已发生的停止**：controller实际exit1、整个make target实际exit2，子进程104.744s。前三条`test-go-costs-lint-01/02/03`各exit0，分别6/22/1个测试节点PASS；第04条在产品测试启动前触发原3GiB（3221225472 bytes）预检，未生成cost-completion，不能把前三条拼成完整60recipes。失败检查没有保存当时实测free值；diagnosis中的当前free是较晚观测，不能反推失败时值，也不能据此宣称产品测试失败或归因于Docker清理。原失败及三条已结束原件保留，随后独立Cost02完整执行及容量/相关guards已另行接受，不拼接Cost01前三条。[E14]

**Cost02完整成本及相关guards**：独立完整60recipes（lint第一/二轮14+14、costs17、guards15），各实际exit0；四组分别1047/1047/89/57 PASS节点，controller实际exit0。14条lint逐项两轮均满足原“子进程均值×2.4小于Go原timeout一半”规则；相关guards05/06/07/13/14另完成第二轮，各2/2/3/4/7个非空节点、两轮全PASS，同一容量规则均通过。源码3346件、252embed与当前native模式核对一致，固定容量校准未放宽。Wiki summary两轮TotalAlloc中位374650768/375132088 bytes，仍超过原250MiB目标；M12义务不因此关闭。两次cleanup子进程exit1属于通过的父测试的预期行为，已排除成本样本。[E19][E20]

其中source-revert原完整guard两轮共18个实际HTTP200：第一轮1.547–2.229s，第二轮1.581–2.219s（显示舍入）。仍按三状态各三样本、100k来源/10k节点组/9900真实文件和原4s请求/2s提交/7.6s操作合同执行；这与早期提交前九HTTP before/after分属不同执行，不能混为一组随机对照。[E21]

**完整EW一轮**：原完整fresh `make e2e`及其前置build实际exit0，353例通过、0 skip/unexpected/flaky，实际case均无retry；原monotonic耗时165.05819911975414s。251件Web产物/252embed及3346源码、native模式已绑定当前78167。LW8已包含在完整用例内；本repair没有额外执行S2专属LW8第二轮。[E22] 本地全门禁接受入口为E18；minimum/作者变异仍是提交前同3346源码bytes记录，76生成文件仅native 100664→100644的事实不改写为完全相同metadata。

## 精确代码CI

[41271794807 attempt1](https://github.com/open-nerve/NerveWiki/actions/runs/41271794807/attempts/1) 的实际输入为分支 `m10-revert-tree`、head `78167b424e5911fbd072ab0a047d82a39526a10d`。全部13个必需jobs、38个必需steps成功，主持者已按精确输入接受。五份bootstrap的**Test步骤**秒数依次455/306/254/293/308；这是一份代码分支样本，第一片455s不改写为小于450，也不能替代未来指定“修复merge＋联合record”两次完整main各片均值严格<450s。[E23]

当前guards job `124745898484` 的source-revert九个HTTP样本全200，原整数纳秒见E24：

| 状态 | sample0 / 1 / 2，秒（舍入到0.000001） |
|---|---|
| no_statistics | 1.623263 / 1.833351 / 2.109275 |
| analyzed | 1.671037 / 1.892778 / 2.215361 |
| force_generic_setting | 1.666682 / 1.833927 / 2.146799 |

无统计组所记表统计为0；三组规模、历史增长、原4s/2s/7.6s均保持。`force_generic_setting`只说明已保存的session setting，不冒充新增真实PREPARE计划验证。九个成功响应和一次完整分支CI不能证明长期稳定，不能把全部收益归因于单条SQL。E24收集时“whole CI pending”是当时状态，后续完整CI由E23单独接受；原13ed/run41247708831仍FAILED。

## 三席独立审查与主持者核对

A/B/C分别在独立detached worktree审同一78167/bea20fde与26文件diff，各做真库或真实组合路径及独立源码变异。三份报告均为**0 Important / 0 Minor / 0 Nit / 0 Q**，无未核实产品怀疑或过度设计发现。主持者已阅读三份报告、相关探针/patch/runner，并解析原GoJSON、核对恢复与pins：A73、B45、C77份本席文件及C34个外部引用；不是逐行人工阅读所有raw。最终接受见E28，A/B先行收据沿其引用保留。[E25–E28]

| 席位 | 实际方法与独立控制 | 结果与边界 |
|---|---|---|
| A：完整树、事务/取消、同步/后台读路径 | 手写六字段oracle的真PG探针；未提交事务内/外/回滚状态；先确认nodes表锁等待再取消；查询错误链。A-TX-01丢事务选择、A-CANCEL-01丢Query取消ctx，各为0→1→0。 | 两变异分别出现3/2个语义失败，其他4/5个正控仍PASS；恢复轮各7 PASS。保留一个private未跟踪调查探针，tracked全部恢复，不合入产品。 |
| B：完整事实、来源、SortOrder及独立模型 | 12顶层/21节点unit基线与恢复通过，真PG组合3顶层/15节点通过；将模拟器原SortOrder改0。 | 固定source facts将rename错报move，独立domain模型也抓到；mutant19 PASS/2 FAIL，0→1→0。**后台property仍PASS**，不把所选测试全称为kill。WT最终clean。 |
| C：生产接线、库存、CI与原成本边界 | 仅把WriterDeps.RevertTrees接成非nil空树，TrashReads保留真实Store；真实同步/后台组合各轮4顶层/6节点。另跑新增测试/allocation共10顶层/15节点，编译清单1240顶层只计inventory。 | 0→1→0；同步原200变409 `page.revert_conflict/parent_gone`，后台任务failed/revert_conflict且done0。content-only和锁外title_taken正控仍PASS（共4 PASS节点）；恢复6 PASS。WT最终clean。 |

所有控制均在独立WT按备份逐字节恢复；没有把编译错误、panic或基础设施失败算kill。C独立核原race bootstrap库存495，五片167/39/114/123/52互斥且完整；allocation原两轮1612/1613次及C本轮1613次只证明原25000分配次数预算，不能代替总分配字节或HTTP延迟。A路径拼写与C的UUID子名/清单排序辅助审计首败保留，未计作产品失败；已知第2片三个UUID子名不强求逐字相同。资源观测只覆盖本人实际ID，无exe/startticks或physical lease证明。

主持者处置：三席范围及语义控制接受，无需产品修复提交，因而没有另造fix-check或重跑作者26变异的结果。原S2 B-M1及原席核对仍归S2自己的对象；本次四个独立控制也不并入S2或作者26项的数量。

## 接受与待落实

| 项目 | 本记录写入时的状态 |
|---|---|
| 产品及本地门禁 | 78167、TDD/历史HTTP、7项minimum、作者26项源码变异、fullstatic含postgen/fresh build、Web、native19/affected双轮、Cost02完整60/容量/五项related双轮及完整EW一轮均按各自范围接受 [E2–E14][E18–E22] |
| 代码CI、独立审查 | 精确41271794807 attempt1和A/B/C及主持者最终核对接受，无待修复发现 [E23][E25–E28] |
| 失败与限制 | 原13ed main失败、native中断退出未知/partial排除、Cost01磁盘前置失败及failed free未知、作者/审查辅助首败与存活正控均保留；250MiB目标及M12性能边界开放 |
| 正式修复文档 | 本记录及00/P3修订的正式应用、minimum/gen/链接核对、提交与精确doc CI待实际执行 |
| 修复集成 | 正常no-ff、修复main完整CI、联合四文件main record及其完整CI、指定修复merge＋联合record五片均值验收均PENDING；合并见00第12节 |
| Phase完成 | 指定配对和§8全项闭合后再独立五文件P3done，并接受其精确main CI；此前P4不释放，P5/M11不开始 |

联合四文件record与独立五文件P3done的**准备范围已根审接受**（E17，PREP-only）；实际merge/record/pair/P3done未发生。该范围不要求重做原S2已验控制，不单独创建旧S2三文件record，也不新增record-of-record。本文是三席及主持者接受事实的文档汇总，起草者不是第四位代码审查者。

## 证据索引

树外证据根缩写：`INV = m10/p3-s2/s2-main-merge-guards-investigation-01`，`FIX = m10/p3-revert-tree`。均在 `/home/parallels/.local/state/nervewiki/` 下；T7原CI日志路径另列。深层命令、每个文件与原日志沿下列已接受回执引用，不复制成长表。

| 标号 | 原件与 SHA256 |
|---|---|
| E1 | T7 `nervewiki-codex-m10-ci-cost-history-20261101-01/incremental-04/run-41247708831/collection/guards-job-124674497913.log`；`9a8a0bfff678c7c84a312b98421b8e2bfd3addf4d21227be8d726462e321bd8b`；sample行615/618/621、应用错误行648 |
| E2 | INV `root-revert-tree-implementation-review-01.json`；`b3c6a7b2ad815d181a108dc5184939a505dae94117aa2f5a5180ef8ee873a990`；实际 `revert-tree-implementation-01/full.diff`：`a13b4e29441e557b4a01f58bde204af4738e73ad0d613e04ea938a982ac47262` |
| E3 | FIX `host-decision-70-preparation.json`；`f0f3e5c18183e117d72016615a83f9ba3a5239c651036518d35702f23565cea7`；它的原阶段状态保留，不改写成最终接受 |
| E4 | FIX `host-decision-70-timing-correction-01.json`；`023d5ee76505fbc352901ab09c05b95d67959aa4942fc1697c26f12d93bbc201` |
| E5 | FIX `root-http-measurement-review-01.json`；`336c7f3ad2bd01a83d35420af9a1bbb023a3bb0459ef0909b027f7e1b26211bc`；包含原baseline、九样本、argv与原日志引用 |
| E6 | INV `author-auto-explain-analysis-01-timing-erratum.md`；`94fcb1e8bf93ea2c2a951b3a072e731657fee792af1b45c8e982bb9148fd263e`；绑定原报告与固定helper，六计划的限定结论沿原独立报告保留 |
| E7 | FIX `root-minimum-review-01.json`；`988a8b91363a9d4ee0d60bd498ecbf753eb37f20f18b96a4a8ffe6c921b5952a`；7命令、原输入/退出及short跳过分别列明 |
| E8 | FIX `root-author-mutations-review-01.json`；`c1158e8a138dfb0012d820b219f5ca589c1d7b8a6773c5ca9369fb39da4135c4`；26units/78次、RT07/19/20首败和RT18限制 |
| E9 | FIX `root-generation-mode-preparation-01.json`；`8817c04f4c02c9659eaa0f48027a9ca7540e55e940c68259ce90217a8ad03941`；76件native模式准备及原备份 |
| E10 | FIX `root-fullstatic-review-01.json`；`bbf1bcd399f5bdd346c2df44ef9089daa3a55e0f1d0a3b1c67dcbd93d516c552`；已结束session75315、原命令、fresh build及postcommit gen-check |
| E11 | FIX `root-candidate-input-03.json`；`3384363097e6a1cb318186e1db6bdb95a3450fb885d54877db963381dab3135a`；真实当前提交与legacy来源字段分离 |
| E12 | FIX `root-native-acceptance-02.json`；`0d2b6e061d9d44da2c0fcd7d3e4a4728d56188ff13e12f91f7d3ef09b60c24b3`；绑定原18阶段、新完整nonbootstrap、两份raw/session审计、affected双轮与原中断记录；内部pins及精确计时见该回执所引原件 |
| E13 | FIX `root-web-tests-review-01.json`；`bce6482b0cac6c2af4fe3ebd6f8ba5baeaff982c45c242a3da22edb6ce72ef25`；完整Web实际退出、182/3925和警告限定 |
| E14 | FIX `cost01-disk-diagnosis-01/diagnosis.json`；`be0261f7ff64fda98078b7f23df684158d1c1cd799d90269169d87e2000c96d6`；Cost01实际outer1/make2、前三条与第04条预检停止，失败free未知 |
| E15 | FIX `root-document-preparation-review-02.json`；`89eb127473b19cd31c501b82bbdff1dd91b51b4579422842a6de58b63d9c4c0e`；仅接受02准备范围并要求纠正SortOrder措辞，非正式产品三审；实际SQL固定于78167的`queries/nodes.sql:37–41` |
| E16 | FIX `integration-document-preparation-01/manifest.json`；`4c7c91011819ee04afe6a54559db704a276e60c0700af37a65481c4b6098a978`；联合四文件record/独立五文件P3done的准备方案；后续E17已接受其范围，实际执行仍待，不是完成证据 |
| E17 | FIX `root-integration-document-preparation-review-01.json`；`275f68d18a589a1e627f9ed65c874950da41aeb88eca4019f5e6567d7a86582a`；PREP-only范围接受，纠正03旧稿的“仍待根审” |
| E18 | FIX `root-local-gates-acceptance-01.json`；`06689a6a60a85d511cbdfa2cba47380a8093a2444427bbe1d32fb813fccf8e1c`；当前源码清单`root-native-release-01/source.json` SHA `3c5571886ba123a40fd4a9038b036cc21de72f1f3fb4b1b86e6d76283462dcf4`，保留提交前minimum/变异及76件native模式限定 |
| E19 | FIX `root-cost-acceptance-02.json`；`a26e05a6507109fbf7f18207dea89f28a8614408d500ddbb4469f23d868911e6`；完整60、两轮容量与related接受，Cost01不拼入 |
| E20 | FIX `root-related-guards-review-01.json`；`54c556925e6e44dd3c0b8abf97c53cccc6f43c158a112e698c65c751606d41f4`；原件实际位于FIX根，其第二轮日志在`root-related-guards-second-01/`；最终接受见E19 |
| E21 | FIX `root-source-http-cost-pair-01.json`；`9ccfc3ddf4362ebb7d62071c6e372ae0e66927413c0fa37a4428eeea535c6693`；两轮18个本地HTTP200，不作CI可靠性推论 |
| E22 | FIX `root-full-e2e-acceptance-01.json`；`ed0408df45b516f53ee83efefaeaac035c05f37291fde26a1668b3c296099fef`；full一轮353，actual service exit见E18所引原件 |
| E23 | FIX `root-code-ci-acceptance-01.json`；`5180bca78ec6e16436fefcb2de00b39038b888e06bf97a8364f502cb6910c621`；精确分支13jobs/38steps，非指定main配对 |
| E24 | FIX `code-ci-01/root-ci-source-revert-http-review-01.json`；`fbef1b74243e8b346dfdfc2c676513a0f131cd8e1f43e0cea58a1bf0f73ff30f`；九样本原纳秒与固定预算；完整CI由后续E23接受 |
| E25 | FIX `independent-review-a-01/review.md`；`b2741ca99eb4604fe46fd08bfc4e1dbec1e94a5265732d01c244c272cf4f6044`；真PG、事务/运行中取消两变异，private探针不合入 |
| E26 | FIX `independent-review-b-01/review.md`；`0ea033dbaeab260b91d1e931d080dd9f1bf2fec745a6d102814d43f81a4a70e1`；原SortOrder变异的两个实际kill及后台property存活 |
| E27 | FIX `independent-review-c-01/review.md`；`e376cbcff31a2d67bf505c93cf6a914165c01227a0d43122cb77a9ba95082365`；真实组合接线、原库存/CI独立核对与正控 |
| E28 | FIX `root-independent-reviews-acceptance-01.json`；`dfc0088abd0a2dc9e2677c84270fb9bb05391faff26751968d905d7148727618`；三席最终接受，引用A/B先行回执`b458c1c00fde9e5f5fbe43a102534c05a6691bea5828b59d97391115b888fc9e`；不代替文档、merge或P3done |
| E29 | `m10/p3-s2/root-s2-final-document-ci-acceptance-01.json`；`a22719c413c9354e08a323031e30c9338cb5efaa130c357167280add133b97e8`；S2原文档987d/CI41245802670 attempt1已接受，不能代替本修复文档CI |
