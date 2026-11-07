# M10/P4 S1（虚拟路径、目录、文件与导出）：审查记录

> 原记录日期2026-11-02；原合并补记日期2026-11-03；修复合并补记日期2026-11-03（Asia/Shanghai）。最终源码0b0df18c2ddd4b6779954458d37a617f66a42c5a / d6614eb5ef0a03ebe5aa2c2f3091b65f8541d296及各原始证据身份保留；旧过程与决定中的“当前”“待验”按原写入时点阅读。原合并见[文档与合并记录](#文档与合并记录)，旧fd失败和878测试修复见[原补充](#main-record-ci失败与会话fixture修复)，b4失败及f689正确grep夹具修复/代码CI见[后续补充](#b4文档ci失败与grep-glob-fixture修复)，986取消与ac84拆分见[调度补充](#986文档ci取消与guards调度拆分)，最新集成见[修复文档与no-ff合并记录](#修复文档与no-ff合并记录)：修复文档与修复merge的精确CI已接受；本record自身CI及新指定两main配对待验，S1集成未闭环、S2未放行。 最新后验随决定76的S2正式设计见[S1自然转录](#s1修复record后验随s2正式设计转录)：ca的完整CI与预登记e07/a1→ca/a1配对已接受，S1实施与主线集成闭环；本次设计自身minimum10/commit/postgen/CI待验，S2实施未放行、未实施。上文及下列旧pending均保其原写入时点。

> 本轮容量维护补充见[e650记录失败与bootstrap容量维护](#e650记录失败与bootstrap容量维护)：e650/a1失败、a2完整CI接受与指定pair容量失败分别记账；代码`af392f641f043e9497ce39abd7734ac3d55007c3`已正常提交/推送，旧字节native17、B-N1注释桥接、新字节minimum02、提交后postgen与原A/B/C第二轮本机/源码补核已接受；CI41556086210/a1因revert12.45394147s>原12s失败保留；预登记的同head完整a2已按14/40/11及原成本/guards/lint接受，原A/B/C03已核代码与完整代码CI、无新增I/M/N；本轮文档`3166bbcde7e23b4eb614b6367ea7e95c801e91cb`已提交并正常推送，精确分支CI41566238435/a1：完整接受；本轮no-ff main `e07cc1964238b3d248c791c124d9a9fc48dd9253`，CI41569049575/a1：完整接受。原段中的“当前”“待验”继续按各自写入时点阅读；本record自身CI与预登记merge→直接record两main配对仍待验，S1未闭环、S2未放行。 最新集成见[容量维护文档与no-ff合并记录](#容量维护文档与no-ff合并记录)。

| 项 | 接受事实与边界 |
| --- | --- |
| 对象 | 分支 `m10-p4-s1` 的四个S1实施提交及八个门禁/审查期代码、测试或CI提交，共十二个。当前候选0b0df18c2ddd4b6779954458d37a617f66a42c5a/treed6614eb5ef0a03ebe5aa2c2f3091b65f8541d296；正式最终接受对象仍由0b0df18c2ddd4b6779954458d37a617f66a42c5a / d6614eb5ef0a03ebe5aa2c2f3091b65f8541d296绑定。S2/S3未实施。 |
| 设计入口 | 正式设计 e5e8a0b3cffcc473aba9085b0965e3f2193f16dd，tree 8ff45b1579dcbc79250e0640a4aca4d4ee4c0072；main CI 41297112914 attempt 1，13 jobs/38 必需 steps 已由 ROOT 接受。这里只证明设计入口，不是产品或容量验收。 |
| 代码审查前置 | 77的本机门禁、CI41402585652 attempt1与三席原报告保真实历史对象；efad成本100/91控的77窄对应不外推到0b0。0b0的B-I1修复、minimum/postgen及fresh EW357已接受；0b0完整native19、96控/288过程、成本100及精确修复CI以本表最终事实为准；C-Q1补测与原席核修见本文正式审查记录。0b0最终源96条/288实际业务过程已ROOT接受：前86与后10保两控制器身份，各条0/1/0；另保第87条因原3GiB预检拒绝、未启动Go的记录；C-Q1 observer另账；0b0最终源新完整native19已ROOT接受：8项inventory、五bootstrap分片双轮及全nonbootstrap race；五片process均值341.073/271.625/292.684/297.700/250.199s，均严格<450s；指定main CI pair另验；0b0 fresh完整6targets/50recipe双轮100过程、1333 recipe-local identities及6case/24固定guards已ROOT接受；各pair原容量均满足，最大占半界65.443%；原首轮100容量失败保留，C-Q1独立补测另账；精确0b0 CI41454798547 attempt1已13jobs/38必需steps成功；10份完整server日志及6case/24固定guards通过，web182/3926、EW357单过；guards job1707/1800s，branch五片357/312/399/400/321s，指定main pair另验。 |
| 三位 fresh 审查 | 77三席原报告为A零发现、B唯一B-I1 Important、C唯一C-Q1证据问题；ROOT已核原报告与实际控制；77原报告不构成0b0核修结论；最终核修见本节事实槽。77初审A无发现、B有B-I1 Important、C有C-Q1；精确0b0第一次原A/B/C核修均通过，B-I1与C-Q1关闭，新增Important/Minor均0。各实施作者和本起草者不充当各自范围的fresh席。 |
| 修复核对 | 0b0修复反引号并完成新完整门禁、96条288过程与精确CI；独立C-Q1冷暖补测及等待修正已核实，原B/C分别关闭两项，原A无回归；原件与逐席结论见下方正式审查记录。 |
| 文档与集成 | 原a1f成功、fd record两失败、b4 glob失败、986文档guards超时取消及878/f689两项测试修复和精确代码CI保留原身份。修复文档`2fc103dd`与no-ff `97c9ad8e`的精确CI已接受，见[修复合并记录](#修复文档与no-ff合并记录)。本record自身CI及指定“本次修复merge＋本record”两main配对仍待实际接受；S1集成未闭环。 |

## 对象、提交与证据身份

| 单元 | 实际提交及 tree | 已接受范围 | 本稿不外推的范围 |
| --- | --- | --- | --- |
| S1-1 | 5225f05b79f79ebfce4b02ec5ba4e0078034dab3 / db6660d2c787ad95b676f7537a89b09eed3f87e6 | 44 文件作者候选、11 控制、8 项 minimum、提交后 gen-check，均有真实收据。 | 不当作 S1-2/3/4 或整个 Step 的最终验证。 |
| S1-2 | 124b055f4fd3712f414864fdccf3457af9589ea6 / 407d6441f67c19b2666238ca0f10d3af1f03a23b | 34 文件候选、16 控制、10 项 minimum、提交后 gen-check。 | 原控制后的格式/注释与 zero-parent fake 修复单列，不虚构全部控制重跑。 |
| S1-3 | 原 167c73b5ff55c5751e9664ef99e0044c43d1fe83；现 15b019cdc752469495b5ef25dbf0694727197807 / 同 tree e1e10cbabe1b314cf71745006f9875cf0a790416 | 34 文件最终候选、22 条最终控制腿、10 项 minimum、原 167c 提交后 gen-check。 | 旧 raw 仍绑定 167c；不得重标为 15b 上运行。 |
| S1-4 | c1120ea003d2a46e7373ffd80c9245c845bbd82a / c8d0c2e22c1359ff4c39754591efe2c3ece359f2 | 初始 17 文件冻结 + 两文件 lint-only fix-01，实际提交17files/2546+/14-、parent15b；作者 38 项有效控制、72 校准样本、3 条正常 targeted 成本/36 样本/12 guard events、3 条完整 DirectoryIndex race。ROOT已核作者包与上述各范围。 | 两文件修复后minimum-02全10项及c112提交后gen-check实际0已接受；正式全门禁与成本收据 S1代码、完整本地门禁、精确修复CI和三位原席第一次核修已接受；当前进入Step文档与主线集成，合并见00第12节；S2/S3仍待实施。 |
| 门禁期L1兼容修复 | 9260d87c86e30b8ae05de14091622335c71c5900 / 0ee4bab9f0b7fdc004b256e2827b3af1c5b5f2a1，parentc112 | 一行首开可见性轮询；原10s/30s与后续流程不变；完整10项minimum实际0后提交。 | minimum绑定c112+一行dirty输入；9260 postgen实际0、新完整EW已接受，generic targeted和八项inventory分别接受；后续native/cost/正式审查按下文最终记录分账，不冒称新产品功能修复。 |
| 门禁期Markdown分组 | 496d709bb98faa7c1a1e12812b58d9a88055f972 / 97d0f4a82e55797a174aa9bbecb6814104d4b65b，parent9260 | 仅Make拆原Markdown top为两个完整leaf；各15m/count1，算法/夹具/样本/预算不变。minimum10真实9260+Make dirty；postgen/new8inventory真实496d均0。 | 当时native19和旧costs/lint64保9260身份，新guards36与496d fresh EW另账。该Make-only对应不适用于后来renderer生产修复，不能替代最终全量重跑。 |
| CI日志传输修复 | b01eebbb234af3e03010ad60abb428eab19771a3 / 365c710ea10b30c3aa8df084a279d41d4a9e5d51，parent496d | 仅workflow与preview脚本：raw前置tee、1KiB有界preview、always上传；Go/Make/测试/预算不变。最终1024输入未另跑本地minimum；保留496d+4096初稿的十项真实结果，随后仅两CI文件改变阈值与注释，另有窄传输实测和产品源对应。 | CI41353659599 attempt1全13 job成功及10日志完整核验只接受b01；不替代后续修复源的最终CI。 |
| 最终显示字节修复 | 9c0504ef7a7a0e3a3588ae35ded8cc1839ad7ced / 94c1ed0ab2052e2f75b28cb904c081cbe552bddb，parentb01 | 两文件精确采用独立WT补丁：256/500按最终转义UTF-8字节计，含省略号、不拆码点/转义对，destination与整文件预算不变。minimum10真实b01+dirty实际0，9c postgen实际0。 | 作者domain0/1/0/1/0是其独立496d+补丁输入；不改标到9c或5a97。其后完整控制/门禁/CI/三审须按实际新源另验。 |
| scope实际cause诊断 | 5a97e9691ce4d2d62cba6f1c641d82e50bafc4ea / 643814f51dad42cad891867eb8d9651591d98efc，parent9c | 仅virtual_export_test.go包装实际唯一contributor，同步t.Logf原error；ctx/scope/sink调用及返回不变，ZIP/业务断言不弱化。9c+dirty两E02完整0/1/0与minimum10实际0，5a97 postgen实际0；该5a97 fresh-build EW357单过保原接受身份。 | 不升级旧E02泛化export failed；成本fixtures/sampling/24limits不变但整文件精确源映射另审。其后完整控制/native/cost/精确CI/正式三审不由这些局部接受替代。 |
| generic成本测试pool修复 | efadfcb669efd1ef8b87a79565bfe7ffb242bc85 / ba6ec64627561905256f123f380a51e38970bc0a，parent5a97 | 只改index_cost_test.go：复制production pool.Config，继承AfterConnect（jit off）和MaxConns=2，仅覆盖force_generic_plan，计时前核实际设置。fixture、三样本与原150ms等预算不变。minimum session63393实际5a97+单文件dirty，efad postgen session75655实际0；源码对应及efad fresh EW session96761/357单过已接受。efad完整native19、cost100及原88+新3共91/273随后各自实际完成并由ROOT接受。 | 5a97 CI41370811260失败原样保留；此前10个候选诊断过程不计入91/273。efad结果不能重标为77新运行；77未受影响成本/控制的沿用已有ROOT明确窄对应接受，最终Step仍另待。 |
| bootstrap分片移界 | 77c5fc474a95614a7984d6532a4f14281fdcb048 / 4cd7e8743e053ec88773b8a92fb1bbacfceedda1，parentefad | Makefile:240只将完整TestVirtualExport由第三份移第二份；原race/count1/Go与job期限不变。minimum真实efad+Make dirty、77 postgen和源对应已接受；随后77新native19/session24201、fresh EW357及精确CI41402585652 attempt1已接受，三席原报告也已产生。 | efad成本100/控制91供77沿用的明确窄接受保原过程身份。上述只接受77，不替代后续0b0生产修复的全量验证或指定main pair。 |
| 正式B-I1反引号修复 | 0b0df18c2ddd4b6779954458d37a617f66a42c5a / d6614eb5ef0a03ebe5aa2c2f3091b65f8541d296，parent77 | 三文件：fileLabel把反引号加入原escape判断，domain补真实渲染身份/256与500B边界，bootstrap补export及notebook/account入口各index/log六分支。ROOT观察真实red→green，minimum10、postgen及提交源对应已接受；未改destination、原预算或产品决定。 | 新全门禁/96控制/成本100、C-Q1独立冷热证据、精确修复CI与原席核修由最终记录分别收口。 |

ROOT 在作者交权后确认远端只有 main=e5e8、没有该分支，随后仅修正 S1-3 未发布提交遗漏的 Co-Authored-By。167c→15b 的 tree、parent=124b、作者字段及全部 3637 源码/embed 的 bytes/native mode 与 status 不变；备份 ref 与两份 commit 原件保留。此为提交说明修正，不是代码重放或改写已发布历史（`MESSAGE_CORRECTION`）。

每份结果都保留当时 HEAD/tree、完整源身份、生成物、真实 argv/cwd、child exit/耗时、raw 和恢复证据。Go 原 timeout、heavy.sh/GNU mkdir 全局锁、锁后 native ≥3GiB/T7、固定 GOCACHE、GOFLAGS=-p=1/GOTOOLCHAIN=local 保持。生成物 0664→0644 只作为明确登记的原生生成 mode 变化，内容/Git执行位与实际 gen-check 仍分别验证；普通命令不豁免 mode。

证据导航见外部固定原件索引。原04的129项保留，本轮追加77后续接受、正式三席、0b0修复与C-Q1协议等已产生原件。准备文件仍只作准备历史，不等于执行/验收。最终20项事实以各节的实际收据为准；原87腿/93尝试附录保留原始记录；仅六处诊断摘录增加行内代码定界符以免误判为文档链接，摘录文字及原始日志未改，不复制大raw。

## 建成的行为与范围

S1-1 在 page domain 统一 VirtualLookup/VirtualFileError（保留 errors.Is）；NodePaths 的实际调用点、batch 五个模拟解析点与 list 目录共用四根键规则。只有规范 index.md/log.md 后续进入生成 read；alias/虚拟父目录/其他路径操作答 page.virtual_file，move 最后保留目标仍由 CheckContract 判断。ChildrenCursor 的 V 与真实 O/I 分域，V2 从真实首项继续，负 sort 不丢；glob 按 K，真实 SDK Fit 的实际保留尾项决定 next。OpenNotebook 在原 100 项内放入两虚拟项，真实 page/asset counts 不加二；真实 empty 页仍有 byte_size=0，虚拟项不伪造 bytes/version/updated。两 scope、码表、工具描述与实际可达调用同步。text.go、permission_matrix_mcp_test.go、mcp_reads_test.go 是对原 187 文件计划表的实际落点补充，不是新增产品决定。

S1-2 的 REST 与 MCP 使用同一 ListIndex 与 type-key/id 次序。actor→cursor 400→资源无关 filter/parent冲突/limit 422→授权/资源 404 的次序保留。有效 frontmatter 的首 ASCII type 键必须为 JS-trim 非空 string；不能跳过首非 string 去取后一个。REST 才设置 Catalogue/ExcludeArea：排来源严格后代、保留根，完整有限 eligible ID 输入按 ≤512 批到 EOF，独立于 type/tag/parent/页游标统计 exact types，≤100 项/实际 types JSON≤32KiB，required types_complete 与 unfilterable_type_pages 明示省略与长值页数。exact 指存储完整值（既有 NUL→FFFD 归一），不是显示/key/trim 前缀；筛选仍以 1024 UTF-8 字节为界。REST 空 items 仍返回 mode/source_area/catalogue；MCP 保留原便宜空返回和精确 type SQL，不承担 catalogue 扫描。PagePath 的真实 Title/ParentID 与 nil 根/真实 zero UUID 父页分别处理；目录 UI 留 S3。

S1-3 通过只读 mode hint 分流；规范虚拟 read 独占一次现有只读 RR，在内重新授权并重读 mode/source root，再用同 ctx 读全部生成数据和 Cut。降级时先结束 snapshot，再在 outer ctx 重新授权走普通读；普通真实页/附件仍无事务。GenerateInSnapshot 不拥有第二个事务；实际 Tools 缺 Virtual 时 panic 位于 `(*page.Module).Tools`，不是 page.New。只读服务的四个窄端口和 bootstrap DTO 不跨模块内部引用。renderer 写英文分组、真实 footer；显示中和/截短与目的地身份分开，read 保留完整 page 路径、export 使用完整 plan path。后续9c把256/500落实为最终转义显示UTF-8字节，包含省略号且不拆码点/转义对；不是保留limit原文字节后再额外加marker。两 scope 真 SDK 的 offset/column/UTF-8/48KiB 与 RR 改名/mode/visibility交错已有作者定向证据。PageRead 联合与 24 个版本读取站点已同步，不能把这个静态审计称浏览器通过。

S1-4 将早构造的同一只读 wiki service 接到实际 transfer contributor；Scope.Nodes 的 ID/Asset/Path 完整映射在 export 已有 snapshot 中使用，whole LLM 才加 index.md/log.md，standard/subtree 不贡献；无 nested snapshot、lint 或持久虚拟节点。真实 job/download ZIP 验证 contributed、全部 map consumers、folder-only/碰撞改名路径、非法映射、NFC reimport、无 meta/首 meta 超界 fallback、保留名冲突、完整内容与编码后字节界。E05 实际破坏的是生产 ReadMeta→Contributed 映射；metaPaths 是测试 helper，不能将只改 helper 算生产 kill。E04 在合法 export profile/scope 下放开实际 Inbound 查询条件，让非空 SQL trace 断言失败。

F07 的最终合法百分号空格夹具是 x%20y.md 与 x y.md，不用会被 CheckTitle trim 的标题尾空格。字面 `&amp;` / `&sol;` 路径经 goldmark→HTML/URL 解码会与 & 或斜杠路径撞目的地；S1-3 已将字面 & 编为 %26，S1-4 进一步核对实际 ZIP entry/body 最后一跳。raw 2/AGENTS 2 的重新导入限制保留；不借本单元实现 M11 的存量升级。

LW10/LW13 故事源码已交付：两 MCP scope、真实非空 grep、whole/subtree ZIP、Guide/Guide/index.md 非空 subtree entry、真实 reimport 数据库断言、目录 REST/MCP 分页与独立 ID/type/count。MCP 字段为 pages，REST 字段为 data。作者未执行这些故事。ROOT首轮完整EW已运行，原生child实际exit2；357cases中356 expected、1 unexpected、0 skipped/flaky。LW10的两case与LW13的standard/llm_wiki两case全部PASS/retry0；这四项执行不能替代完整EW接受。唯一L1首败及后续兼容修复见下，9260的新fresh build完整Chromium EW已由ROOT接受：session2537/native实际exit0，357 PASS、0 retry/0 skipped/0 flaky，原生进程176.65847533475608s；LW10/LW13在该完整轮内通过。原首轮356/1与原L1独立重放0保留，不用重放或后续绿覆盖首败。当前 S1 全套浏览器配置为 1 个 Chromium，S3 才落实至少 3 个 browser，不将 P3 或 S3 的三浏览器义务提前算给当前样本。

## 已处理的实施期发现（不是正式三审）

| 编号/性质 | 真实问题与处置 | 当前核对边界 |
| --- | --- | --- |
| R-S1-1 / Minor | list literal index/log 误建议禁止的 children 路径；pageHint 只排 synthetic 叶，nested/standard 真页提示保留。 | 独立业务 red、最终定向绿；ROOT预读接受，随5225。 |
| R-S1-2-F1 / Minor | fake 将 nil 根与真实 zero UUID 父混淆；按 map 成员位区分，独立非空1/2页 counterexample。 | 真实产品 SQL 本已正确；最后 fake red/green与后控制差异单列，随124b。 |
| R-S1-3-F1 / Important | 字面 ampersand 的 Markdown 实体身份碰撞；私有 destination 编码加 %26，真实 goldmark 解码 + 独立 ID 控制。 | S1-3 red/green/0-1-0 与 S1-4 真实ZIP两实体腿；同tree15b。拟文档决定编号见下。 |
| R-S1-3-F2 / Minor | percent20 第二标题不合法、会 trim；换合法内部空格两标题。 | 仅两字串测试修复，原产品不变，新控制0-1-0；旧轮保留。 |
| R-S1-3-N1 / Nit | ReadPath.Execute 注释笼统称 unlocked/no transaction。 | S1-4注释已区分普通与虚拟 exclusive RR；行为不改，随c112提交。 |
| R-S1-4-P1 / Important | FirstChanges(1000,3) 在实际 custom plan 扫完另一大本的 revisions300000/items300002。 | 两 metadata JOIN 改相关 LATERAL LIMIT1；revision子查用单行anchor LEFT JOIN保真实缺项NULL和sqlc nullable，17字段/type与167c一致。n=3手写item-only/version-only/both metadata/order；原200ms与完整1000cs计划/两个单JOIN 0-1-0均已ROOT核对，preliminary closed；正式三审及原席结论另见本文相应记录。 |
| F08覆盖发现（不虚构正式编号） | 原 next-item 编码前预算fault为真实漏洞，但原夹具没卡在两长度之间，0/0/0。whole-buffer替代fault产生O(n²)等待超时，不算kill。 | 合法235个%路径夹具杀原fault：实际2,097,199B，超过2MiB47B；0/1/0。两旧尝试永久保留。 |
| minimum测试 helper lint | 首轮00–04成功，05 lint-go真实exit2，06–09未跑；两新成本测试共6诊断。 | fix-01仅同9项表函数/5调用、同13项局部query列表、显式忽略Close和%w/状态分支；gofmt0；guard/fixture/SQL/期限/测量不变。ROOT静态接受；其后minimum-02全10项实际0、提交c112及postgen实际0已接受；最终完整成本另按本文收据记账，不编产品决定。 |
| CI日志传输（独立窄诊断，不是正式三审） | 496d CI guards在日志消费积压下触发原30m job上限；未取得该轮export terminal，不能视作通过。4096B preview初稿仍可挤掉36k注解里的FAIL/panic。 | 最终1024B preview/raw上传随b01；独立传输反例关闭该Minor。b01精确CI及十份完整日志已接受；无SQL/fixture/Go或job timeout放宽。 |
| R-S1-DISPLAY-BOUND / Minor（ROOT预读） | 旧fileLabel保留500B原文后加3B省略号，转义还会扩张；旧测试期望不能授权偏离message≤500B。 | 9c最小共用helper计最终显示字节；ASCII/UTF-8/转义/恰好limit与destination完整性的独立domain baseline0→业务red1→green0→单fault1→restore0保原输入。其后efad的91实测保历史，0b0的96控制另按最终记录核验。 |
| scope实际cause诊断（不虚构正式finding编号） | 原E02 standard/subtree只打印Report指针及export failed，无法证明单fault抵达预期业务cause。 | 5a97只包装原真实唯一contributor同步打印实际error，两腿分别观察missing source root / incomplete export mapping，9c+dirty各0/1/0。返回与ctx/scope/sink不变，旧日志保持历史且不升级归因。 |
| generic测试pool设置（ROOT预读，非正式finding编号） | 旧generic pool从URL重新ParseConfig，未继承production AfterConnect/JIT-off及MaxConns；5a97精确CI的generic过滤中位153.380458ms超过未改150ms。 | efad改取pool.Config并只force_generic，核实际jit/plan_cache_mode；fixture/三样本/常量/原plan测试不变。候选诊断5a97+dirty保原身份：缺type index时generic过滤16.390291ms仍过150ms，完整IndexPages三阶段超原1.025s，TestIndexPlans给12条非空索引名失败。efad最终新三控已实际完成、ROOT接受并纳入91/273；旧prepared材料及10个诊断过程不改标。 |
| bootstrap容量移界（ROOT既有规则处置，非产品决定） | b01/5a97第三份Test469/479s，第二份有旧样本余量；单top移动方案已获ROOT接受。 | 77仅Make将完整TestVirtualExport移3→2，不改测试名、race/count1或原期限。旧分支样本与静态预测不是77新run或main pair；77新EW、后来完整native19及精确CI均已接受，0b0的新验证见本文最终记录。不新增决定76。 |

前六提交阶段的两项门禁维护，单列于上述产品实现发现：首轮完整EW实际357cases=356PASS/1失败（0skip/flaky，retry0），LW10/LW13四case全部PASS；唯一L1 Ctrl/Meta popup首个heading断言卡于Playwright1.63的pending-navigation precheck，实际HTML/API200且heading已显示。底层pendingDocument来源未证明；原L1唯一独立重放实际0不消除该首败。 随后门禁期新增普通测试兼容修复9260d87c86e30b8ae05de14091622335c71c5900（tree0ee4bab9f0b7fdc004b256e2827b3af1c5b5f2a1，parentc112）：仅L1首开popup断言改为同一strict role的expect.poll(isVisible).toBe(true)，保原10s expect/30s test及后续reload/全部流程；在c112+一行dirty输入上完整10项minimum实际0后提交，不将minimum重标为提交后运行。

以下native/空间恢复/Markdown结果属于原9260/496d历史epoch；9c生产变化后不再将这条Make-only对应当作当前验证。

首次native八项compiled inventory实际通过；bootstrap首片Go/make实际0（327.788490364328s），collector却因旧identity漏PAT/index child退出1，这次元数据核验失败保留。11遗漏child（PAT1、参数绑定4、自由文本6）补全后，9260的三个完整通用REST top/245节点的一次独立targeted race已接受（session8267、实际exit0、13.100079840049148s）。第二次native八项实际inventory也已接受：499 race tops、531 nonrace tops、116 packages，五片168/39/117/123/52；inventory不是测试通过。这些都是工具期待清单与实际执行范围的分别核对，不是产品功能失败或预算调整；这些局部接受本身不等于完整native接受，后者见本记录原9260 native19的单独收据。

native执行02（第二次完整执行尝试）在bootstrap-4-first的child启动前被原3GiB空间预检拒绝，controller/session51488实际exit1；前三片各一次已结束结果保留，该次尚未得到五片双轮或完整nonbootstrap结果；随后有限恢复已另接受完整原9260 native19。成本第一次完整尝试session25386实际exit1、costs-lint-first整target exit2；只完成该target前三条recipe，第四条child没有启动，整套为incomplete。收据root-capacity-prefight-failures-01.json分类为容量预检中止，未观察到该中止对应的产品测试失败。瞬时占用来源没有证实，不将收据diagnosis中的PG临时存储推断写成确定原因。

ROOT随后只实际清理未使用的~/.cache/go-build：go clean -cache实际0，显式GOCACHE指向该目录；清理时无Go/test进程。native可用空间从4,227,272,704到9,378,701,312字节，当前T7任务cache的device/inode/mode/uid不变；原3GiB门槛未改。后续native有限恢复实际0并接受；cost-collector-02/session42698完成全部98个原进程、各child0，但Markdown容量失败，旧aggregate仍失败。清理事实不能推定瞬时占用的具体来源。

ROOT已接受窄gate obligation map：原五bootstrap片双轮、all-nonbootstrap一轮及每条成本recipe双轮义务保持；随后仅Markdown一条按决定74成为两个完整leaf，最终50recipe；当前合同没有额外affected第二整模块race义务，不新增该门槛。新top的race用时和成本process/算法/指定mainPair仍各自按原条款记账。

中间生成不能改写成成功产品版本：S1-2 的 filter_type 经两次 sqlc 推导不符（string/bool）后才采用私有空串投影→公开 nil；S1-4 两次中间 FirstChanges 生成仍得到 int32，其中 `virtual-export-plan-green` 实际编译失败。最后 anchor 保 NULL/nullable，而不是把0在adapter改nil。故障期不gen；两个 FirstChanges JOIN 逐个在SQL及其对应generated片段同步安装同一查询病灶，独立恢复bytes/native mode。

## 主持者的决定73、74与75（负责人可以改判）

固定efad的00最新正式编号仍为72，原50锚与5a97/496d相同；本候选准备73/74/75，未发布或提前占号。正式记录采用状态：ROOT已正式采用73、74、75，均为“主持者的决定（负责人可以改判）”；原预算与接口边界保留，技术修复和C-Q1补测不另增产品决定编号。发布工具核三个旧编号冲突和输出各仅出现一次。

73本身不改显示预算：字面&→%26保合法`&amp;` / `&sol;`目的地经过Markdown/HTML/URL解码的身份。显示规则不变，不trim/截短/中和destination，编码后字节守原cap；renderer与真实ZIP独立验证。技术lint修复不编产品决定。

74：原Markdown完整top两轮process为207.96296564117074/207.57106755720451s，均值207.76701659918763s，×2.4=498.6408398380503s，超过原15m半界450s。算法/所有节点PASS不等于容量通过，旧aggregate保持失败；原因未证明，不补硬件归因。ROOT明确采用单recipe例外于原“top不可分”：^TestTheAppsMarkdownCostsAboutItsSize$/^to_a_page$ 与同top /^to_an_image$ 各完整leaf独立15m/count1，parent和指定非空leaf各RUN/PASS，两leaf互斥且并集等于原两项，重复parent按各进程记账。其他45legacy+3P4字面recipe/次序不变，不改Go、夹具、维度、GC、采样、early-stop、算法/请求/worker预算或heavy/T7/3GiB。

上述是496d时点的Make-only处置历史：3386源文件/252embed与命令对应支持原native19/旧costs+lint64保9260身份，两个新guards target18×2=36保496d身份；分组minimum为9260+Make dirty，postgen/库存/EW各保原件。旧64与新36合成不能复制、重标或沿用到renderer生产修改后的最终修复源；当前六个完整成本targets须实际跑100进程，50recipe/1333 recipe-local nodes及全部容量/算法guards仍逐项验收。

75：256/500常量不变，按最终转义显示字符串的UTF-8字节计且包含3B省略号；恰好limit不省略，超界不拆完整UTF-8码点或Markdown转义对。只用清晰小helper，不改变完整destination、节点身份或整文件cap。9c修复及其独立边界反控保真实输入；0b0最终96条须按其实际源码核验，不把旧87加作者局部控制拼成修复后的同源通过。

决定71/72仍保原设计；S1只落实72服务端及测量，目录UI仍S3。

## 变异、定向执行与失败历史分账（旧epoch）

| 单元 | 设计关联/最终控制腿 | 实际尝试与命令 | 作者运行库存 |
| --- | --- | --- | --- |
| S1-1 | V01–V11的list腿，11；V04为旧guard回归 | 11轮/33命令，全0/1/0 | 50测试：29个0、21个1；没有作者gen。 |
| S1-2 | D01–D11，D04a–f分开，16 | 16轮/48命令，全0/1/0 | 60测试：36个0、24个1；另4gen均0。 |
| S1-3 | V11 read、V12–V19、F01–F08及补充，22 | 实际25轮/75命令；最终22腿/66命令，3轮历史另列 | 90测试：58个0、32个1；另1gen为0。 |
| S1-4 | E01–E09逐map/非法映射、F07/F08真实ZIP及两个FirstChanges JOIN，38有效 | 41尝试；36个E/F有效+2个SQL有效，3个不纳入 | 148测试+4gen=152：104个0、48个1；其中4gen均0。 |

上述87条仅为各自旧输入当时纳入的独立控制腿，不是136个设计ID，也不是当前最终树上的87次重跑。旧E02两条泛化export failed日志只证明当时失败，不作为实际cause的证据；新的原因诊断0/1/0独立记录。完整逐腿表已附本记录，独立来源为 `control-ledger-draft.md` / `control-accounting.json`；V11的list/read、F07/F08 renderer/ZIP不能合并记成一个运行。S2/S3控制与五个S2交错不因S1读/导出交错而宣布通过。

S1-1保留初始缺虚拟前缀、路径码、metadata、literal hint等业务red；app-domain-mcp-green名含green却exit1（旧顺序/Refused断言），candidate-short-green是:=重声明编译失败，其后domain与完整short才0；nil map与SDK边界未命中为fixture失败。实际Fit最终为40/42，完整seed手写首段尾index，后段恰log/raw且Next空，不能沿用早期仅检查尾项的草稿。

S1-2保留typed编译错误、D11非法fixture标题、PermissionMatrix selector漏prepare、空data错误oracle与zero-parent fake真实red。D01/D02只改实际执行的生成SQL文本是受控单fault，不是持久手改生成物；源/生成物/原生mode都恢复。控制后6个生产文件仅格式/注释/YAML空白，2个测试为fake修正，有逐字post-controls差异。

S1-3保留canonical字段/pointer编译失败、零ChangesetCursor造成的空log、[]byte/string编译错误与无效tags未产生长行；旧V12 synthetic先被SDK成功/响应上限断言抓住，不冒称抵达后面的next_column断言。旧两轮V12及旧percent20保留为历史；修amp和合法标题后新腿作为最终证据。

S1-4保留E01真正ZIP缺index.md的业务red；导入缺base_version、raw仍有正文/AGENTS入链的fixture失败；read深度/禁用search worker排空等待错误；第三轮read有18探索样本但整体exit1，不进入正式校准。absolute旧0/0/0是重叠防御等效；F08原0/0/0是真实fault未触界，不能归为等效；O(n²)超时为无效kill。FirstChanges真实全扫、非nullable中间生成/compile失败和lint首败全部保留。

## 成本算法、SQL与进程容量

S1初始保原46条完整Make recipe及其顺序/预算，新增3条：costs 的 `TestIndexCatalogueHasBoundedWorkAndOutput`（linking/postgres），guards 的 `TestVirtualIndexOfAHundredThousandPagesTakesItsTime` 和 `TestVirtualExportOfAHundredThousandPagesTakesItsTime`（bootstrap）。三条均!race/non-short/default Go10m；原tools recipe的race例外仍在，不能把全部49条统称!race。原49条/98进程实际child均0，但Markdown完整process容量失败，旧aggregate未接受。决定74后为50条：18costs/14lint/18guards；这是历史分组规模；最终repair改为同一最终源的六个完整targets/100个新过程，不沿用旧64+36合成结果。最终实际记录如下。

0b0最终源的新完整成本重收由session66120实际完成（terminal09a970，exit0）；ROOT verifier session63440/terminal4dc5f8实际exit0，接受收据`root-backtick-cost-acceptance-02.json` SHA256 `ee629651bc2af59ee9a65e942aa20f5f6dc4550ea14244eaf9febcc825fb29fb`。本次独立目录`root-backtick-costs-execution-02`的completion为`aff879d688aee5173b78fbf8e8c7f5132a87823a7eda7a9649ba5c256f8f0fbf`；原首轮失败目录01不覆盖、不拼接。

六个完整targets真实执行18costs、14lint、18guards各两次，共50recipe/100过程，1333个recipe-local非空RUN/PASS身份及原tools race例外均核对。369项原件refs、所有实际child exit/argv/cwd/process计时、六个target和旧/新容量verifier完整终态已验；源3387项/embed252项及native bytes/modes/tools保持最终clean0b0。各pair均按完整process均值×2.4≤原timeout一半，不换用package用时；最大者test-go-guards-16为81.80410131020471s×2.4=196.32984314449132s，相对原300s半界为65.443281%。先前失败的guards15本次真实两轮78.14620853727683/78.10098791215569s，均值×2.4=187.49663573931903s≤300s；不混入原首轮的较快样本。

index-catalogue、virtual-directory、virtual-read-index/log、virtual-export-whole/contributor六类原24个预算数值逐字不变，新guard接受收据`new-cost-guards.json` SHA256 `6c13a083ea152cdd1a649cd454ac3c07fc9884ace1bef94396e4ea5db689a4df`。renderer在历史校准后有生产修复，当前字节另行绑定，不声称与旧校准输入完全相同；采用固定预算的原明确处置仍保留。C-Q1的PG shared_buffers冷暖与外部等待修正独立记录，不更改这六个targets的CI循环；exact修复CI、正式原席核对与main集成也不由本机成本接受替代。

0b0修复后的首次完整100过程（session53111，terminal5390b8，exit1）保留为容量失败：100个recipe child及六个target verifier均exit0，但`test-go-guards-15`的两轮完整process为173.24285508319736/81.54375887196511s，均值×2.4=305.743936746195s，超过原300s半界。其余49对满足原容量算式；失败发生后新guard aggregate尚未执行，不能把目标测试通过写成整套接受。后续fresh100由session66120在完全相同的50条argv、源/夹具、24个算法预算及原进程期限下重新完整收集；不从首轮挑选较快过程、不拼接两套样本。后续实际结论只依上方最终收据，首败不被覆盖。

该首轮异常的88.930410s位于Go package PASS之后。独立诊断核对Go的Close→Trim路径、同秒`trim.txt`和24h维护间隔，强烈指向Go缓存退出维护，但未取得当时syscall归因；不能声称某个IO调用已被证明占满该时间，更不能扣除它后改判容量通过。ROOT接受诊断收据`root-cost-repeat-01/root-diagnosis-acceptance.json` SHA256 `3a999cf8f921d02beb6f9d54073ebe69f478088175cb65b8807fbdeeb778e8e0`；完整重收决定`root-cost-repeat-01/decision.json` SHA256 `38092c63cb6da94f62745f2548164b147d3f4ae742130d5ef176590c11833a17`。原执行`root-backtick-costs-execution-01`与新执行`root-backtick-costs-execution-02`分开保存。

100k夹具的current/history/parser完整；更大另本200k、200目标各1001来源、10合法source页、实际asset、>1000changesets；SearchIndexInterval=0明确没有worker，待投影数如实记，不声称FTS追平。Catalogue长型夹具有99980个distinct合法1024B转义type、18个1000属性的无type页、2个5MB空白慢值，100000ID/196批EOF，15项/30961 JSON字节及精确count为独立oracle；短type旧预算保留。

测量分离：真实SDK HTTP transport只读tee捕获ActualHTTPBodyBytes，另报ReserializedResultBytes；REST4s请求实际catalogue=true，真实MCP index catalogue调用为0。whole export从job到真实ZIP下载，另独立观测原贡献者在原RR/sink里的算法段；内层不额外GC，进程与observer并发开销如实计入。seed、登录、验证、EXPLAIN在算法计时外。Wall/Go进程CPU为ns，TotalAlloc和1ms采样heap增量为bytes；采样peak不是精确最大值，不是PG CPU/RSS。

Catalogue生产DefaultQueryExecMode实际为cache statement、显式override空；同连接prepared计数首样本5custom+191generic，之后每样本+196generic。真实custom/generic执行及参数OID/编码、独立rawSQL EXPLAIN、物理rows/buffers/TOAST分账，不把EXPLAIN说成named execution自身plan。既有LinkTargetsByIDs在noStats的窄node bound按目标100002+每ID至多10祖先，仅该query/相关nodes适用；Inbound link关系按ID×1001、独立max_count1000；其他metadata无跨本全扫豁免。

72个成功校准样本分为24 export whole/contributor、36 read/REST、12 catalogue：每个算法两独立进程，每state各3。ROOT批准复用FirstChanges items控制的成功baseline/restored两完整进程作export两轮；fault不计。Read两轮全输入相同；catalogue两轮算法/fixture/observer/linking包相同，6个无关文件差异另列。所有校准事件calibration=true、Accepted=false，不算预算门禁。

| 固定case | Wall ns | Go CPU ns | TotalAlloc B | sampled heap增量 B |
| --- | ---: | ---: | ---: | ---: |
| index-catalogue | 4527240378 | 1668177000 | 699248952 | 141228192 |
| virtual-read-index | 2591535876 | 1625994000 | 1095755136 | 270810024 |
| virtual-read-log | 68649624 | 17772000 | 16709208 | 16709208 |
| virtual-directory | 1865567625 | 936228000 | 554644536 | 151983048 |
| virtual-export-whole | 5738227254 | 6159636000 | 5306074272 | 528859608 |
| virtual-export-contributor | 2443183002 | 2740134000 | 2879099568 | 335038608 |

这些literal按两轮各state中位数的最大值×3经ROOT逐项重算接受；普通CI calibration=false、fixed guard，不用当轮慢值自动上调。初始冻结前3条正常targeted真实exit0，36 samples/12 accepted guard events；随后3条完整DirectoryIndex race（bootstrap/linking-postgres/page）12个parent/subtest实际RUN/PASS、0skip。同一输入与hash均核对。其后lint-only修复未动数值/算法，原两条源对应与原校准raw均保留；5a97的virtualExportScopes诊断已有实际接受；成本工具只准追加它的精确第三源映射，不改metaPaths、fixtures、sampling、24limits或旧校准身份，实际工具绑定和最终成本验收另凭收据。原9260 native19、496d Make-only对应与各次EW都是历史，不能替代renderer修复后的完整重跑；最终成本、精确CI及正式三审只按下方实际收据记账。

原请求4s、数据库commit2s、既有lint worker2s与历史200ms预算不变。FirstChanges修复后的原历史守卫最慢整体73.39ms，FirstChanges三状态最大2.084/1.464/0.855ms；完整1000cs四state/mode返回2998行，物理界仍20000。不能将这些算法样本/targeted进程wall冒充每规定!race进程两轮均值×2.4≤原timeout一半，更不能替代五bootstrap组两轮及指定mainPair每片实际Test均值严格<450s。旧P3或e5e8自己的绿数不证明P4余量。

## 最终同源控制与门禁

当前候选事实基线为0b0。efad native19、成本100、91控/273过程和EW357，以及77只改Make时的明确窄对应，均保原source/WT/native mode与实际过程身份。77随后native19/session24201及精确CI41402585652 attempt1已接受，三fresh原报告已产生；它们不是0b0的新通过。0b0改变显示转义及两个测试文件；按02§3.4要求的完整native19、96控制/288过程及成本100，以本文最终记录和实际收据为准。0b0 minimum/postgen和独立fresh EW357已接受；C-Q1冷热补测、精确修复CI及原席核修分别见本文对应最终记录。

首次77 native session16674已actual exit1（terminal8e00b0）：8个inventory均0，外部helper硬编码旧SECOND导致严格比较拒绝，bootstrap/nonbootstrap测试stage未启动。外部唯一常量修正、离线核对及NEW runtime02绑定分别保原件；77后续完整19/session24201实际0已由BALANCE_NATIVE_ACCEPTED接受，不以这次后来绿抹掉首败。0b0的新完整19须保自己的实际终态，不借77结果替代。

### 最终96条控制：原91＋两项修复回归＋三席原控制

0b0最终源96条/288实际业务过程已ROOT接受：前86与后10保两控制器身份，各条0/1/0；另保第87条因原3GiB预检拒绝、未启动Go的记录；C-Q1 observer另账

最终修复源0b0df18c2ddd4b6779954458d37a617f66a42c5a的新96条控制已实际验收，收据`root-backtick-controls-all96-acceptance-01.json` SHA256 `a3c580602a06513e9c225981d70d33244eb8a83a16b56fd721da2664b433cc78`；独立原始合并验收`root-backtick-controls-all96-raw-01.json` SHA256 `ec1c52e1fc318801966ad0c47eb3b0d6e0faf9cb8bec17d085a088dba1aa29c3`。原91条、两项B-I1 domain/bootstrap回归、三席原故障共96条，全部相同argv三腿0/1/0，共288个实际业务过程；不是96种无重叠故障。A的outerctx控制与旧V16不同，B的SQL源加generated/完整top与旧D01不同，C原末跳控制逻辑重复E01，但保原席命令与注释单独重跑。

原controller session53084在实际terminal ce4463以exit1结束：前86条/258过程已全部完成；第87条baseline在启动Go前被原3GiB资源预检拒绝（可用3210403840B低于3221225472B，child_started=false），没有业务失败或fault/restored阶段。原目录完整保留。将已完成P2/P3的183份历史原始日志逐字节校验后迁至T7、原路径保可读链接，释放1778594155B；P4输入/原证据未改。原86条先由冻结verifier独立复核、ROOT再核真实终态和完整源恢复后保留。

新tail controller session42592在实际terminal302381以exit0完成87–96的30个新过程。原96行控制定义与源码快照字节不变，runner和独立tail verifier的原有非main函数全部保留，只调整尾段编排/实际来源绑定；继续使用原heavy mkdir锁、T7/3GiB检查、各测试timeout与业务首败规则。tail raw verifier session2547/terminalbf1baa exit0，aggregate实际dad0d6 exit0，ROOT实际cabc2d复读故障诊断、确认同源与恢复；合计258+30=288，不把那次未启动测试的资源拒绝算入。

原始JSON/verbose帧、非空RUN与终态、业务断言、备份与字节/native mode恢复均按各自冻结worktree验证。ROOT工作树3387源文件/embed252与控制树全部内容及Git executable位一致；76个生成文件在两个worktree之间为0644/0664的既有组写权限差异，各自完整native mode均与本树冻结快照一致，不误称跨树原生mode相同。三席旧77实验及作者87有效腿/93尝试仍按旧source单列；C-Q1缓存observer控制保持独立。

本轮96条不是96个全新或互不重叠行为：原91保持完整，新增B-I1 domain与bootstrap两条真实回归控制，再保留三席原故障/原argv三条。A保留RR但把GenerateInSnapshot的snap改外层ctx，与旧V16直接绕开WithinSnapshot不同；B同步改源SQL和generated的typed投影并跑完整TestDirectoryIndex，与旧D01只改generated/窄leaf不同；C去Contributors的逻辑与旧E01重复，但保原席注释及JSON/-p=1命令。三条均不冒称和旧91完全等价，映射见BACKTICK_CONTROL_MAPPING。原席77的既有0/1/0保77身份；本轮0b0的96×3=288实际过程及逐条核验结果见本文最终控制记录。C-Q1外部观察器反控另属补测，不并入96或凑旧样本。

### 最终完整native19与新race用时

0b0最终源新完整native19已ROOT接受：8项inventory、五bootstrap分片双轮及全nonbootstrap race；五片process均值341.073/271.625/292.684/297.700/250.199s，均严格<450s；指定main CI pair另验

0b0df18c2ddd4b6779954458d37a617f66a42c5a / d6614eb5ef0a03ebe5aa2c2f3091b65f8541d296 的新完整native19由session85179实际完成（terminal eec72b，exit0），ROOT以实际b67246验收；收据 `root-backtick-native-acceptance-01.json` SHA256 `003eda54c9701eeb2aeec8cd1e7c2909b637c6be2366b151d8ec3dcd564407a6`，completion SHA256 `7d9251fb36cb2c23660fcb0382c5750fe63b17f36be8d5a769cfcc230b42582f`。8项inventory、10条bootstrap命令和全部nonbootstrap race均保各自完整raw/timer/实际终态；本轮新加的11个反引号父/子节点共18次race观察均已核，所有旧节点义务不变。完整源3387项及embed252项的bytes/native mode、tools/environment与清洁Git状态在验收时一致。

五分片两轮process秒数为335.346/346.800、266.729/276.520、282.295/303.074、293.504/301.896、249.654/250.743；均值341.073/271.625/292.684/297.700/250.199s，均严格小于450s。package与top interval分别另记，不替代完整process。这是本机真实两轮；将来指定两次main CI的各片Test步骤均值仍须独立验收。77 session24201及首次helper拒绝保持历史对象，不作为本轮通过依据。

### 最终fresh build/EW

旧5a97 session31086、efad session96761、77 session80942各自fresh build/EW357单过保原身份；其各次接受不能互相重标。0b0的BACKTICK_EW_ACCEPTED实际session14866、terminal1912e3、exit0、171.1545584979467s，357单过且0 skip/retry/flaky，一次fresh build、全部required roles和owned清理已获ROOT接受。当前仍只Chromium；最终EW槽及其它门禁分别依真实收据填入。

0b0 fresh build/EW session14866已ROOT接受357单过、0skip/retry/flaky，实际171.155s；仅Chromium，S3至少三browser仍待执行

0b0最终源fresh build/EW session14866实际terminal1912e3、exit0，完整命令171.1545584979467s；ROOT35b70d接受357单过、0skip、0retry、0flaky。收据 `root-backtick-ew-acceptance-01.json` SHA256 `c92753712d2af1764a1529958fcda7bf249ee9f1c48f3d80a41eaaa6a3ac2f71` 绑定单次fresh build、完整源/runtime、所有必需角色与自有清理；原九个in-body slow annotation保留并与历史逐项一致。LW10真实REST/两MCP scope及ZIP、LW13两模式/REST/两MCP scope和实际served web均有真实通过身份。77、efad、5a97及更早356/1失败各保原身份，不重标或抹去。该结果仅Chromium；S3网页变更与至少三browser仍未实施。

## 门禁、CI与独立审查分账

| 材料 | 实际范围 |
| --- | --- |
| 四实施提交minimum/postgen | 8/10/10/10条与各自postgen接受，每份保原dirty/commit身份。 |
| 9260 L1、496d Markdown | 各10项minimum分别绑定c112+一行dirty、9260+Make dirty；各postgen/EW与当时native/cost对应保历史。 |
| b01 CI-only | 最终1024输入未另跑本地minimum；保留496d+4096初稿的10项真实结果，后仅preview阈值和workflow注释改为1024，另有精确源码对应及传输验证。不是把4096轮重标为1024轮。 |
| 9c display | minimum session42233在b01+两文件dirty实际0；9c postgen session78192实际0、mode差异0。 |
| 5a97 scope诊断 | minimum session40209在9c+测试dirty实际0；5a97 postgen session61999实际0、mode差异0。十二提交不写成同源十二次门禁。 |
| efad generic pool | minimum session63393实际5a97+index_cost_test.go dirty，十项actual0；efad postgen session75655 actual0；commit-source对应保持3387源码/252embed bytes/native mode。各次minimum保各自实际输入。 |
| 77 bootstrap balance | minimum session3580实际efad+Makefile dirty，十项exit0；77 postgen session83934 exit0、0mode变化。BALANCE_COMMIT_CORRESPONDENCE仅接受3387源码/252embed bytes/native mode的minimum→commit对应，不是旧成本/控制沿用接受。 |
| 最终96控制 | 0b0最终源96条/288实际业务过程已ROOT接受：前86与后10保两控制器身份，各条0/1/0；另保第87条因原3GiB预检拒绝、未启动Go的记录；C-Q1 observer另账；原91＋两项B-I1回归＋三席原故障共96，需0b0的288实际过程；efad/77接受仅保历史。 |
| 最终完整native19 | 0b0最终源新完整native19已ROOT接受：8项inventory、五bootstrap分片双轮及全nonbootstrap race；五片process均值341.073/271.625/292.684/297.700/250.199s，均严格<450s；指定main CI pair另验；77首次helper拒绝与后来完整19通过分列，0b0自身五片两轮及全部nonbootstrap race的结果见本行最终事实。 |
| 最终完整成本 | 0b0 fresh完整6targets/50recipe双轮100过程、1333 recipe-local identities及6case/24固定guards已ROOT接受；各pair原容量均满足，最大占半界65.443%；原首轮100容量失败保留，C-Q1独立补测另账；0b0需完整50recipe/100过程，不沿用efad→77窄对应或旧prior64；C-Q1独立补测另账。 |
| 最终fresh EW | 0b0 fresh build/EW session14866已ROOT接受357单过、0skip/retry/flaky，实际171.155s；仅Chromium，S3至少三browser仍待执行；0b0新fresh EW357已接受；77、efad和5a97保原收据。 |
| 最终精确代码CI | 精确0b0 CI41454798547 attempt1已13jobs/38必需steps成功；10份完整server日志及6case/24固定guards通过，web182/3926、EW357单过；guards job1707/1800s，branch五片357/312/399/400/321s，指定main pair另验；修复后精确代码CI结果见本行最终事实；77精确CI41402585652 attempt1仅为修复前历史。 |
| fresh三审及原席核对 | 77初审A无发现、B有B-I1 Important、C有C-Q1；精确0b0第一次原A/B/C核修均通过，B-I1与C-Q1关闭，新增Important/Minor均0；0b0修复反引号并完成新完整门禁、96条288过程与精确CI；独立C-Q1冷暖补测及等待修正已核实，原B/C分别关闭两项，原A无回归 |
| 文档与集成 | 原a1f成功、fd两失败和b4 glob失败保持，878/f689测试修复与代码CI接受保原身份；修复文档`2fc103dd`及no-ff `97c9ad8e`的精确CI已接受。本record提交后检查/自身CI与新指定两main配对待验，见[修复合并记录](#修复文档与no-ff合并记录)。 |

### 旧9260 native与旧EW历史

原native五片本地两次数据如下；这些是native process/Go package数，不能当作main CI Test时长。每片process均值实际<450s，指定main pair的每片Test均值仍须真实验收。

| 片 | 原9260 process两次(s) | process均值(s) | Go package均值(s) |
| --- | --- | --- | --- |
| 1 | 332.127700/318.736668 | 325.432184 | 319.939000 |
| 2 | 199.201229/189.255666 | 194.228448 | 189.177000 |
| 3 | 327.195992/306.876699 | 317.036345 | 311.967000 |
| 4 | 293.544367/272.394272 | 282.969319 | 276.512000 |
| 5 | 226.776014/226.209311 | 226.492663 | 221.878500 |

16个新race top的run→terminal及GoElapsed原件见NATIVE19_ACCEPTED；新八项inventory为499race/531nonrace/116包、168/39/117/123/52，库存不是测试PASS。当时ROOT源对应接受原native与旧64cost保留9260身份，不声称496d重跑了它们；更不将其沿用为最终修复源的完整重跑。

首轮完整EW的原生child为 `/usr/bin/make -j1 -f Makefile -f .../observe-built.mk VERSION=0.1.0-dev e2e`，实际exit2、wall189.055739257s；完整argv/cwd与UTC原件见 `EW_FIRST_PROCESS`。Playwright报告357 cases，356 expected/1 unexpected/0 skipped/0 flaky；唯一失败是 `links/l1-links.spec.ts:181` 的Ctrl/Meta新tab首个Target heading可见性断言，retry0、10s断言超时，call log两次等待navigation完成。本稿保留JSON首败及独立只读诊断，不据trace线索臆断底层CDP原因，也不把356项改写成完整EW通过。ROOT会话52760已收实际exit1，并已核源/runtime与owned清理；独立只读诊断已定位到Playwright1.63在读取DOM前的navigation precheck阻塞，底层pendingDocument来源尚未证明。原case唯一独立重放actual0/3.748736960s不覆盖首败；一行兼容修复9260及10项minimum已完成，9260的新fresh build完整Chromium EW已由ROOT接受：session2537/native实际exit0，357 PASS、0 retry/0 skipped/0 flaky，原生进程176.65847533475608s；LW10/LW13在该完整轮内通过。原首轮356/1与原L1独立重放0保留，不用重放或后续绿覆盖首败。 EW接受过程还保留一次ROOT stdin审计exit1：工具曾误要求没有runtime annotations，而9个原有in-body slow annotations与首轮报告完全相同。核对后只纠正工具期待；未改产品、annotation、timeout或retry，也没有因此重跑EW。 首轮原件SHA与错误全文保留在step-doc-preparation-01/ew-first-failure-observation.json；新增接受与中止证据见上方固定原件索引。

Markdown成本首次ROOT只读audit误把observer timer argv与child Go argv直接相比而exit1，后来分别核exact timer前缀与actual Go命令；未改产品/raw/helper或借重跑修记录。该次审计失败与后续核对保留各自实际退出及原始身份。

### 496d失败与b01日志传输修复（历史）

496d CI41349063534 attempt1最终cancelled：guards触发原30m job最大时限，10个其它job成功，e2e/image跳过未跑。最终已消费日志没有export terminal；不得按算法样本或没有Go timeout字样推断该轮通过。独立诊断实证无界约1.15MB事件行经过runner控制台消费出现严重积压；SQL+Scan与独立EXPLAIN耗时不支持把日志时间差说成SQL分钟级执行，具体matcher/masking耗时未profile、仍不作确定归因。

b01只改CI日志传输：前置tee保存原raw，有界1KiB preview供console/首失败注解，always上传；Go选择器、SQL/args/EXPLAIN、夹具、算法与Go/job timeout均不变。4096初稿会挤掉36k注解中的FAIL/panic，实际反例后收窄至1024；formatter/IO失败保持非零，但断管或硬job超时不保证raw完整，不能把always理解为无条件保证。

CI41353659599 attempt1精确b01已13 job成功，10份server日志上传/下载完整核验。原五bootstrap Test秒数为379/288/469/400/281；只是该分支run，不是指定main pair，也不保证最终每片余量。原件CI_B01_ACCEPTED/CI_B01_RAW_ACCEPTED还只将10个guard事件归到五个virtual bootstrap case，不外推本地完整成本或最终5a97。该CI-only产品字节对应只对b01成立；9c renderer变化后全部规定门禁必须在最终源实跑。

### 5a97代码CI失败与efad测试pool修复（历史）

CI_5A97_FAILED记录精确5a97的CI41370811260 attempt1为failure：只有server(costs)失败，generic过滤IndexPages中位153.380458ms超过原150ms；其余10job成功，e2e/image跳过，不能标green或释放正式三审。b01的绿色仍只属于b01。

GENERIC_POOL_REPAIR记录的修复在server/internal/modules/linking/adapter/postgres/index_cost_test.go:199复制pool.Config；原pool在:126用production NewPool且MaxConns=2，platform/postgres/pool.go:34–35设置MaxConns/AfterConnect、:53关jit。新测试只在:200设force_generic_plan，并在:207–215核实际jit/mode；计时三样本循环仍在:222，原150ms仍在:50。生产代码没有改动。旧47.5–49.0ms等generic校准保留raw-pool历史，不重算／抬高预算。

5a97+dirty候选诊断分别核AfterConnect、force_generic与缺type index；accepted候选到最终文件仅非directive注释变化。第一次no-afterconnect的不同-run baseline仍只是额外历史，三项最终源控制不能复用此前10个诊断过程。缺索引时generic过滤16.390291ms通过150ms不再是确定捕获；完整IndexPages三阶段都超过原1.025s，TestIndexPlans另有12条非空索引名失败。此为测试环境一致性修复及历史说明修正，不新增产品决定或更改73–75。

efad的generic-pool/afterconnect、generic-pool/force-generic、generic-pool/type-index三项已按各自同命令baseline/单fault业务首败/restore实际完成，session38043/exit0由ROOT接受；原88控制session37025/exit0也已接受。GENERIC_CONTROLS_91_ACCEPTED汇总同efad产品内容的91控/273实际过程，各WT/source/mode身份保留，排除此前10个诊断过程。旧binding.unreleased仍是原准备快照，不改其原件。ROOT已另明确接受精确77的未受影响控制窄对应；原91/273过程仍属efad，本稿不发布执行许可。

### 77仅Make的bootstrap容量移界

BALANCE_PLAN与BALANCE_APPLICATION分别保留方案和实际应用身份；固定77提交只改Makefile:240，在SECOND外层分组内加入`|VirtualExport$$`（Make展开后为整top尾锚）。完整TestVirtualExport从第三份到第二份，其子测试仍随top；FIRST/THIRD/FIFTH、排除规则、测试名字及原race/count1、Go15m/job25m均不变。这是既有容量规则的落实，不新增产品决定，73–75不变。

b01/5a97第三份Test469/479s只是旧分支CI样本，不构成指定main pair。方案依据旧499 race tops/531 nonrace tops核静态唯一移项：race五份168/39/117/123/52→168/40/116/123/52，nonrace177/46/120/136/52→177/47/119/136/52；该静态方案自身不证明77已完成编译或运行；后续完整native19/五片两轮的实际结果见本文最终native记录。minimum session3580的实际efad+Make dirty输入、77 postgen session83934，以及两者3387源码/252embed bytes/native mode的ROOT对应已接受，均不替代最终Step门禁。

77新fresh EW、后来完整native19及精确CI41402585652 attempt1已实际接受。ROOT明确接受未受影响efad成本100/控制91供精确77沿用，原100/273过程仍属efad，不能伪称77重跑100/273；独立WT原76个mode差异保留。该窄对应不适用于0b0生产修复。77三fresh原报告A零发现、B-I1 Important与C-Q1的发现及后续处置见本文正式审查记录，最终修复对象另由事实槽绑定。

### 77精确分支CI（修复前历史）

CI41402585652 attempt1精确77，13作业/38必需步骤全成功，10份server日志及六算法case/12状态/36样本/24固定限额已由ROOT核验。五bootstrap Test为389/252/401/387/277s；guards Test1698s、job1720s，原1800s余80s，不能称余量充足或代替指定两main配对。该CI并未验证后续0b0，也没有证明seed/检查后的no_statistics为冷缓存；C-Q1独立实测与原席核修结论见本文发现记录。

### 最终repair精确代码CI原件

最终修复0b0的精确[CI41454798547 attempt1](https://github.com/open-nerve/NerveWiki/actions/runs/41454798547)已实际完成success，ROOT snapshot session49184/terminald4a5d9及verifier实际98d979均exit0；`root-ci-backtick-acceptance-01.json` SHA256 `84fd80f61f6502b317e36dd8e44e4ed6bb6301edd4b50f025acecdb53e0316c9`核13 jobs/38必需steps。五bootstrap Test为357/312/399/400/321s，仅是分支本轮样本；指定两个main的各片均值仍在合并后另验。guards Test1685s、job1707s，原1800s job界保留，实际余量93s（5.17%）；这不是对未来波动的保证。

10份server原始artifact已由实际18888/2c250e exit0下载，API ZIP digest与原raw→1KiB preview逐字节一致；收据`ci-backtick-artifact-download-01/receipt.json` SHA256 `8515356fec9639b4700b6bb64fbda7f009ab172f1c2045a9e2e6a075ec5fd930`。ROOT以两份完整成本raw重算6case/12states/36samples的中位数，24个固定限值完全一致且通过；实际3d864f exit0，`root-ci-backtick-cost-events-acceptance-01.json` SHA256 `f404b10e81e928926b73b44aa677c8e28f9f59610b8ead6221b0f861c22ab71a`。generic池实际jit=off、force_generic_plan，IndexPages三样本26.017640/26.048388/26.452553ms、中位26.048388ms低于原150ms，top实际PASS；不是从预览长行或标签推断。

额外web/e2e/image原始job日志由63095/bba569 exit0保存，ROOT10c359核web182 files/3926 tests、Chromium357个唯一序号单过及四条LW10/LW13故事，镜像烟测亦成功；`root-ci-backtick-extra-logs-acceptance-01.json` SHA256 `90396e40544e40271bdef4fa5ddeaa4f5e18de3094539b1b3d4d8ee9e4343f50`。日志下载与核验不是新的产品执行；S3三browser仍待做。旧496d/b01/5a97/77的失败或成功各自保原HEAD/run身份，不用本轮绿色重写历史，也不替代文档/main/record各自CI。

### 三位fresh代码审查与原席核对

77三席原报告和各席真实0/1/0已分别由FORMAL_AB_VERIFIED、FORMAL_C_VERIFIED接受。A零发现；B唯一B-I1 Important；C无Important/Minor/Nit，唯一C-Q1为缺失的冷/暖实测证据。此处仅归属77原报告；0b0原席核修结论见本节最终记录。

| 发现 | 已采取处置 | 验收要求/证据落点 |
| --- | --- | --- |
| B-I1：反引号破坏显示/完整链接身份 | ROOT在77＋测试dirty观察domain和真实export/两scope read业务red，补fileLabel的反引号转义，domain边界及六bootstrap分支green后提交0b0；minimum/postgen已接受。 | 新96控制、完整native/cost/EW、精确修复CI与B原席核修；不能以定向green代替。 |
| C-Q1：矩阵cold/warm未有明确PG缓存实测 | ROOT接受独立单测试文件overlay协议：自有PG18.6的九算法表/index/TOAST闭包，逐对驱逐与全零驻留证明，之后同操作warm；保4s、固定guards、100k＋200k/1000cs、每state三对和两独立轮。 | 验收需实际四正常过程、独立observer反控及原始驻留/实际请求/ZIP/样本/容量；实测与C原席核修结论见本节最终记录。 |

C-Q1只明确PG shared_buffers目标关系冷态；OS文件缓存、backend/catalog/plan保留，不宣称物理磁盘冷态或将no_statistics/Go GC当冷。whole export从冷边界开始，原job前段会预热contributor，不能另称contributor独立冷态。补测保持原两条CI recipe及循环次数，不把guards CI加倍，不延迟到M12；EXPLAIN不得插在cold与warm之间。观察器反控应真实拒绝未驱逐而标cold，编译/权限/参数/环境失败不算kill。此为验证协议处置，写在发现记录，不新增第76个产品决定。原73–75及4/3/3编排不变。

77三席原报告完整保留；0b0精确修复CI见本文对应最终记录，原席核修结论由本节事实槽绑定，不把准备报告当正式核修。

正式代码初审的三个原席均在精确77c5fc474a95614a7984d6532a4f14281fdcb048/tree4cd7e8743e053ec88773b8a92fb1bbacfceedda1，各自独立worktree开始和恢复后干净。A报告Important/Minor/Nit/Q均0；B报告唯一B-I1 Important：合法反引号标题让真实Goldmark把label/destination吞成代码片段，六条ZIP/两MCP scope index/log缺真实链接；C无Important/Minor/Nit，唯一C-Q1要求明确并补足cold/warm缓存状态证据。原77的三报告不等于修复0b0已经核对，核修结果另见下节。

ROOT原始报告验收`root-formal-ab-reports-verification-77-01.json` SHA256 `f42190ea07fa2cc041bce9beba8b7fe94b11e7edba5a3acef6fbfcc4fa626a82`逐项核A51、B96原件及恢复；`root-formal-c-report-verification-77-01.json` SHA256 `265c4aa1d7fdc67d2d836b0b9832d0af7a2d1222138fb39962285a98c0f5de8e`核C36自产原件及原始控制。A报告SHA `8edf8be0408a61e46001b60fd7ec9e0e56d81a68979651cfad19b44c9597b740`、B报告`e9210f1cd2f3ec04d0afd8f7e163c7067ab78b7b8190393621af0192d27ec544`、C报告`88858e25a361c92d17d66bfac14ea1aa06359cd27fd7674be987c2d37b628770`。

A自己的真实RR outerctx故障三腿为sessions66380/57952/77469；B的SQL源加generated typed投影三腿79195/14446/31809；C的贡献者末跳15836/30416/1043，均实际0/1/0、从备份复原native bytes/modes。B另外真实复现六条反引号业务失败并有ordinary正控；那是原77诊断，不冒充0b0修复绿。C对98个产品/测试/Make minimum与其余完整门禁/成本/控制为独立读取原件，只有E01为本席新跑；b01最终1024输入未另跑local minimum，额外dirty4096初稿十项另记。各席scope、未验证部分和过度设计判断保持原报告，S2/S3/P5不提前计入S1。

B-I1已由原B席在精确0b0关闭。生产只把反引号加入既有显示转义集合，完整destination与最终256/500B界保持；两个新回归文件消费真实Goldmark、ZIP及两MCP scope。ROOT修复前在77加回归取得domain六处/组合根八个失败事件，修复后通过；minimum实际输入仍是77+三文件dirty，由提交对应收据证明与0b0一致，不重标为提交后执行。最终0b0的新完整native19、EW357、100成本、96条控制与精确CI已分别记录。

C-Q1补测的ROOT收据`root-c-q1-final-evidence-acceptance-01.json` SHA256 `700284599704cd4a62e14c22a029030dab570fda7bc448d5d227b8b7ff0c67c2`，原C席已独立核原件并关闭。明确测同私有PG中九算法表的67个目标关系及所有fork的shared_buffers驻留；OS/backend/catalog/plan缓存保留。每个cold样本先证明正驻留，实际驱逐后全0且skipped=0，再由真实RR观察miss；随后同业务warm操作观察驻留与hit，不声称全工作集常驻或物理磁盘冷读。export只证明whole-start冷态，contributor内层不冒称独立冷态。source原3387文件、真实SDK/ZIP、4s请求、原recipe/Go timeout、24固定预算均保留，外部observer未加到原CI循环。

五实际阶段保持各自身份：旧runtime01 read baseline29364/59f056 exit0、observer fault38598/d9d652 exit1、restored5106/b8beca exit0；新export runtime02两次10189/ef75ab与50833/d122f6均exit0。四正常过程有72 RR、180驻留观察、96 cache metrics、12 directory samples、36 guards和24个不同export job IDs；read双进程×2.4为155.24570217300206s，export为154.32644707337022s，均低于原300s半界。observer只取消实际eviction而保持oracle，真实四处cold前置业务拒绝，恢复后通过；没有把故障样本计作正常数据。原export52178/929a48失败及未知due row身份保留，3行外部awaitExports修正仅作用新export两轮，旧三个read不改标为新overlay执行。

**第一次原席修复核对（精确0b0）**：

- 原A报告`formal-fix-check-a-0b0-01/fix-check-report.txt` SHA256 `d7cadb8c572a9287503d8b510a136f5ac0d4610939677cb8c1018ea9273a26a0`，Important/Minor/Nit/Q均0。A自己重跑outerctx故障sessions69484/36216/76355实际0/1/0，首败为真实重命名窗口的混合快照；另18个真库读/导出叶子与15个domain显示叶子通过。ROOT核61原件、5阶段repair三文件始终保持0b0、恢复原bytes/0664及clean；接受`root-formal-a-fix-check-0b0-acceptance-01.json` SHA256 `3e558997df762db7833d213927aaf18dbf487994d198dbb61242be5551a64199`。
- 原B报告`formal-review-b-fix-check-0b0-01/fix-check-report.md` SHA256 `88553fd882a6c887c97a8c2ecf2f85d6cb2d7cf074234b6dafb1ac5e2c27050c`，关闭B-I1，新增Important/Minor/Nit/Q均0。其真实PG/serve/ZIP三腿73675/37437/23373实际0/1/0，仅移除12B反引号条件时六份文档的完整href业务断言失败。HEAD domain与独立诊断68492/65030也实际通过：329B真实路径、415B生成器目的地、混合UTF-8/escape最终256/500B界及302B整文预算。ROOT核84原件并重算六份HTML decoded链接，产品与原0664已恢复、诊断删除；Git index仅stat cache原始hash变化，3387项stage0 mode/blob/path逐项等于HEAD，不声称index原始字节未变。接受`root-formal-b-fix-check-0b0-acceptance-01.json` SHA256 `ca566d0ddef3d6c43d8f900de382d4206e31b3572334d5f085da90dffb61dc6f`。
- 原C报告`formal-review-c-fix-0b0-01/report.txt` SHA256 `ad8f7746842372b9c5041e87cae14e465373cbd770193b2236f09388dc2eae25`，关闭上述PG shared_buffers范围内的C-Q1，新增Important/Minor/Nit均0。C以独立源码/原始结果重算五阶段、完整100成本、96条288过程、native/minimum/EW和精确13job CI；本轮未再次启动Go/PG，不冒称C重跑。ROOT核其15自产原件和1480去重外部pins及数值对应；接受`root-formal-c-fix-check-0b0-acceptance-01.json` SHA256 `73e5d9c31f7fc397d6f714e90d3c80eba76b49b8bd76716d5a2e79e59bfd0d5c`。guards原job上限余量93s、首100容量失败、86+10控制器身份、原export未知due row继续明确保留。

三份原报告与核修报告分开保留，不把本轮核修写成fresh初审；全部原Important/Minor已关闭，无新open Important/Minor。后续文档CI、main合并/record与指定两main配对仍独立验收。

C-Q1首个export-round-1（原runtime01，session52178/terminal929a48/exit1，完整process39.51889389380813s）是实际失败，不计入两轮正常export：no_statistics和analyzed的首个cold ZIP均通过，但两次warm之前的quiet均以DueJobs=1/holders=0拒绝；该轮没有完整三样本或最终guard。独立诊断确认产品transfer成功提交与River v0.47批量completed写回存在正常时间差，且外部观察器缺少自然完成屏障；原日志未留due行id/kind/state，所以不追认那一行必然是哪个任务。

外部overlay03仅在export双phase的cache.before前加既有awaitExports调用及两行注释，共3行/165B；等待沿原60s jobWait/50ms，只读river_job，保原quiet全零断言，计入完整process且在算法sampler之外。除export测试函数这一处外所有字节相同；读取0/1/0保原runtime01/source，受影响的两个完整export使用独立runtime02，最终原五阶段raw摘要重读全部五个真实过程并重算两对容量，不把旧read改标为overlay03运行。root-scope-proof的SHA256为`b2ac9a5664796203a6b8e7e6ebece6421f4108a4c15ab4fd555f5655a0cb02e2`；独立静态审查Important/Minor均0，manifest为`ec034ec0dff908eee96b4c6bbf0c7b84fad1282407ab6aa963fcd674024fcf6e`。实际新export结果与最终C-Q1接受只以上方记录为准；原失败、旧read身份和产品源码不变。


## 接受与推后

修复文档`2fc103dd`与no-ff `97c9ad8e`的精确CI已接受，见[修复合并记录](#修复文档与no-ff合并记录)。原a1f/fd配对因fd两失败未完成，b4 glob失败、986文档guards超时取消与f689正确夹具/代码CI接受保原身份；本record自身CI及新指定两main配对仍待实际接受，S1集成未闭环、S2未放行。性能原红与M12未解范围保留；S2/S3、P5及M10收尾未完成，M11未开始。

**以下保留Step文档前的接受原文及当时待验状态。**

S1代码、完整本地门禁、精确修复CI和三位原席第一次核修已接受；当前进入Step文档与主线集成，合并见00第12节；S2/S3仍待实施 ROOT已在原始门禁、精确CI与三份原席核修均复核后，接受最终0b0代码进入Step文档；收据`root-pre-document-acceptance-0b0-01.json` SHA256 `eda4a0f28844ccffb6b5757e632ab3bdb0ba36ed2f442d7aae24ad6edd26baad`。ROOT再次逐字节/native mode核3387源文件与252项embed均等于本树已接受快照且Git干净；无open Important/Minor。本文自身commit/CI、no-ff/main/record及指定main pair尚须实际执行，不能提前写成已完成。

各旧epoch的87腿/93尝试、72校准及36正常targeted保原source/raw；9260→496d及efad→77窄对应仅为各自历史。最终代码0b0df18c2ddd4b6779954458d37a617f66a42c5a / d6614eb5ef0a03ebe5aa2c2f3091b65f8541d296的96条控制、完整native19/成本100/fresh EW、C-Q1独立补测、精确CI、三席与原席核修均以上节实际收据为准；不能将77的接受重标到0b0。文档自身CI与no-ff/main/record配对仍待；P4仅S1，S2 ingest/00053与S3UI/至少三browser未完成。P5、M10收尾、M11不提前关闭，M7/M9仍open，raw 2/AGENTS 2限制保留。

下列旧逐腿附录保留原始记录，仅六处诊断摘录增加行内代码定界符；摘录文字及原始日志未改。“最终纳入”仅表示各实施作者当时epoch的选择，不能解释为0b0或未来修复已执行；E02的原泛化错误也不改写成新实际cause。当前96条以“最终96条控制：原91＋两项修复回归＋三席原控制”一节对应真实源码及每次0/1/0，C-Q1外部实验另账。

## 文档与合并记录

本次补记于2026-11-03 00:26:53.492156（Asia/Shanghai）写入。产品、各轮minimum/postgen、三席初审与原席核修均保持原输入身份；决定73–75不变。同次补记将96条控制的内部槽名改为现有节名，原附录及数字不变。

| 阶段 | 实际身份与接受范围 |
|---|---|
| Step文档 | `2ec19be8793338c59a5f4014fe5812bd98831341` / tree `76dba8afa44282b6cc9313fe0ffeea74e14d27c6`，parent `0b0df18c2ddd4b6779954458d37a617f66a42c5a`；提交时间2026-11-02 23:12:26（Asia/Shanghai）。 |
| 文档分支CI | [CI41469628154 attempt1](https://github.com/open-nerve/NerveWiki/actions/runs/41469628154/attempts/1)，精确head `2ec19be8793338c59a5f4014fe5812bd98831341` / `m10-p4-s1` / push / completed/success，全部13个必需jobs及38个必需steps成功；完成于2026-11-02 23:47:59，ROOT接受于2026-11-02 23:50:53.441324（均为Asia/Shanghai）。分支用时不进入main配对。 |
| no-ff合并 | `a1f21e75804eafd3739558612933ae0c90b44d33` / tree `76dba8afa44282b6cc9313fe0ffeea74e14d27c6`；有序父`e5e8a0b3cffcc473aba9085b0965e3f2193f16dd` / `2ec19be8793338c59a5f4014fe5812bd98831341`；合并时间2026-11-02 23:53:48（Asia/Shanghai），与已接受Step文档同树。 |
| 精确main CI | [CI41476185798 attempt1](https://github.com/open-nerve/NerveWiki/actions/runs/41476185798/attempts/1)，精确head `a1f21e75804eafd3739558612933ae0c90b44d33` / `main` / push / completed/success，全部13个必需jobs及38个必需steps成功；完成于2026-11-03 00:23:02，ROOT接受于2026-11-03 00:25:43.834082（均为Asia/Shanghai）。五bootstrap分片实际**Test步骤**依第1至5片为306.0/332.0/392.0/409.0/318.0s，仅是指定配对的merge侧样本。 |
| 此前CI尝试 | 文档在本次绑定head上无此前CI尝试。merge main在本次绑定head上无此前CI尝试。各次尝试保留原run/head/attempt与结论，只采用上列两个完整成功attempt，不拼接片段。 |
| 本record及后验 | 本记录写入时，自身SHA、提交后检查及精确main CI尚待实际产生；指定两main配对仍待ROOT接受。不为回填自身SHA/CI另建record-of-record；后续事实保存在树外回执，并随下一次既定文档自然转录。 |

指定配对只取上列no-ff merge与本record两个完整接受的main attempt；逐片计算`mean_i=(merge_Test_i+record_Test_i)/2`，五个均值均须**严格<450s**。不能以分支CI、本地process/package用时、job wall、部分成功片或跨attempt拼接替代。record自身完整CI与该配对实际接受前，不声称S1集成闭环；S2/S3及00053、S3至少三browser、P5与M10收尾仍待，M11未开始。

## main record CI失败与会话fixture修复

本节补记于2026-11-03（Asia/Shanghai），事实截点为会话测试修复`878381485a5c3d1209f0afdce48ebb4865e85e35`的本地接受、提交后gen-check及第二轮本地计划观测；精确878代码CI及其完整日志/成本凭证已由ROOT接受；本文档自身CI和集成仍待验。上节`a1f21e75`的main CI成功仍成立；随后record `fd2607ab9fec4cade43fb28622873607891ae58f`的[CI41482903019 attempt1](https://github.com/open-nerve/NerveWiki/actions/runs/41482903019/attempts/1)为completed/failure，不能接受原指定两main配对。S1集成尚未闭环，S2未放行；S2/S3未实施。

### 两项失败与修复对象

| 对象 | 已观察事实与处置边界 |
|---|---|
| fd main record | tree `a3d39e2bad8e585a2e73def491c54256eabcc977`、parent `a1f21e75804eafd3739558612933ae0c90b44d33`；仅三份Markdown的36+/11-。该head的attempt1最终9 jobs成功、bootstrap-2和guards失败、e2e/image跳过；十份server原始日志已保存，下载成功不等于门禁接受。 |
| bootstrap-2 | `TestSourceSessionVetoRollsBackExpiredCleanup`报告durable source state改变、expired session消失；没有HTTP状态不符。原始CI未记录删除者/事务身份，不能据症状确定历史调度。 |
| guards | `TestARevertAtTheUnitsBoundTakesItsTime`在10000 nodes/400000 history的原夹具上测得12.299969446s，超过原12s限值；相邻many-tops为2.845851567s并通过。会话fixture修复不解释或关闭这项性能失败。 |
| 会话测试提交 | 分支`m10-session-veto-startup`的`878381485a5c3d1209f0afdce48ebb4865e85e35` / tree `b6148cf5c161b6a8f96350aac29256ff2dc8735e`，parent为失败的fd main；仅`server/internal/bootstrap/source_guard_test.go` +10/-3。已普通push；[精确代码CI41494579188 attempt1](https://github.com/open-nerve/NerveWiki/actions/runs/41494579188/attempts/1)已按13个必需jobs/38个必需steps全部成功接受；仅绑定该878代码分支。 |

精确run/head/jobs与十份日志的原件见失败CI归档回执。fd的原minimum/postgen成功保原身份，不能覆盖其后真实CI失败；失败attempt不进入成功配对，也不拼接其它attempt的片段。

### fixture竞争、永久断言与真实控制

清理器设置`RunOnStart=true`，测试的一小时周期并不禁用首次清理。原fixture先人工过期，再快照/发请求，允许首次worker独立删除并提交。修复在人工过期**之前**调用已有`awaitJob`，等真实cleanup completed；再要求UPDATE恰好影响1行、请求前该session恰有1条expired且unended记录。原409/`page.source_immutable`、全部session及node/content/notebook状态比较、精确expired行与后续`page.edit_session_ended`断言保留。

此屏障按本测试的独立数据库、单app、一次scheduler启动及原15m包期限成立；未关闭生产RunOnStart，未新增sleep、重试或延长期限。它不是跨scheduler重启或超过一小时运行的全局排他保证。生产事务路径未修改，临时SQL tracer、强制排序与变异均未提交。

| 实际运行 | 退出、观察与范围 |
|---|---|
| `baseline-original-01` | 原fd目标单次真实PG/race：exit0、1 RUN/PASS。这只是无干预样本，不能证明竞争不存在。 |
| `forced-startup-red-02` | exit1；基线永久源码未改，临时诊断控制真实SQL顺序：worker以autocommit删除1行，随后请求事务删除0行，再同请求连接rollback并返回409；原durable/expired两项oracle均红。它实证一种可造成原症状的因果路径，不证明历史CI恰好采用该调度。`red-01`因诊断误计identity同名SQL而未获接受，失败原件保留。 |
| `after-barrier-green-01` → `transaction-escape-red-01` → `transaction-restored-green-01` | 同一双目标selector、原`-count=1 -race -timeout 15m`，真实退出**0/1/0**，每轮两顶层均实际RUN；首绿→红仅把`DeleteExpiredSessionsOf`的`s.queries(ctx)`改成`gen.New(s.pool)`，首绿→恢复绿所有输入一致。**永久目标测试自身**在红轮报durable state与expired行失败，不只依赖临时诊断。红轮诊断观察到事务外提交后先Fatal，不能声称它走完HTTP/rollback链；两绿轮观察到请求事务删除1行后rollback。 |
| `related-cleanup-race-01` | exit0；已移除临时诊断，bootstrap/page postgres两包7个顶层、连同子测试11个节点实际RUN/PASS，包含empty/nonempty与move_first/open_first。不是skip、空选择器或编译失败代替业务控制。 |

上述原目标0、强制交错1、修复后0是不同输入/选择器的诊断经历；同源同命令的rollback变异三轮另按0/1/0记账。旧96条控制及各旧epoch的0/1/0保持原归属，不重标为878重跑。真实原件见startup控制与事务逃逸0/1/0回执。

测试小轮由独立审查者先作源码审查，封存后再复核运行原件，均为**I=0/M=0/N=0/Q=0**；审查者未执行Go/PG，也未用作者报告替代证据。ROOT复核其86个原件引用后接受此测试修复；这是该小轮的独立核对，不重写原S1三席结论或外推性能问题。

### 本地门禁、提交后检查与未闭合范围

| 阶段 | 实际输入与接受边界 |
|---|---|
| minimum8 | fd + 单文件测试修复dirty候选，3388源/252 embed；原八命令实际exit0。server short为8906 RUN、7373 PASS、1533 short SKIP；tools为27 RUN/PASS，旧terminal集合完整保留。SKIP和无tests包不计作PASS；不是878提交后的新执行。 |
| 完整bootstrap-2 | 同一测试候选，原Make完整分片argv、race/count1/15m实际exit0，package 259.211s、process 265.12997104320675s；原命令无`-v`，不补造逐测试RUN数。此本地分片不替代精确CI或指定main Test步骤配对。 |
| 提交 | 878仅提交测试修复，生产变异已恢复、临时诊断已移除；ROOT本地接受回执 SHA256 `23ecf965414ab211c2ad1089a27dda05d6853dd1d886177e1b8c8adc09c5dc0e`。 |
| postgen-01 | 启动方式导致PATH顺序与冻结值不同，前置检查拒绝，launcher exit1、child exit为null；没有启动generator。该原始失败保留，不计作gen-check执行或通过。 |
| postgen-02 | 在878上采用与冻结值一致的launcher，绑定的内部cwd仍为修复WT；完整`make -j1 gen-check`实际exit0。3388源/252 embed字节及Git执行位不变；76项固定generated output原生mode的0664→0644逐项登记，其它mode不豁免。提交后接受回执 SHA256 `e3751c762d9c1ac344c76f04a6a0c032f0a4387c26012189cf2cd7c8515c3c69`。 |

精确878 / `m10-session-veto-startup` / push / CI41494579188 attempt1已完整成功，由ROOT按13 jobs/38必需steps接受；十份server原始日志完整、六case/12 states/36 samples及24固定限值重算接受，见精确代码CI接受回执、十份原始日志接受回执及六case/24固定限值重算回执。五bootstrap实际**Test步骤**依第1至5片为385/397/390/287/307s；guards **job**为1470/1800s，实际余量330s。此为代码分支样本，不进入指定两main配对，也不证明原fd性能失败的历史原因；本文档自身CI与新main集成仍待验。

revert的原Make两目标无插桩本地单样本为6.179896086s/1.591799417s，实际exit0，保原12s/8s断言与Go期限；该回执明确不作为CI修复接受。另一次带临时`!race`观测覆盖的诊断为HTTP200、6.669028961s、1072事件/0 pending；离线汇总将RestoreNodes与TrashHistory列为主要观测区间。它包含客户端等待、调度及观测开销，不能当作PG纯执行时间、原生gate或历史CI解释。

第二轮`revert-plan-02`在878上实际exit0、两顶层PASS，原HTTP计时6.593024586s。ROOT核两条原SQL、原参数形状/模式、同PID92、各执行一次、连接/事务身份、设置恢复及前后源不变，按本地诊断观测回执 SHA256 `1f9525e92277e4cca5ec27ffd9164f6c0aa5729be4866e621c359e1fe1672508`接受；插桩仍不计作native gate。该计划中RestoreNodes两历史表各一次scan/hash join，更新输入189900/210100行，没有逐节点重复全表扫描；确有6328kB磁盘排序/temp使用，`TIMING OFF`不能分配各节点耗时。TrashHistory按10000个节点索引范围读到合计400000行；其只读语句也报告WAL，但原因未证实。不能相加父子累计buffers/WAL、将观测读块/WAL归因于历史CI，或据此断言无优化空间。现有证据不足以选定生产修复；large guard 12s、many-tops 8s、请求15s期限及夹具不变，CI当时资源状态仍未知。[M12性能移交第15项](../../M12-release/handoffs/M4-performance.md)保留具体观测与未解风险，status仍open。

性能计划另经独立离线复核（154项断言、43个输入pins），ROOT接受回执保留相同观察边界；wrapper调用点124→221的引用勘误单列，原报告和运行输入未改。不新增性能修复或历史CI归因结论。

原指定`a1f merge＋fd record`配对因fd两失败未完成，历史样本原样保留。ROOT已接受精确878代码CI；后续计划是文档分支CI接受后no-ff，再补实际merge记录；明确以**该次修复merge＋新record**的两个完整main attempt作新的指定配对，逐片实际Test均值须严格<450s，不跨attempt拼接。该no-ff及其记录尚未发生；本文档自身CI、新main/record CI及新配对均未在本节获接受。S1集成与原性能失败保持开放，S2/S3、P5和M10收尾不提前关闭；决定73–75及其它产品合同不变。

## b4文档CI失败与grep-glob fixture修复

本节补记于2026-11-03（Asia/Shanghai）。取证对象为文档提交`b4a031253c985b73d8f756d9ae7477421fb63e95`（tree `faaeb6687d2e9195fd6624dc323c98265d6de4bd`，parent 878）及其后仅`server/internal/modules/page/app/grep_cost_test.go`修改的候选；候选文件6021B、SHA256 `c05312271ad34d7ea2fbd57909fa7b76106a110cc541682820facbfb272fd0b3`。随后实际仅测试提交为`f689c839f2bbb8167db17b2b981da686b78faa1b` / tree `fe80ab4a49b20f8ce44194cac2131fcea1e1443d`，parent b4，唯一文件+34/-1且字节与该候选相同，见提交回执。原878会话修复的接受、a1f成功、fd两项失败及上节当时pending均保历史身份；[精确代码CI41510773390 attempt1](https://github.com/open-nerve/NerveWiki/actions/runs/41510773390/attempts/1)已按13 jobs/38必需steps接受；文档自身CI与集成仍待验。

### b4失败与独立结构证据

[文档CI41501780779 attempt1](https://github.com/open-nerve/NerveWiki/actions/runs/41501780779/attempts/1)的精确head为b4，`server (costs)` job125470814199的原`TestGrepOverTheGlobGuardTreeTakesItsTime`测得**1.252742361s > 1.25s**，超界2.742361ms；stop为glob、incomplete为true、next非空。原失败job日志的643–652行保留原recipe、RUN、测量、FAIL及Test步骤exit2；718–726行只是失败注解重印，不是第二个样本。whole-test 1.95s含构造，不与1.25s窗口比较；本次costs后续recipe未到达，不用旧绿补算执行。该失败阻断文档/main集成，不能用先前878成功覆盖。

旧`newGuardNodes`在每组inner loop把所有child的ParentID指向同一个反复赋值的`parent`，最终十个child都指向末child，包括其自身；110万构造量不等于110万可达。独立BFS直接读ID/ParentID，不调用生产treeOf/Glob.Walk；旧图red实际exit1：total/unique 1100000、roots 100000、self-parent 100000、reachable 100000、unreachable 1000000、max-depth 1。它证明测试夹具未满足原十万条十一层独立链义务，**不证明历史CI时间超界的原因**。

永久改动仅在测试文件：每次inner iteration复制`parentID := parent`再取址；原成本测试在计时前对即将传给Grep的同一个slice做全量校验，核1100000个非零唯一ID、每11条一组的根/前驱关系。任何结构错误立即终止，不另造一份fixture、不借生产遍历自证。没有生产代码、Make recipe、计时表达式或断言放宽；原1.25s、110万构造规模、1016B glob、MaxGlobWork=300000000以及其它预算不变。校验的set分配在计时外，仍可能影响GC/缓存，不能称零成本。

| 实际原件 | 结果与边界 |
|---|---|
| 永久校验的旧图反证 | `grep-glob-regression-red-01`是UUID API误用导致的build failed，不算业务red；修正诊断稿后的red-02实际RUN、exit1，在node 1的前驱关系处失败，未进入原计时。 |
| 修复后原生recipe | fixture-fixed-01在上述c053候选无overlay、原count=1/selector下exit0，原窗口827.240875ms、stop glob/0 pages，永久结构检查及原结果/时间断言PASS；仅为这次本机单样本。 |
| 修复后独立BFS | shape-fixed-01实际exit0；total/unique 1100000、roots 100000、self-parent 0、reachable 1100000、unreachable 0、max-depth 11，十一层各100000；前后候选源码相同。独立诊断与原生cost recipe分账，不冒充另一轮原生成本门禁。 |

原图未插桩单样本850.167959ms与原图profile的760.499458ms窗口/820ms CPU samples各保输入身份。profile只在fixture后、原start前开始，在took后停止；CPU采样与MemStats会干预执行，treeOf、match、sort及GC仅是该旧图本机窗口的热点线索，不能把CPU累计值相加为墙钟、当原生gate，或据此归因CI。原五次CI同case值901.438207ms–1.178970452s及b4红见有界历史原件复核；有限旧样本没有解释新红，也不能与修复后的正确图混作同一标定或挑选成成功配对。

### 接受边界与待绑定事实

f689的提交与ROOT接受的独立测试小轮核对已绑定：66个输入pins、I/M/N/Q均0；这不重写原S1三席结论。提交前minimum8的实际输入仍为b4+上述单文件dirty，session66607实际exit0，八命令全部完成；server short为8906 RUN/7373 PASS/1533 short SKIP，tools为27 RUN/PASS，skip/无tests不算PASS，不重标为f689提交后的新运行。

精确f689的提交后完整`make -j1 gen-check`实际exit0（session25184）；3388源/252 embed字节及Git执行位不变，native mode差异为0，见提交后gen-check接受回执。精确f689 / `m10-session-veto-startup` / push / [精确代码CI41510773390 attempt1](https://github.com/open-nerve/NerveWiki/actions/runs/41510773390/attempts/1)已由ROOT按13 jobs/38必需steps完整成功接受，见精确代码CI接受回执。五bootstrap实际**Test步骤**依第1至5片为467/310/395/308/256s；guards **job**为1139/1800s，实际余量661s。此为代码分支样本，不进入指定两main配对，也不证明原b4或fd时间失败的历史原因；十份server上传的ZIP digest、完整raw与重建preview均已核对，见十份CI原件；六case成本重算按原24固定限值接受12 states/36 samples。

正确fixture的原glob命令在该CI只执行一次：完整costs原件第263–268行依次为原`-count=1 -v`单测试命令、RUN、计时、单项PASS、包PASS与包结果。实际计时为 **1.060893919s（1,060,893,919ns）≤ 原1.25s**，日志为1016B glob、stop glob、0 pages；单项总时1.93s、包1.985s另列，不能代替计时窗口。原件绑定精确f689及上述c053测试源码：计时前的全量结构检查通过，原断言仍同时要求incomplete=true、next非空与原时间界。见原glob CI核对及完整costs日志。这是正确图的一个CI样本，未声称历史失败的耗时原因或性能改善。

文档自身CI、修复no-ff/main CI、新record CI与指定两main配对继续待验。本节本机结果不代替这些义务，不宣布性能优化或原b4时间失败已解释。原fd revert失败及M12 open不关闭；决定73–75、全部原limits不变，不新增产品决定。后续仍以修复merge＋新record两个完整接受的main attempt逐片实际Test均值严格<450s，不跨attempt或以分支/本机样本拼接；S1集成未闭环，S2未放行。

M9原0.40–0.41s等历史成绩保留，但不能再当作正确十一层app fixture的标定。手工`BenchmarkGrepAtFullLoad`直接复用`newGuardNodes`经globRoots形成背景CPU工作；修复也改变其实际背景图，未来负载标定必须绑定正确图、硬件/负载和新原件，旧标定不可直接继承。本次没有执行该手工长负载benchmark，不把它新增为自动CI门禁，不据本机一次绿改预算；重新标定义务留[M12性能移交](../../M12-release/handoffs/M4-performance.md)。

## 986文档CI取消与guards调度拆分

本节补记于2026-11-03（Asia/Shanghai）。986文档提交`986195a308629131e99fa5ae63e2f8c2ab474c75`（tree `6d27bba6547cf19ac057c0cb208c9d9b1a63b257`，parent f689）的[CI41517408885 attempt1](https://github.com/open-nerve/NerveWiki/actions/runs/41517408885/attempts/1)为completed/cancelled，不接受该文档CI。之后仅CI调度修复提交为`ac84c8e68561e31183c1784ef714cbb4f7b8eb43`（tree `92ab644bdbdae9fb1cd1556975cc318e2b9e7f7b`，parent 986）；其[精确代码CI41523895769 attempt1](https://github.com/open-nerve/NerveWiki/actions/runs/41523895769/attempts/1)已接受，本轮独立审查亦已ROOT接受，证据另列。原f689代码CI成功、b4 costs失败、fd两失败与此前原席审查均保其原身份。

### 取消证据与最小调度改动

run原件与jobs原件绑定上述head/run/attempt；取消注解明确`The job has exceeded the maximum execution time of 30m0s`。该次guards job `125524369612`与Test步骤均cancelled，web和其余9个server jobs成功，e2e/image skipped。十份日志已保存，但不能由上传完成推导测试成功。

完整guards raw含全部18条原recipe及各自一个package `ok`，未见具体业务FAIL/Go timeout/panic/SKIP；job raw末条package ok后约74ms报取消。观察到包输出不等于Go/make/tee/preview整个管道exit 0；API终态仍是取消。现有证据没有逐进程数值退出码或资源剖面，不归因CPU、内存、磁盘、缓存或主机负载，见只读诊断。

ac84仅修改`Makefile`、`.github/workflows/ci.yml`与`README.md`，见实际提交回执及完整改动。`test-go-guards-core`保原前12条，`test-go-guards-sources`保末6条来源操作；两组交集为空，按core→sources拼回与原18条命令和顺序逐字相同。所有Go argv、selector、count、race模式、timeout、fixture与业务预算不变。完整`test-go-guards`以两个顺序递归make调用保留本地入口，前段失败即停止；`make test-go`仍先并行race六份、再串行costs→costs-lint→完整guards。CI仅以两份替换旧guards一份，不同时重复执行总guards。实际三入口native dry-run与完整recipe核对见静态覆盖回执，这不是新的产品测试执行。

当前精确workflow的清单为**11 server legs／14 jobs／40必需steps／11 server artifacts**：每个server的Go version与Test共22步，costs另2步，web9步、e2e6步、image1步；日志上传另行完整核验。`guards-core`与`guards-sources`各保持原**1800s** job上限，两Markdown仍15m、source六条仍10m、其余guards仍Go默认10m。原fail-fast:false、pipefail、日志预览/完整原件、costs独有lint/gen及e2e/image依赖不变。本次精确ac84代码CI已按该清单完整接受；后续每次完整CI仍须实际核验；历史13/38/10与单guards时长不全局改写，也不以旧串行包秒推算新job实际余量。

### 本地证据、代码CI与独立审查

提交前minimum8接受回执绑定986＋上述三个dirty文件（其字节与随后ac84相同），session21570实际exit0，八条命令全部完成；server short为8906 RUN／7373 PASS／1533 short SKIP，tools27 RUN/PASS，skip与无tests不算PASS。它不是ac84提交后的新一次minimum，也不替代完整CI。精确ac84提交后的完整`make -j1 gen-check`由session43349实际exit0，3388源／252 embed字节与Git执行位不变、native mode差异0，见postgen接受回执。

| 验收对象 | 本节接受事实 |
|---|---|
| 精确ac84代码CI | [精确代码CI41523895769 attempt1](https://github.com/open-nerve/NerveWiki/actions/runs/41523895769/attempts/1)为completed/success，ROOT按14 jobs/40必需steps接受，见精确CI回执；11份server上传、ZIP digest、完整raw与preview重建均接受，见日志回执。原六case／12 states／36 samples／24固定限值重算接受，见成本回执。原18条完整recipe在core12＋sources6两份中按原序各出现一次并各有package ok，无新增FAIL/SKIP，见recipe核对。core job `125546555127`为1066/1800s、余734s，Test1040s；sources job `125546555043`为568/1800s、余1232s，Test547s。五bootstrap实际Test依1至5为486/365/402/411/260s；分支样本不进入指定两main配对，不由此声称容量配对通过。 |
| 本轮独立审查 | 独立报告针对986..ac84及其tree，仅审Makefile、CI与README；Important／Minor／Nit／Q均0，无已证实待修复发现。审查者独立核原18条recipe、完整包装入口失败停止、workflow14/40/11与已下载原件，并执行7个隔离make/fake-go控制；没有运行真实Go、PG、浏览器或重跑产品门禁。ROOT已全文读取报告与控制代码、重核13个Git／48个输入／15个输出pins及6份命令日志序列，见ROOT接受回执。该窄审查不替代原S1三席，也不是main接受或S2放行。 |

正确fixture的原named-glob命令在本次CI执行一次，原计时窗口为**1.106197215s（1,106,197,215ns）≤原1.25s**，1016B glob、110万节点、stop glob/0 pages；原计时前结构检查、incomplete=true与next非空断言保留，单项总时1.94s与包1.974s不替代该窗口，见原glob核对。这仍是正确图的一个CI样本，不解释旧b4/fd耗时失败、不构成新标定或性能改善。

后续新Step文档自身CI、实际修复no-ff/main CI、新record CI及指定两main配对仍待验。新的完整main接受须按当前14/40/11清单逐项核验，五bootstrap指定修复merge＋新record的实际Test均值仍须逐片严格<450s，不跨attempt、不用分支/本机样本拼接；两个guards分别记录实际job用时与原1800s余量。本节不预填未来main身份、配对或S2放行。fd revert成本失败的历史原因及M12性能移交仍open；b4时间失败也不因调度拆分或一次后续绿被解释。既有产品合同、全部limits及手工full-load重新标定义务不变，不新增产品决定；S1集成未闭环，S2/S3未实施，P4/M10未完成。

## 修复文档与no-ff合并记录

本次补记于2026-11-03 06:33:56.456718（Asia/Shanghai）写入。此前各补充节的会话fixture、grep夹具修复、独立核对、本地门禁及性能观察均保原输入身份；其中“待验”“尚未发生”按原写入时点阅读。原a1f成功、fd CI41482903019 attempt1两失败、b4文档CI41501780779 attempt1的glob成本失败及原配对未完成不改写；986文档CI41517408885 attempt1因guards job原30m超时取消，18条recipe的package ok不等于整job或CI成功。本节只绑定本次实际ac84代码、最终修复文档与no-ff main，不增加产品决定。

| 阶段 | 实际身份与接受范围 |
|---|---|
| ac84调度拆分代码CI | `ac84c8e68561e31183c1784ef714cbb4f7b8eb43`，[CI41523895769 attempt1](https://github.com/open-nerve/NerveWiki/actions/runs/41523895769/attempts/1)，`m10-session-veto-startup` / push / completed/success；14个必需jobs、40个必需steps及11份server日志各自完整接受。完成于2026-11-03 05:20:46、ROOT接受于2026-11-03 05:33:32.465026（均为Asia/Shanghai）；CI、11份日志、六case/24原限值及原glob 1.25s。本地全量入口仍串行core12＋sources6，所有原Go recipe和预算保持；分支样本不进入main配对。 |
| 修复文档 | `2fc103dda645c4cced48dd830e445d2b5018d868` / tree `ad55f85fd2133480b76f543dce89ceb72ec8e229`，唯一parent `ac84c8e68561e31183c1784ef714cbb4f7b8eb43`；提交于2026-11-03 05:43:56（Asia/Shanghai），仅三份M10文档。最终三份原文以该提交Git对象及实际文件bytes/SHA绑定，准备预览不替代最终字节。承接fd→878→b4→f689→986→ac84→本次文档的一父链。 |
| 精确文档CI | [CI41531309743 attempt1](https://github.com/open-nerve/NerveWiki/actions/runs/41531309743/attempts/1)，head `2fc103dda645c4cced48dd830e445d2b5018d868` / `m10-session-veto-startup` / push / completed/success；14个必需jobs、40个必需steps及11份server日志完整接受。完成于2026-11-03 06:06:50，ROOT接受于2026-11-03 06:08:41.242275（均为Asia/Shanghai）；CI、11份日志、六case/24原限值及原glob。 |
| 修复no-ff | `97c9ad8e96c51a68a2824a893ddb92fd82340ae5` / tree `ad55f85fd2133480b76f543dce89ceb72ec8e229`；有序父`fd2607ab9fec4cade43fb28622873607891ae58f` / `2fc103dda645c4cced48dd830e445d2b5018d868`，合并于2026-11-03 06:09:11（Asia/Shanghai）。与已接受修复文档同树；实际身份回执。 |
| 精确修复main CI | [CI41534799713 attempt1](https://github.com/open-nerve/NerveWiki/actions/runs/41534799713/attempts/1)，head `97c9ad8e96c51a68a2824a893ddb92fd82340ae5` / `main` / push / completed/success；14个必需jobs及40个必需steps全部成功。完成于2026-11-03 06:31:36，ROOT接受于2026-11-03 06:33:49.054770（均为Asia/Shanghai）；CI、11份server原始日志、六case/12 states/36 samples/24原限值及原glob分别绑定本head/run/attempt。 |
| ac84分支实际用时 | 五片实际Test步骤486.0/365.0/402.0/411.0/260.0s；guards-core job 1066.0/1800s、余量734.0s；guards-sources job 568.0/1800s、余量1232.0s。 |
| 文档分支实际用时 | 五片实际Test步骤466.0/241.0/389.0/405.0/330.0s；guards-core job 817.0/1800s、余量983.0s；guards-sources job 553.0/1800s、余量1247.0s。 |
| 修复merge侧用时 | 五片实际**Test步骤**461.0/402.0/266.0/416.0/275.0s，仅为新配对的merge侧样本；guards-core **job** 1000.0/1800s、余量800.0s；guards-sources **job** 852.0/1800s、余量948.0s。不把job/process/package或分支用时用作main Test步骤。 |
| 此前尝试 | ac84代码在本次绑定head上无此前CI尝试。文档在本次绑定head上无此前CI尝试。merge main在本次绑定head上无此前CI尝试。各尝试保原run/head/attempt及结论，不拼接片段。986取消及更早失败保上述历史身份。 |
| 本record后验 | 本次补记写入时，自身SHA、提交后检查与精确main CI仍待实际产生；新指定两main配对待ROOT接受。后验留树外回执，随下一次既定文档自然转录，不为回填自身SHA/CI另建record-of-record。 |

当前40个必需steps为11个server各Go version/Test共22，加costs额外2、web 9、e2e 6、image 1；11次artifact upload单独核验，不计入40。core12与sources6命令完整性沿真实CI及独立原始日志诊断核验；P4成本只取costs的index-catalogue一case及guards-core的五个virtual case，共六case/12 states/36 samples/24固定限值，不声称覆盖sources六项成本。

f689代码CI41510773390 attempt1的五片Test 467/310/395/308/256s、guards job1139s，以及正确fixture的原glob 1.060893919s/PASS、十份原始日志与六case/24限值接受均保[原补充节](#b4文档ci失败与grep-glob-fixture修复)身份；旧13/38/10及单guards实测仅描述当时工作流，不借作当前14/40/11接受。

ROOT指定的新配对只取**本次实际修复no-ff merge（first parent为fd、second parent为上述最终修复文档）＋本record**两个完整接受的main attempt；逐片`mean_i=(repair_merge_Test_i+record_Test_i)/2`，五均值均须**严格<450s**。旧a1f/fd失败配对、b4/986失败、本地及代码/文档分支样本均不计入，不跨attempt拼接。修复main CI成功不证明原fd、b4或986的历史资源原因，测试fixture修复与调度拆分不称为性能优化；原1.25s、12s/8s guard、15s请求期限、其它原limits、M12 open及决定73–75不变。M9/M12的正确图与手工full-load重新标定义务保持，不混用旧图样本。本record CI与新配对实际接受前，S1集成未闭环、S2未放行；S2/S3、P5与M10收尾仍待，M11未开始。

## e650记录失败与bootstrap容量维护

本节记录`m10-bootstrap-capacity`分支本轮容量维护，记录日期2026-11-03（Asia/Shanghai）。实际代码`af392f641f043e9497ce39abd7734ac3d55007c3` / tree `d96032187be0079cdc5e807cba37741b7c105c5e`的本机证据、完整CI41556086210/a2及原A/B/C03同席核对已获ROOT接受；文档自身CI和后续指定两main配对仍待，S1未闭环、S2未放行。合并见[00§12](../00-M10-design.md#12-phase-进度表)。

按02-workflow保持作者、原A/B/C独立审查者与文档起草者/ROOT审阅分工：01是冻结源码初审，02是实际提交、完成的本机证据及注释桥接补核；三份02报告作成时尚未收到CI最终原件。随后a1整CI失败、A后发失败补核、一次预登记完整a2和原席03最终CI补核分别记账，不能倒填成02报告已验CI。原S1三席、96/288业务控制、全部旧成功/失败/取消/pending保原身份；本轮六控制独立分账，不改变S2正式设计草稿、产品决定或M12已open条目。

### 实际触发：lint首败、同head复核与指定pair失败

| 事实 | 实际身份、结论与界限 |
|---|---|
| e650 record | `e65043fe059c2788338278a2e68a1b80ee7c0a80` / tree `f2a9baecde16c45d27611a2a86d3868e492f5917`，唯一parent `97c9ad8e96c51a68a2824a893ddb92fd82340ae5`；只是本轮实际Git起点，不绑定为S1 accepted_main。 |
| a1整CI失败 | CI41540276246/a1、main/push；11 jobs success、costs-lint job125602371138 failure、e2e/image 2 skipped。100000页的`TestLintOfAHundredThousandPagesKeepsItsTimeAndMemory/summary/sample-1`在原4s request/2s worker下Execute 2.002009056s返回busy；第13条lint recipe失败、第14条`TestLintPropertiesReadOnlyNecessaryValues`为NOT_RUN。summary原固定median guard未走到断言，记NOT_EVALUATED；本轮未作新median校准；不能据壁钟判断具体等待/worker期限，cleanup预期失败子进程由外层PASS核验，不是第二个产品失败。 |
| 调查与复核边界 | ROOT接受只读调查的事实/界限：未核实实现或fixture问题，根因UNKNOWN。2026-11-02 23:12:27.908449 UTC预登记一次同head完整复核，23:12:40.601829 UTC请求回执；无源码/fixture/limits改变，无新增产品决定号。原a1所有日志及失败保留，不继续盲rerun。 |
| a2完整接受 | CI41540276246/a2、精确e650/main；14 jobs／40必需steps／11 artifacts全部接受，ROOT最终回执2026-11-02 23:39:09.421554 UTC。六P4 cost cases、原glob1250ms、core12＋sources6、全部14条lint recipe和六个真实100k样本、八项原median guard及预期失败child分别核验。六样本成功只说明本次未复现，不称修复、根因已知、环境已证实或余量可靠。a1成功jobs不拼接给a2。 |
| 指定pair失败 | 固定97c9/CI41534799713/a1＋e650/CI41540276246/a2；真实pair verifier exit1，首片Test平均466.5s不满足严格<450s。整a2成功不取消容量失败，不换pair、不改用job/process/package或分支时间。 |

| bootstrap片 | 97/a1真实Test(s) | e650/a2真实Test(s) | 均值(s) | 原严格<450s |
|---|---:|---:|---:|---|
| 1 | 461 | 472 | 466.5 | 失败，超16.5s |
| 2 | 402 | 317 | 359.5 | 满足 |
| 3 | 266 | 359 | 312.5 | 满足 |
| 4 | 416 | 285 | 350.5 | 满足 |
| 5 | 275 | 258 | 266.5 | 满足 |

已知原件：a1整CI失败、a1十一原件、独立调查接受、一次完整复核预登记、a2全部接受但pair拒绝、真实pair verifier拒绝。这些原件支持历史结论，不充当新候选运行证据。

### af392f精确分支CI失败与一次完整复核

| 已发生事实 | 实际身份与界限 |
|---|---|
| a1最终整CI | 精确`af392f641f043e9497ce39abd7734ac3d55007c3` / `m10-bootstrap-capacity` / push，CI41556086210/a1为completed/failure；14个jobs全部终态，11 success、`server (guards-core)` job125655196383 failure、e2e/image skipped。ROOT失败观察。 |
| 原core第8条失败 | `TestARevertAtTheUnitsBoundTakesItsTime`在10000 nodes/400000 history实测12.45394147s，超过原12s；同条`TestARevertOfManyTopsAtItsBoundTakesItsTime`为2.786200471s、原8s下PASS。真实core raw第521–526行；第8条进程失败，原第9–12条NOT_RUN，不能记整core通过或把同条many-tops成功当失败已消失。 |
| 原件封存与拒绝 | a1十一原件收据封存全部11个server原件，receipt SHA256 `b004230d596ce4747f061b5798418641476fc6dfb7e845c74b3f0f9d3a0d8659`；ZIP/raw/preview与attempt上传身份分别保存。原checker拒绝回执真实exit1，没有接受输出；watch59741/d01a1c exit1。 |
| 根因与原风险 | 与fd原12.299969446s同属large-revert guard越界；根因仍UNKNOWN。af的server树与workflow相对e650相同，原guards recipe未改；这不能证明runner/环境归因，也不能把窄调度改动称为性能修复或重新标定。12s/8s/15s、其它原预算与既有M12债不变。 |
| 一次完整a2预登记 | 一次完整workflow复核决定在2026-11-03 01:19:26.210543 UTC预登记同head、最多一次、全workflow重复观测；实际请求回执在01:19:30.693196 UTC实际exit0，ROOT工具session37451/terminal313230。真实argv没有`--failed`，没有更换源码/fixture/阈值，也不挑只失败jobs。 |
| a2完整终态与接受 | a2最终run及全部jobs确认同run41556086210/attempt2/同head的completed/success：14 jobs全部成功、40必需steps和11个server Upload齐全；watch31877/26b466 exit0。ROOT完整代码CI及原件接受于2026-11-03 01:47:38.130408 UTC接受完整原件及原限值检查。一次预登记完整复核已完成，不拼接a1成功jobs；仅说明本次未复现，不代替未来指定main配对。 |
| A后发失败补核与当前处置 | A同席失败诊断及当时ROOT接受保留1项Important：a1未通过的CI验收阻塞，非已证实的新Make因果缺陷；Minor/Nit/Q无新增。实际HTTP200完整请求12.45394147s超原12s，不是15s请求超时、Go超时或DB错误。ROOT原三席03接受确认本轮完整代码CI前提由预登记a2满足；a1仍失败，根因UNKNOWN，M12性能债仍open，不作修复/标定或环境归因。02无新增I/M/N与这项后发历史发现分别保留。 |
| 精确完整代码CI及原件 | a2十一原件收据保存精确af/41556086210/a2的11份ZIP/raw/preview，receipt SHA256 `698a3d033b150ce7c2c1c6c9adbcf4cf49fc96a40b33ffe68954ace90bce9bb5`；API digest、解包字节与本attempt成功Upload窗逐份相符。精确CI checker回执及四项raw检查回执均实际exit0；六P4 cost cases/12 states/36 samples/原24 limits、glob1.040098623s≤原1.25s、core12＋sources6、lint14 recipes/1047 RUN→PASS/六真100k样本/八原median guards/六Finished cleanup均接受；预期失败child与外层PASS单独核验；原席03已核完整代码CI，文档自身CI和未来main pair仍待。 |

| bootstrap片 | 失败a1成功job的Test(s) | 完整接受a2的Test(s) | 计量边界 |
|---|---:|---:|---|
| 1 | 425 | 358 | a1局部job与a2分支样本分别记账 |
| 2 | 336 | 269 | a1局部job与a2分支样本分别记账 |
| 3 | 274 | 390 | a1局部job与a2分支样本分别记账 |
| 4 | 356 | 297 | a1局部job与a2分支样本分别记账 |
| 5 | 259 | 320 | a1局部job与a2分支样本分别记账 |

以上数字分别取各精确attempt的Test步骤，既非job总时长，也非Go package或本机process。a1整条失败，其成功jobs不构成整CI接受；a2整条已接受，但本分支两attempt不组成未来main配对。a2的guards-core/sources job为853/592s，Test为826/572s，均保原各1800s job上限；large9.058581014s、many-tops1.917504607s在原12s/8s下通过。旧97/e650首片466.5s失败pair仍按前表保留。

### 维护范围、源码身份与逐名归属

既有[M9/P2§6.5](../../M9-mcp/02-P2-writes-history.md#65-ci-的余量)要求过线片向可容纳邻居挪界，挪前本机估计、挪后两次真实CI核对；两侧都须在线下，邻居越线则撤回并按原规则处置，从不提高`-timeout`。本轮实际commit `af392f641f043e9497ce39abd7734ac3d55007c3`，tree `d96032187be0079cdc5e807cba37741b7c105c5e`，唯一parent `e65043fe059c2788338278a2e68a1b80ee7c0a80`，完整Git差异只有Makefile；最终38163B、native0664、SHA256 `6f6de4f506865ea7c91445583f12a9fff86516999b73c82b86272b7352032036`，只改FIRST/SECOND可执行内容及说明注释。实际提交与正常分支推送均真实exit0，正常推送本分支，没有main接受。

旧ee1327／38139B输入的native17在2026-11-03 00:43:51.659862 UTC实际完成，ROOT旧字节native17接受接受五片各独立双轮；其source一直是e650＋dirty Make，旧minimum01与六控制同样保原身份。随后B-N1实际应用和桥接在00:45:16.606329 UTC只把Makefile:244的“与 e650 的源码顶层相同”改为“与 e650 在 race 构建中参与的源码顶层相同”；非注释字节相同、17对真实Make展开（34条命令）逐对argv/stdout/stderr相同且均exit0。最终6f6de4字节与af Git Make完全一致，B-N1已解决。它是明确的注释桥接，不是把旧执行改标成新字节或新提交运行。

新minimum02已在提交前e650＋最终6f6de4 dirty Make上实际重跑八命令并获ROOT接受；只有随后提交后的postgen才绑定clean af392f。三种输入身份分别保留，旧native、旧minimum及旧控制不会改名成af新跑。

候选两个表达式为：

```make
BOOTSTRAP_FIRST = ^Test[A-D]
BOOTSTRAP_SECOND = ^Test(E|Source|The(MCP[C-Z]|Contents)|VirtualExport$$)
```

真实make展开后`VirtualExport$$`保留单个`$`；SECOND仍按原FIFTH排除，THIRD/FIFTH及第四片的既有互斥语义保持。历史race编译499顶层与e650在race构建中参与的源码顶层对应，旧分配168/40/116/123/52，候选157/51/116/123/52；只移动以下11名，其他488名逐一保持原片，3/4/5完整成员不变。本轮fresh编译499 race/531 nonrace及5个真实Make dry-run已获ROOT库存接受，库存SHA256 `1e4b55f6945fae28028c7cb7d6c262d26fc9f61ffd8cb17ada2c79a21ad5544a`。它只证明编译名单与分配，32个nonrace专属top未在race过程执行，不等于native测试pass。

| 顶层测试 | 原片→候选片 | 真实路径 |
|---|---|---|
| TestEachChangeIsRevertedThroughServe | 1 → 2 | `server/internal/bootstrap/history_test.go` |
| TestEndingASessionASaveInItHolds | 1 → 2 | `server/internal/bootstrap/interleavings_content_test.go` |
| TestEveryKindBecomesItsProblem | 1 → 2 | `server/internal/bootstrap/errors_test.go` |
| TestEveryMCPAnswerIsNotStored | 1 → 2 | `server/internal/bootstrap/mcp_test.go` |
| TestEveryMCPToolHasItsCodes | 1 → 2 | `server/internal/bootstrap/permission_matrix_mcp_test.go` |
| TestEveryNotebookDeletionResetsItsStreams | 1 → 2 | `server/internal/bootstrap/events_deletion_test.go` |
| TestEveryOperationAcceptsAPersonalAccessToken | 1 → 2 | `server/internal/bootstrap/contract_test.go` |
| TestEveryPageWriteKeepsTheIndex | 1 → 2 | `server/internal/bootstrap/links_test.go` |
| TestEveryPageWriteReachesTheStreams | 1 → 2 | `server/internal/bootstrap/events_pages_test.go` |
| TestEveryProblemResponseDeclaresItsHeaders | 1 → 2 | `server/internal/bootstrap/contract_test.go` |
| TestEveryResolutionCaseExportsAsAVault | 1 → 2 | `server/internal/bootstrap/transfer_obsidian_test.go` |

上述11名的10份文件在历史0b0与e650的Git blob相同；历史两轮顶层Elapsed和27.05/28.21s、均值27.63s乘规划比1.6得44.208s，第一/第二片估计422.292/403.708s。run→terminal口径另存，不能把Elapsed、process或预测当新CI Test。所有估计只支持候选选取，不证明本轮通过。作者调查、ROOT计划接受、实际patch与静态记录、ROOT候选核对保留原bytes/SHA。

### 已完成实施证据与尚未发生的接受

| 项目 | 实际结果与源身份 |
|---|---|
| minimum01历史输入 | session57154/f52359 exit0、八命令已接受；输入e650＋旧ee1327 dirty Make（3388 source/252 embed），保持minimum01原记录，不改标新字节。 |
| fresh库存与旧native17 | native17：执行37435/011b4a exit0，verifier64551/1c8cc5 exit0，全部17阶段完成。499 race/531 nonrace真实库存分配157/51/116/123/52，32个nonrace专属仅枚举；十个实际race过程两轮各4099 RUN/终态（4095 pass/4 skip）、4097归一身份和499顶层，skip不算pass。旧ee1327输入，完整source/工具/锁/原始日志与五个process均值已核。 |
| 六项调度控制 | 六控制：session70381/55c35b exit0，实际各0/1/0，故障影响15/15/53/157/6/0；原preparation01未执行静态问题不计kill。所有控制保旧ee1327输入及恢复身份，借已核注释等价连接最终Make，不声称在af重复执行。 |
| B-N1实际应用/桥接 | 实际注释桥接已落地，唯一注释差异在244行；17组34条真实Make展开相同，最终38163B/SHA6f6de4。旧native source未重标，B席02确认Nit解决。 |
| 新字节minimum02 | minimum02：执行84603/e0e2f4 exit0，verifier22908/371c76 exit0，八原命令完整；提交前e650＋6f6de4 dirty Make，3388 source/252 embed。server短测8906 RUN=7373 pass+1533 skip，93包pass/23无测试包skip；tools27 RUN/pass。不是af提交后重跑。 |
| 实际code commit与push | code commit：`af392f641f043e9497ce39abd7734ac3d55007c3` / tree `d96032187be0079cdc5e807cba37741b7c105c5e` / parent e650，00:51:36.714678 UTC生成clean提交、仅Make变更；code push正常`git push --set-upstream origin m10-bootstrap-capacity` exit0。源码身份与真实Git对象一致。 |
| 提交后postgen | postgen：clean af392f上执行97766/366b82 exit0、verifier b8b05e exit0。源码字节/Git执行位、embed与Git状态相同；固定生成路径中实际76项native0664→0644，不能称完整磁盘snapshot无变化，未豁免其它路径/mode/字节。 |
| 三席第二轮补核 | ROOT原三席第二轮接受已接受A/B/C02对af实际提交、旧native完整原件、唯一注释桥接、新minimum与postgen的核对，无新增Important/Minor/Nit，B-N1解决。报告时尚无最终CI原件；该范围不含后来a1失败或a2终态接受。后续原席03完整CI核对已完成：ROOT原三席03接受于2026-11-03 01:57:36.534634 UTC接受原A/B/C03对精确af/41556086210/a2的核对，无新增Important/Minor/Nit；B-Q1代码CI部分与C代码CI等待项已核，文档自身CI和未来指定main pair仍待。 |
| 精确代码CI | a1最终失败、原checker exit1、11原件保留；预登记完整a2按14 jobs/40必需steps/11 artifacts及原costs/guards/lint接受，原checker实际exit0。ROOT完整代码CI及原件接受及ROOT原三席03接受共同绑定精确af/41556086210/a2，不替代文档自身CI或未来main pair。 |
| 本文档提交前检查/自身CI | 文档自己的minimum、commit/postgen、push与精确完整CI待真实执行并另留回执；按02-workflow由ROOT审文档、先archive试用再应用，代码CI不替代文档自身CI。 |
| 后续no-ff、record与指定两main配对 | 尚未发生；合并见[00§12](../00-M10-design.md#12-phase-进度表)。未来两个完整main head/tree/run/attempt、有序父与逐片实际Test均值严格<450s另验，尚无本轮main接受；本机、分支CI、a1局部成功job与旧失败pair不替代。 |

| 旧ee1327本机片 | process第一/第二→均值(s) | package第一/第二→均值(s) | 顶层run→terminal区间和第一/第二→均值(s) |
|---|---|---|---|
| 1 | 295.724509599/301.030830894 → 298.377670246 | 291.02/296.093 → 293.5565 | 289.80813801/294.869301641 → 292.338719826 |
| 2 | 274.231661839/278.353828424 → 276.292745132 | 269.395/273.413 → 271.404 | 268.166639714/272.185372171 → 270.176005942 |
| 3 | 269.129572462/274.750352339 → 271.9399624 | 264.09/269.612 → 266.851 | 262.840678378/268.362066093 → 265.601372236 |
| 4 | 294.758599641/287.609326637 → 291.183963139 | 289.769/282.69 → 286.2295 | 288.469777713/281.379607763 → 284.924692738 |
| 5 | 244.34717095/241.422895823 → 242.885033387 | 238.83/236.276 → 237.553 | 237.605124951/235.056518321 → 236.330821636 |

本表用于阅读，完整原精度与引用见native完整verification；五process均值均严格<450s，仅为本机旧字节双轮接受。process来自heavy锁后Python `time.monotonic()`围住实际child；heavy排队/wrapper另记，package为Go包终态Elapsed，顶层区间取真实run→terminal，不把并行矩阵父Elapsed或本机均值换成CI Test。

本轮验证范围及实际Git差异确认server/web/api/workflow与其余Make可执行行未变。旧完整组件观察保原身份；本轮native覆盖七库存阶段和十个bootstrap race过程，minimum/postgen各有上述范围，不冒称重新本机跑过成本/guards/web/e2e。a1整CI失败保留，预登记完整a2已按真实原件接受；本次通过不能解释a1性能根因或补写新的本机执行。heavy全局锁、锁后3GiB/T7预检、真实source-command.lock、源码bytes/native modes、index/embed/tool/env检查与全部预算延续。

| 控制名 | 实际唯一故障与诊断 | 正常/故障/恢复 | 实际证据 |
|---|---|---|---|
| `first_reincludes_e` | FIRST重新包含E；15个TestE重复归入1/2，含11个迁移名及4个原在第二片的TestE | 0 / 1 / 0 | 实际三腿结果；受影响15名，旧ee1327恢复已核 |
| `second_omits_e` | SECOND漏E；同15个TestE错误落入4，虽全集覆盖仍由错片检查拒绝 | 0 / 1 / 0 | 实际三腿结果；受影响15名，旧ee1327恢复已核 |
| `third_omits_skip` | 第三片去掉SECOND/FIFTH skip；53名双归属：52个第五片成员为3/5，另TestVirtualExport为2/3 | 0 / 1 / 0 | 实际三腿结果；受影响53名，旧ee1327恢复已核 |
| `fourth_omits_first_exclusion` | 第四片漏FIRST排除；157名重复1/4 | 0 / 1 / 0 | 实际三腿结果；受影响157名，旧ee1327恢复已核 |
| `fifth_omits_wikitools` | 第五片run漏WikiTools；6名遗漏 | 0 / 1 / 0 | 实际三腿结果；受影响6名，旧ee1327恢复已核 |
| `timeout_relaxed` | 原15m改20m；归属受影响0名，但五片COMMAND_CONTRACT均拒绝 | 0 / 1 / 0 | 实际三腿结果；受影响0名，旧ee1327恢复已核 |

六项的期望归属独立取历史已接受逐名分组，仅把手工11名移至2；这里的11是正常候选迁移数，不是前两项变异的15个实际受影响名。原第二片另4名为`TestEveryResolutionCaseImportsFromAVault`、`TestEverySessionChangeReachesTheStreams`、`TestEverySoftDeletedTableHasAPurger`、`TestEveryVisibilityChangeResetsItsStreams`。不能从候选selector反推期望答案。旧ee1327输入的实际分组已由真实Make展开与fresh compiled inventory比对，ROOT重解析90条真实make命令、逐腿重算499名归属；全集/互斥/每名原片及3/4/5不变同时核查，第二项覆盖全集仍被错片检查抓住。控制作用于树外临时Make副本，原候选不改；各项从备份恢复到38139B/ee1327/native0664后复验，实际控制原件与ROOT回执共同保存。与原S1业务96/288控制分账。

### 独立审查的已发生阶段与后续边界

| 席位 | 01冻结源码初审（原报告与控制保持） | 02实际提交/完整本机同席补核 | 03精确代码CI同席补核与剩余范围 |
|---|---|---|---|
| A | A01：I/M/N/Q均0；独立499归属、美元转义/惰性统一入口；两个实际0/1/0控制抓VirtualExport双归属及无效selector。旧控制保ee1327原执行，不称af新跑。 | A02：实际af Git、3388 source/252 embed、17对桥接和独立17条make-n、完整native/新minimum/postgen已核；I/M/N/Q均0。 | A03核精确14/40/11、五片原命令、18 guards及costs/glob/lint原件与限值，无新增I/M/N/Q。02当时未收最终CI及随后A的1项Important失败补核保留；本轮完整代码CI前提现已满足，文档CI/main pair仍待。 |
| B | B01：0 Important/0 Minor，B-N1一项Nit已采纳；15m→20m负对照真实0/1/0，旧pair被严格拒绝。 | B02：B-N1在af已解决，无新增Important/Minor/Nit；独立十过程/每节点身份和十进制用时、17对桥接/本席make-n、新minimum与76项生成mode核对通过。B-Q1本机部分已完成。 | B03独立核两attempt原run/jobs、22份ZIP/raw/preview、Test时间、18 guards及原限值，无新增I/M/N；B-Q1代码CI部分已核实，文档CI/merge/record/指定main pair仍待。02当时未核CI的范围不倒填。 |
| C | C01：0 Important/Minor/Nit；吞递归错误的checker0/1/0、Make2/0/2及50条错误后续调用原件保留。 | C02：af、旧ee1327完整17阶段、五片各独立双轮、注释桥接、新minimum/postgen已核，无新增Important/Minor/Nit；旧Q的本机/提交部分完成。 | C03核精确14/40/11、50条costs/lint/guards原recipe、6case/24limits、glob及lint节点/样本/median/cleanup，无新增I/M/N；本席代码CI等待项已关闭。02当时in_progress按历史阅读，文档CI和指定main pair等仍待。 |

ROOT同席第二轮接受在2026-11-03 01:17:04.978678 UTC只接受上述实际源码和已完成本机证据补核，并已明确知道a1失败、当时独立失败诊断另在进行；merge_ready=false、main_pair_accepted=false。初审B/C、ROOT分类与第二轮三席自写检查的预检查/解析断言错误均保在各自原报告/原件，未记作产品首败或控制kill；最终补核退出与范围不抹去那些历史。B-N1是注释口径修正，不是性能修复；旧native/控制身份经具体字节和实际命令桥接，而非重命名。

ROOT于2026-11-03 01:22:07.676771 UTC接受后发A失败补核：Important=1为a1未通过的验收阻塞，Minor/Nit/Q无新增；没有证明本次Make调度是原因。12份直接调用链文件在fd/878/e650/af四head逐文件同blob；af/e650/97的整个server同tree，未把fd整个server错称相同。失败守卫位于!race文件，不在499 race迁移名单，独立guards-core recipe不使用FIRST/SECOND。代码/逻辑独立不是底层资源物理隔离证据；现有CI缺同一慢请求内SQL/pool等待与同期CPU/I/O，旧878插桩成功计划/失败后的资源快照不能解释当前慢样本。A当时限定一次完整a2只观察原阈值下是否再次越界、若再红即停止自动重跑；本次通过仍不能作根因、修复或校准结论。

ROOT原三席03接受于2026-11-03 01:57:36.534634 UTC接受原A/B/C03对精确af/41556086210/a2的核对，无新增Important/Minor/Nit；B-Q1代码CI部分与C代码CI等待项已核，文档自身CI和未来指定main pair仍待。ROOT接受代码与完整代码CI，允许最终文档绑定；A后发Important所列本轮完整代码CI前提已满足，a1失败原件、未知根因和M12性能义务仍保留。原席03审查脚本自身的预检查/解析失败记录照实封存，不记作产品失败或控制kill。接续仍须文档应用/提交/自身CI、未来no-ff/record与指定两main配对；未知提交号不预填。

本轮执行既有容量规则，没有新增产品决定、性能优化、测试/fixture/命名变化、组数或阈值变化。原glob1.25s、revert12s/8s、request4s/summary worker2s、summary原固定median guards、请求15s、race15m、五CI job各25m、两guards各1800s、14/40/11与指定main均值严格<450s均保留。原a1f成功、fd两失败、b4失败、986取消、e650/a1失败、97/e650指定pair失败与本轮af/a1失败分别保留；未知根因、O25 OPEN、M12 open和正确图full-load重标定义务仍在。本轮代码及完整代码CI接受不替代文档自身CI和未来指定main配对；S1未闭环，S2继续未放行。

## 容量维护文档与no-ff合并记录

补记日期2026-11-03（Asia/Shanghai）。此前容量维护及独立审查结论保原记录时点；本节只补分支文档、no-ff身份与已发生的CI接受，不提前记录本record自身CI或配对结果。

| 项目 | 实际记录与边界 |
|---|---|
| 分支文档与前置检查 | 文档`3166bbcde7e23b4eb614b6367ea7e95c801e91cb` / tree `c879036e7db93178922d966d199f54f08f0db710`、parent `af392f641f043e9497ce39abd7734ac3d55007c3`，只改本三文档；提交回执。提交前minimum8八命令实际exit0（70273/7d5129；原verifier68145/a0e62c exit0），保提交前af输入身份；提交后ROOT postgen接受47776/6c9fe8 exit0、verifier423e23 exit0，3388 source/252 embed字节与Git modes不变、本次native mode差异0。正常推送actual_exit0。 |
| 文档精确完整CI | 3166 / CI41566238435 / attempt1：完整接受。完整14 jobs/40必需steps/11精确attempt原件及原costs/guards/lint按实际ROOT回执绑定，不与代码分支CI拼接。 |
| 本轮no-ff main | head `e07cc1964238b3d248c791c124d9a9fc48dd9253` / tree `c879036e7db93178922d966d199f54f08f0db710`，有序父第一`e65043fe059c2788338278a2e68a1b80ee7c0a80`、第二`3166bbcde7e23b4eb614b6367ea7e95c801e91cb`；实际合并回执：no-ff回执、正常推送。精确main CI41569049575/attempt1：完整接受。本main五片实际Test(s)：422/344/360/403/280，仅是指定pair第一份样本。 |
| 本record与指定配对 | 预登记固定顺序为上述no-ff、其仅改本三文档的直接child record，两者各attempt1；本record自身完整CI及两者五片Test算术均值严格<450s仍待真实产生并接受。任一完整attempt失败或容量违反即阻塞、先诊断，不自动重跑、拼接成功job、替换pair或提高限值；旧97/e650首片466.5s失败pair、af代码分支、3166文档分支及本机时间均不充数。 |

旧a1f成功、fd/b4失败、986取消、e650/a1失败、旧指定pair失败与af/a1失败均保留；af/a2只证明该次未复现，根因UNKNOWN、M12性能债和O25 OPEN仍在。合并见[00§12](../00-M10-design.md#12-phase-进度表)。此record不预言自身未来CI或配对通过，S1未闭环、S2未放行。

## S1修复record后验（随S2正式设计转录）

本节随决定76的S2正式设计自然转录S1实际收口，不新增record-of-record。ROOT于2026-11-03 12:17:16.929206 +08:00（Asia/Shanghai）以ROOT收口接受S1实施与主线集成，B-Q1已关闭，开放Important/Minor/Nit均0；原B04最终核查保留原B独立核查及其原件，不代签ROOT接受。下列历史失败与各段写入时点的pending全文保持。

| 对象 | 实际后验与边界 |
|---|---|
| accepted main | ca4688a1e9255d68b9ce54ff1d9c0acb824b3068 / tree 1977eec1626950d51c11d3e6f2fc80ad6be7974c，唯一parent e07cc1964238b3d248c791c124d9a9fc48dd9253；ROOT收口时main与origin/main相同且clean。 |
| record本机检查 | 提交前minimum8在e07加三文档工作差异上实际通过；ca提交后postgen已接受，检查3388源文件/252内嵌文件，native mode差异0。先前postgen未启动的PATH诊断原件保留，未改写为产品失败。 |
| 精确record CI | main、ca、CI41573821884/a1的record完整CI接受：完整14 jobs／40必需steps／11 artifacts及原四raw检查均接受；不拼接attempt。11原件归属连接原件与各Upload窗口。 |
| 成本与原限值 | 本record六P4成本case／12 states／36 samples／24固定限值已接受；glob 1.073215746s≤1.25s，revert large 8.81704111s／many-tops 1.950760754s分别在12s／8s内。lint为14 recipes、1047 RUN/PASS、6真100k样本、8 median；不以P4成本case代替sources六条recipe。详情及原件pins见上述完整CI回执。 |
| 有序main配对 | 按原预登记固定为e07的CI41569049575/a1→直接child ca的CI41573821884/a1；指定配对接受接受以下Test步骤均值。merge的完整接受见merge CI；不以branch、本机、job或package时间代Test。 |

| bootstrap片 | e07/a1 Test（s） | ca/a1 Test（s） | 均值（s，严格<450） |
|---|---:|---:|---:|
| 1 | 422 | 322 | 372 |
| 2 | 344 | 336 | 340 |
| 3 | 360 | 351 | 355.5 |
| 4 | 403 | 295 | 349 |
| 5 | 280 | 281 | 280.5 |

两guards的job与Test另列；job原上限各1800s，余量仅为本次观测。

| main样本 | core job／Test／job余量（s） | sources job／Test／job余量（s） |
|---|---:|---:|
| e07/a1 | 1018／993／782 | 813／795／987 |
| ca/a1 | 827／805／973 | 670／649／1130 |

原a1f成功、fd两失败、b4 glob失败、986取消、e650/a1 lint失败、旧97/a1＋e650/a2首片466.5s容量失败及af/a1 revert超原12s失败继续成立；后续通过只说明本次未复现，性能根因仍UNKNOWN，不作环境归因或性能修复结论。原1.25s、summary 2s、12s/8s、15s请求期限、15m/25m、严格450s及全部原limits不变。O25 OPEN、M12 OPEN和正确图手工full-load重新标定义务保留。

本次只放行决定76的S2正式设计绑定；该文档自身minimum10、实际commit、提交后gen-check及精确完整CI仍待真实接受。随后才可创建/绑定隔离S2实施WT并放行原三次有序代码提交。S2/S3均未实施、S2实施未放行，实际SDK/producer/控制/成本仍未运行，P4/M10未完成、M11未开始。190文件、146控制ID及34基础腿＋4 producer复验＝38计划腿保原合同，不把S1闭环写作S2实施完成。

## 附录：全部独立控制腿分账


这是既有作者/ROOT证据的导航；没有新运行。S1-3保留25轮，其中22条最终腿；S1-4保留41次尝试，其中38有效。不同单元的源码身份各自保留，不冒充同一最终树重跑。设计仍是136个计划ID，S2/S3控制未执行。

## S1-1

| 控制 | 最终纳入 | 实际退出 | 记录的业务首败/限定 |
| --- | --- | --- | --- |
| V01 | 是 | 0 / 1 / 0 | notebook read index returned page.not_found instead of page.virtual_file; real_paths remained passing |
| V02 | 是 | 0 / 1 / 0 | batch [edit Locked, delete index.md] and reverse returned page.not_found at the expected op instead of virtual_file in both scopes |
| V03 | 是 | 0 / 1 / 0 | all four reserved directory names returned page.not_found instead of virtual_file; real child check remained passing |
| V04 | 是 | 0 / 1 / 0 | move to root index.md actually succeeded and changed position/history, so the expected tool-error assertion failed; original existing guard regression, not a new implementation red |
| V05 | 是 | 0 / 1 / 0 | limit 1 omitted the virtual prefix; limit 2 repeated the first true ID and omitted log when virtual -2/-1 values were mixed with real ordering |
| V06 | 是 | 0 / 1 / 0 | V1/V2 resumed through O=0 and skipped the -3 real first node in LLM root, nested and standard cases |
| V07 | 是 | 0 / 1 / 0 | V=-1, V=3, V=1 plus nonzero O and V=2 plus nonzero I all returned nil instead of bad_request |
| V08 | 是 | 0 / 1 / 0 | official SDK still fit the complete first 40/42 through index, but wrong candidate-tail cursor resumed with zero items instead of log/raw |
| V09 | 是 | 0 / 1 / 0 | literal log.md and both globs omitted log while true seeded paths and index remained |
| V10 | 是 | 0 / 1 / 0 | real overview pages count was 5 instead of 3; attachments remained 1 |
| V11 | 是 | 0 / 1 / 0 | real Empty.md retained its version but lost byte_size:0 in both MCP scopes |

## S1-2

| 控制 | 最终纳入 | 实际退出 | 记录的业务首败/限定 |
| --- | --- | --- | --- |
| d01 | 是 | 0 / 1 / 0 | typed只看value_start非NULL；真实PG typed_first的3项手写oracle失败。 |
| d02 | 是 | 0 / 1 / 0 | 先过滤string再取first；真实PG first_key首非string/laterstring对照失败。 |
| d03 | 是 | 0 / 1 / 0 | MCP误启用ExcludeArea；真实bootstrap的两种scope丢来源页。 |
| d04a | 是 | 0 / 1 / 0 | REST首遍套type条件使全局types计数收窄。 |
| d04b | 是 | 0 / 1 / 0 | 合法空父children提前返回；另有topic页与非空来源metadata。 |
| d04c | 是 | 0 / 1 / 0 | TagKey=false提前返回；topic count1/mode/source_area应保留。 |
| d04d | 是 | 0 / 1 / 0 | 空TagPages提前返回；同上非空metadata oracle。 |
| d04e | 是 | 0 / 1 / 0 | 精确type零匹配清空metadata；同上非空oracle。 |
| d04f | 是 | 0 / 1 / 0 | cursor过尾清空metadata；同上非空oracle。 |
| d05 | 是 | 0 / 1 / 0 | 按截断/case折叠key合并exact值；A/a及同128前缀不同尾的2/3计数与选择失败。 |
| d06 | 是 | 0 / 1 / 0 | 把1024 UTF8字节校验误改为rune计数；1025B多字节过滤值应422。 |
| d07 | 是 | 0 / 1 / 0 | 忽略types实际JSON 32KiB；8个转义长值只能保留5，额外第6个实序列化超界。 |
| d08 | 是 | 0 / 1 / 0 | 强制complete=true；项数/字节/long分别被断言捕获，有限513输入两批及后批错误亦执行。 |
| d09 | 是 | 0 / 1 / 0 | Access提前至cursor/filter；原错误优先次序断言失败。 |
| d10 | 是 | 0 / 1 / 0 | REST反转共用顺序；4个明确ID oracle失败，MCP无catalogue调用对照。 |
| d11 | 是 | 0 / 1 / 0 | catalogue adapter将FFFD改回NUL；真实Markdown YAML转义\0→存储FFFD→HTTP catalogue/filter链失败。 |

## S1-3

| 控制 | 最终纳入 | 实际退出 | 记录的业务首败/限定 |
| --- | --- | --- | --- |
| f01-label | 是 | 0 / 1 / 0 | missing safe fragment |
| f02-destination | 是 | 0 / 1 / 0 | unsafe destination |
| f03-newline | 是 | 0 / 1 / 0 | multiline display injection |
| f04-escaped-budget | 是 | 0 / 1 / 0 | escaped file exceeds 40KiB |
| f05-footer | 是 | 0 / 1 / 0 | actual footer or whole item differs |
| f06-source | 是 | 0 / 1 / 0 | wired source membership/count |
| f07-percent20 | 否，保留历史 | 0 / 1 / 0 | resolves to；Historical fixture has an illegal trailing-space page title; superseded only for accepted F07 proof by f07-percent20-internal-space. |
| f07-percent20-internal-space | 是 | 0 / 1 / 0 | resolves to |
| f07-percent41 | 是 | 0 / 1 / 0 | resolves to |
| f08-export | 是 | 0 / 1 / 0 | encoded output exceeded budget |
| f08-read | 是 | 0 / 1 / 0 | encoded output exceeded budget |
| r-s1-3-f1-entity | 是 | 0 / 1 / 0 | rendered entity link |
| v11-read | 是 | 0 / 1 / 0 | virtual read identity |
| v12-renderer | 否，保留历史 | 0 / 1 / 0 | hand-written header window；Historical pre-amp-encoding V12 proof; final selected proof uses -amp. |
| v12-renderer-amp | 是 | 0 / 1 / 0 | hand-written header window |
| v12-synthetic | 否，保留历史 | 0 / 1 / 0 | long line lost byte continuation；Original collector expected a later assertion. Exact actual SDK internal_error business failure is classified in v12-synthetic-actual-oracle-01.json. |
| v12-synthetic-amp | 是 | 0 / 1 / 0 | synthetic SDK read: <nil> |
| v13-fresh | 是 | 0 / 1 / 0 | did not regenerate after a real write |
| v14-dependency | 是 | 0 / 1 / 0 | Tools accepted missing Virtual |
| v15-normal | 是 | 0 / 1 / 0 | ordinary read gained a snapshot |
| v16-rr | 是 | 0 / 1 / 0 | one read mixed paths across the rename |
| v17-mode | 是 | 0 / 1 / 0 | mode recheck did not fall back |
| v17-outer-auth | 是 | 0 / 1 / 0 | fallback did not reauthorize on outer context |
| v18-visibility | 是 | 0 / 1 / 0 | is no tool error |
| v19-nested | 是 | 0 / 1 / 0 | generator nested its normal snapshot |

## S1-4

| 控制 | 最终纳入 | 实际退出 | 记录的业务首败/限定 |
| --- | --- | --- | --- |
| e01-wire | 是 | 0 / 1 / 0 | whole LLM ZIP missing virtual index.md or its real Guide destination: "" |
| e02-standard | 是 | 0 / 1 / 0 | export failed: {ID:01a1977b-00ac-75d4-944b-cc8e2cc01b53 State:failed Name:Eng RootID:<nil> CreatedBy:{DisplayName:alice} Progress:{Done:0 Total:2} Report:0xc000a5c120 Download:<nil> Problems:[]} |
| e02-subtree | 是 | 0 / 1 / 0 | export failed: {ID:01a1977a-7820-75cd-bbbf-d1023bb4a308 State:failed Name:Parent RootID:0xc001874d00 CreatedBy:{DisplayName:alice} Progress:{Done:0 Total:2} Report:0xc001874cd0 Download:<nil> Problems:[]} |
| e03-mode-snapshot | 是 | 0 / 1 / 0 | snapshot index.md mixed paths/mode with concurrent downgrade: "" |
| e03-path-snapshot | 是 | 0 / 1 / 0 | `snapshot index.md mixed paths/mode with concurrent rename: "# Index\n\nSource area: [raw.md](<raw.md>) — 0 sources. Use pending_sources or lint for status.\n\n## Parent: /\n\n- [AGENTS](<AGENTS.md>); changed: 2026-11-01T14:20:17Z\n- [AfterAlpha](<BeforeAlpha.md>); changed: 2026-11-01T14:20:17Z\n- [raw](<raw.md>); changed: 2026-11-01T14:20:17Z\n\nShowing 3 of 3 pages. Use index with type, tag or parent to filter and page through more.\n"` |
| e04-inbound | 是 | 0 / 1 / 0 | export executed the read-only inbound profile's SQL |
| e05-nfc | 是 | 0 / 1 / 0 | contributed matching was not exact NFC: ["café 2:case-distinct real page" "Café:contributed NFC spelling" "Café:nested real page"] |
| e05-real-meta | 是 | 0 / 1 / 0 | real contributed ZIP became nodes: ["AGENTS" "AGENTS 2" "Guide" "index 2" "log 2" "raw" "raw 2"], want ["AGENTS" "AGENTS 2" "Guide" "raw" "raw 2"] |
| e06-conflict | 是 | 0 / 1 / 0 | reserved root folder conflict was swallowed or overwritten: {ID:01a1977f-4230-72eb-8cf3-d69dd27119e4 State:succeeded Name:LegacyWiki RootID:<nil> CreatedBy:{DisplayName:alice} Progress:{Done:4 Total:4} Report:0xc00126da70 Download:0xc00126da60 Problems:[]} |
| e06-no-meta | 是 | 0 / 1 / 0 | absent llm fallback skipped or misnumbered the real files: [], want ["index 2:fallback index" "log 2:fallback log"] |
| e06-oversize-first | 是 | 0 / 1 / 0 | oversize_first llm fallback skipped or misnumbered the real files: [], want ["index 2:fallback index" "log 2:fallback log"] |
| e07-folders | 是 | 0 / 1 / 0 | `folder-only index/source destinations are not non-links: "# Index\n\nSource area: [raw/](<raw/>) — 1 sources. Use pending_sources or lint for status.\n\n## Parent: /\n\n- [A](<A/>); changed: 2026-11-01T14:24:48Z\n- [AGENTS](<AGENTS.md>); changed: 2026-11-01T14:24:48Z\n- [raw](<raw/>); changed: 2026-11-01T14:24:48Z\n## Parent: A\n\n- [B](<A/B.md>); changed: 2026-11-01T14:24:48Z\n\nShowing 4 of 4 pages. Use index with type, tag or parent to filter and page through more.\n"` |
| e08-plan-paths | 是 | 0 / 1 / 0 | `index.md did not link to the actual renamed ZIP entry: "# Index\n\nSource area: [raw.md](<raw.md>) — 0 sources. Use pending_sources or lint for status.\n\n## Parent: /\n\n- [AGENTS](<AGENTS.md>); changed: 2026-11-01T14:25:23Z\n- [N](<N.md>); changed: 2026-11-01T14:25:23Z\n- [N.md](<N.md.md>); changed: 2026-11-01T14:25:23Z\n- [raw](<raw.md>); changed: 2026-11-01T14:25:23Z\n## Parent: N.md\n\n- [c](<N.md/c.md>); changed: 2026-11-01T14:25:23Z\n\nShowing 5 of 5 pages. Use index w…（完整原文见control-accounting.json）` |
| e09-absolute | 否，保留历史 | 0 / 0 / 0 | 未命中目标断言；不得算有效kill |
| e09-duplicate-id | 是 | 0 / 1 / 0 | invalid export map duplicate_id reached generation before its mapping was rejected |
| e09-duplicate-path | 是 | 0 / 1 / 0 | invalid export map duplicate_path reached generation before its mapping was rejected |
| e09-empty | 是 | 0 / 1 / 0 | invalid export map empty reached generation before its mapping was rejected |
| e09-history-id | 是 | 0 / 1 / 0 | history with a live mapped ID but no old Path lost its real ZIP destination |
| e09-history-missing | 是 | 0 / 1 / 0 | `missing historical node became a false link or vanished: "# Log\n\n- 2026-11-01T14:27:10Z 01a19781-d4ce-74f9-b535-9dbe2792b032 — edit; client: mcp:unknown; by: alice; nodes: 1; contents: 0; ingests: 0\n  - deleted page: [Gone](<Gone>)\n- 2026-11-01T14:27:10Z 01a19781-d4ab-70ec-befd-bd93a32c0770 — edit; client: web; by: alice; nodes: 1; contents: 1; ingests: 0\n  - created page: [Kept.md](<Kept.md>)\n- 2026-11-01T14:27:10Z 01a19781-d48d-71ff-b81a-a57350463e6b — edit; client: w…（完整原文见control-accounting.json）` |
| e09-index-path | 是 | 0 / 1 / 0 | `index.md did not link to the actual renamed ZIP entry: "# Index\n\nSource area: [raw.md](<raw.md>) — 0 sources. Use pending_sources or lint for status.\n\n## Parent: /\n\n- AGENTS; changed: 2026-11-01T14:25:49Z\n- N; changed: 2026-11-01T14:25:49Z\n- N.md; changed: 2026-11-01T14:25:49Z\n- raw; changed: 2026-11-01T14:25:49Z\n## Parent: N.md 2\n\n- c; changed: 2026-11-01T14:25:49Z\n\nShowing 5 of 5 pages. Use index with type, tag or parent to filter and page through more.\n"` |
| e09-missing-live | 是 | 0 / 1 / 0 | invalid export map missing_live reached generation before its mapping was rejected |
| e09-source-path | 是 | 0 / 1 / 0 | `source-area consumer ignored its actual folder mapping: "# Index\n\nSource area:  — 1 sources. Use pending_sources or lint for status.\n\n## Parent: /\n\n- [AGENTS](<AGENTS.md>); changed: 2026-11-01T14:26:16Z\n- [N](<N.md>); changed: 2026-11-01T14:26:16Z\n- N.md; changed: 2026-11-01T14:26:16Z\n\nShowing 3 of 3 pages. Use index with type, tag or parent to filter and page through more.\n"` |
| e09-unknown-id | 是 | 0 / 1 / 0 | invalid export map unknown_id reached generation before its mapping was rejected |
| e09-wrong-kind | 是 | 0 / 1 / 0 | invalid export map wrong_kind reached generation before its mapping was rejected |
| e09-absolute-02 | 是 | 0 / 1 / 0 | invalid export map absolute reached generation before its mapping was rejected |
| e09-asset-folder | 是 | 0 / 1 / 0 | invalid export map asset_folder reached generation before its mapping was rejected |
| e09-backslash | 是 | 0 / 1 / 0 | invalid export map backslash reached generation before its mapping was rejected |
| e09-control | 是 | 0 / 1 / 0 | invalid export map control reached generation before its mapping was rejected |
| e09-dot | 是 | 0 / 1 / 0 | invalid export map dot reached generation before its mapping was rejected |
| e09-dotdot | 是 | 0 / 1 / 0 | invalid export map dotdot reached generation before its mapping was rejected |
| e09-invalid-utf8 | 是 | 0 / 1 / 0 | invalid export map invalid_utf8 reached generation before its mapping was rejected |
| e09-line-separator | 是 | 0 / 1 / 0 | invalid export map line_separator reached generation before its mapping was rejected |
| f07-entities | 是 | 0 / 1 / 0 | index.md collapsed identity "x&amp;Y.md" onto another real ZIP entry |
| f07-percent | 是 | 0 / 1 / 0 | index.md collapsed identity "x%41.md" onto another real ZIP entry |
| f08-encoded-budget | 否，保留历史 | 0 / 0 / 0 | 未命中目标断言；不得算有效kill |
| f07-entity-slash | 是 | 0 / 1 / 0 | index.md collapsed identity "folder&sol;leaf.md" onto another real ZIP entry |
| f07-percent-space | 是 | 0 / 1 / 0 | index.md collapsed identity "x%20y.md" onto another real ZIP entry |
| f08-encoded-budget-02 | 否，保留历史 | 0 / 1 / 0 | the job 01a19790-9e81-78ff-ade7-7aeac4068941 is still running |
| f08-next-item-boundary | 是 | 0 / 1 / 0 | encoded index.md crossed its real ZIP byte budget or lost its footer: 2097199 |
| first-changes-items | 是 | 0 / 1 / 0 | actual FirstChanges read outside fixture-shaped physical bound 20000 through changeset_items: {Scans:1 Returned:300002 Fetched:0 Blocks:5952 Hits:1481} |
| first-changes-revisions | 是 | 0 / 1 / 0 | actual FirstChanges read outside fixture-shaped physical bound 20000 through page_revisions: {Scans:1 Returned:300000 Fetched:0 Blocks:8610 Hits:1641} |

S1-1 V04为既有守卫回归、V11只含list腿；S1-3 V11补read腿。S1-4两个FirstChanges JOIN控制是额外生产计划发现的独立腿。F07/F08的renderer与真实ZIP腿分层保存，不相互替代。S1-4的absolute旧fault为重叠防御等效未杀；原next-item F08是夹具未触界的真实未杀，不能标等效；whole-buffer O(n²)超时不是业务kill。
