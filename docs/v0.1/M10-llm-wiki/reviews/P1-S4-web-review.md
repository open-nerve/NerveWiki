# M10/P1 S4（网页的模式与降级）：审查记录



| 项 | 内容 |
|---|---|
| 对象 | 起点 `f2fe618040945923c5d32fb438d78632645210fd`，原始完整审查头 `a0aa88dd647e7579022b6c0e5b54ed22c8a0c8ff`，最终受测修复头 `13ee0bc8ab4a302d654aa58ad994cbc8cedd6c24`；23个代码/测试文件。前六个提交依次 `ab3bd0eb`、`35804c04`、`d83cd89a`、`8e4c1899`、`fa662978`、`a0aa88dd`，第七个 `13ee0bc8` 只补B-I1六个参数化测试实例。分支文档 `8556ad07`；2026-10-27 合并为 `f261952f757bb486908210955f3b679a407b71e7`，父提交 `f2fe6180`、`8556ad07`，合并树与分支相同；见 [00-M10](../00-M10-design.md)第 12 节。 |
| 依据 | [01-P1](../01-P1-contract-frame.md) 3.29 的 S4 行、4.4、第 5 节、6.4/6.5、7.3 的 54–64、7.5–7.6、第 8 节；[工作流](../../claude_to_codex/02-workflow.md) 3.1–3.7；S3 的 [决定 33–34](P1-S3-prechecks-refusals-numbering-review.md)。 |
| 日期 | 2026-10-27 |
| 结论 | 三份原始完整 Step 独立审查及修复后的范围核对已由主持者接受；B-I1（Important，测试覆盖）已关闭，没有剩余核实的 Important、Minor、Nit 或 Q。原始审查头为a0，最终接受头为13ee；B由原审查者重放核对，A/C由主持者按工作流3.4的测试修复例外核对未变范围，记录分别保存。 修复候选 `13ee0bc8` 的13项门禁范围已覆盖：本头新跑 static-build、web、E2E 三项（合计246.659s），网页166文件/3,415测试、E2E291测试、22条已知TimeoutNaNWarning；十项Go命令保留实际受测头 `a0aa88dd` 的通过记录，以唯一改动为51行前端测试、增加六个参数化实例的完整输入等价证明复用，未声称重跑。 最终修复候选 `13ee0bc8ab4a302d654aa58ad994cbc8cedd6c24` 的 [分支 CI 40362256209](https://github.com/open-nerve/NerveWiki/actions/runs/40362256209) 十个作业全部成功；bootstrap 1/2/3 的Test步为370/391/360s（作业392/415/383s），E2E步190s（作业247s）。其他Test步/作业为web214/250s、short182/201s、race624/653s、costs532/714s、guards770/799s；Image smoke为73s（作业81s）。原始 `s4-evidence/branch-ci-13ee0bc8/{run,jobs,timings}.json` 的run、head、branch、push事件、workflow、十个job与计时逐项核对，B另独立核对同一CI。该分支运行不能代替未来合并后的两个main样本。 S4 已合并，两次完整 main CI 及 Test 步平均已核对；P1 完成见 [P1 10.5](../01-P1-contract-frame.md)。 |

## 建成的行为与任务核对

树的 AGENTS 以根下规范化标题、来源根以 id 判定；不能拖的行不登记 draggable，但保留 drop target；同一 DOM 行在降级后随 canDragRow 变化重新登记。种类图标为 aria-hidden，名字后有可访问后缀，真实空格在 sr-only 外。创建以原生模式单选发送 mode 与用户偏好的 template_locale，新手引导不变。降级按笔记本排队、put/changed 防旧读覆盖；只有三个指定问题码重读列表，busy/网络/无码错误不重读；看不到的拿出但不记删除，读失败保留原拒绝。持有的对话框保留403，成功或按钮消失后的焦点落到 Mode。

真实 mode 表项只使对应工作区的笔记本列表重读；应用测试在连接已安定时核对恰好一个 GET notebooks、工作区与锁日志为空、流身份不变。LW1 两种语言页面、LW2 页面、LW12 API/页面共五个新增故事（P1 共九个）。LW12 用普通 worker 的唯一导入工人、B 的 FOR SHARE 锁与 A 的持久 queued 状态证明 busy；释放锁后 drain 已返回的任务，保留原错误。成功的帧/standard-null/删除均核对；独立上下文的另一流更新不靠重载、导航或重连。

任务1–5各有实施报告、命令日志和提交后哈希对应的 committed-proof；主持者读过各任务范围的实际 diff 与证据。它们是任务范围核对，不代替以下三位全 Step 独立审查。代码不改服务端、SQL、API、依赖或工作流；每提交的 Go 证据复用基于与607e761e相同的输入，不免除最终完整 Step 门禁。记录在树外 `s4-evidence/task1` 至 `task5` 及 `s4-entry.json`、`s4-code-facts.json`。

## 主持者发现与修复核对

| 编号 | 严重性 | 发现 / 处置 | 提交与核对 |
|---|---|---|---|
| R-M1 | Minor | Task2 creationServer 从 notebookJSON 继承 null 根，只回显 llm_wiki；真实创建一定有来源根。单行给 LLM Wiki 非零 UUID、普通创建仍 null，未增加完整模板模拟或生产逻辑。没有证明原来掩盖了产品行为回归。 | `a0aa88dd`；已有20项创建/导航测试3.336s、lint-web2.504s通过，主持者复查；`s4-root-r-m1-repair.md`、`s4-evidence/root-r-m1/committed-proof.json`。没有新产品决定或新增编号变异。 |

## 完整 Step 的独立审查

三位审查者从 main `f2fe6180` 到 `a0aa88dd` 的全部 23 文件差异入手，在形成源码判断后才读作者证据；各用独立 checkout，重命令经机器共用锁串行执行。原报告保存实际审查头，不因后续补测试而重写。

| 审查 | 原始完整 Step 结论与实际核对 | 原始记录 |
|---|---|---|
| A | 没有 Important/Minor/Nit/Q。五个文件128项定向测试；真库/Chromium九个现有 wiki 故事加树探针共10项，另有原生创建控件探针。根下 AGENTS 分类、来源根按 id、受保护行仍作 drop target 与语言偏好均核对。 | `s4-full-review-a.md`、`s4-review-evidence/a/` |
| B | B-I1 Important：53个已提交 store/设置实例缺少“拒绝后重读完成，再回来更旧读”的交错。生产源码本来正确；只跳过 reconciliation 的 changed 计数时，包含原53项与7个外部探针的一次组合运行是60通过→6失败/54通过→60通过，原53项全部存活。这不是另跑了一次53项命令。真实403/409与持有的对话框另有真库/Chromium核对。 | `s4-full-review-b.md`、`s4-review-evidence/b/proof.json`、`B-X3.diff` |
| C | 没有 Important/Minor/Nit/Q。真 mode 注册的独立删除被应用测试与真浏览器最后一跳抓到；C-X2 实际故障注入核对锁释放、两个真实任务 drain、SQL 两行 succeeded 与主错误保留。逐提交原始证据、a0的13项门禁与CI亦核对，没有声称重新跑了全部重门禁。 | `s4-full-review-c.md`、`s4-review-evidence/c/` |

B-I1 只补一张包含六个参数化测试实例的表：三个指定拒绝码 × 目标仍在/已不在列表，先让 reconciliation 完成再释放旧读，并核对目标字段或缺席、无关笔记本、removed 与原拒绝语义。`13ee0bc8` 仅在 `notebook.store.test.ts` 加51行；生产源码、API与其余测试输入都未改。修复后的完整门禁覆盖、15变体重放、精确分支CI与最终头修复复核分开记录，不能由原始绿色推出关闭。

修复处置：`13ee0bc8` 只在 `notebook.store.test.ts` 新增51行、六个参数化测试实例，生产代码原本正确且未改。原审查者B在此精确头以同一命令重放原B-X3：59通过（3.327s）→恰六个新实例断言失败、原53个通过（3.375s）→原字节恢复后59通过（3.369s）；全部是已提交的永久测试，没有scratch实例或重试，最终树干净。主持者读完整报告与原始证明后关闭B-I1；`s4-full-review-b-fix1.md`、`s4-review-evidence/b/fix1-prepared/proof.json` 与 `s4-root-final-review-acceptance.json` 保存处置。

最终头复核（B由原审查者核对；A/C由主持者按工作流3.4测试修复例外核对输入等价，不声称A/C亲自重跑修复头）：

| 原始审查范围 | 最终头的核对者与依据 | 处置 |
|---|---|---|
| A：树、创建、语言与可访问性 | 主持者（`root (test-only repair)`），按工作流3.4的测试修复例外，核对唯一51行测试差异及整棵其余输入相同；永久89和原树/语言/可访问性变体已在13ee重放。原A报告及外部探针保持a0归属。 | 接受，零未结发现 |
| B：降级store/设置与B-I1 | 原审查者B亲自于13ee重放B-X3，59→6新失败/53旧通过→59，六个新实例全部运行断言失败；主持者核对报告/原证据后关闭B-I1。 | 接受，零未结发现 |
| C：mode登记、LW12、清理与证据 | 主持者（`root (test-only repair)`），按同一例外核对生产/E2E/fixture字节不变、最终新E2E与变体54。原C真浏览器最后一跳与C-X2故障注入仍归属a0。 | 接受，零未结发现 |

A/C并未亲自重跑13ee，也未重写原始完整Step报告；其最终范围核对在 `s4-root-fix1-review.md`，B的独立复核在 `s4-full-review-b-fix1.md`，全局接受在 `s4-root-final-review-acceptance.json`。这些修复核对与原始三份完整Step审查分别保留，不冒充三份新全量审查。

## 主持者的决定（负责人可以改判）

本 Step 没有新增产品决定。B-I1 的六个参数化测试实例补足属于已批准契约的验证修复；完整输入等价证明支持十项未变Go命令的旧头记录复用。修复头上的三项新门禁、15变体重放、B的原审查者复核、主持者按工作流3.4测试修复例外对A/C范围的核对，以及十作业分支CI均已完成并核对。

S3 决定33–34继续有效：全部三个main运行及失败已在本次S4文档转写，S3审查的原决定文字不改。IndexItems与grep稳定性仍是M10收尾义务。本轮没有新增产品决定；当时给下一条 P1 实施记录保留编号 35。P1 完成后的候选预算维护在同一实施与维护序列使用决定 35（[维护审查](grep-candidates-budget-review.md)），下一条实施记录为 36；这一序列与 00-M10 第 11 节的设计审查编号 1–40 独立。

## 实际安排与接受的范围

1. LW1只对本工作区notebooks端点加首次导航前的route.continue透传观察，不改原请求/正文/答复或精确postDataJSON断言。先前读到null是观测事实，不能把认证clone断言为已证实原因；窄安排由主持者接受。
2. notebookReads独立计读，不改旧sent语义；额外store控制覆盖无码HTTP、重读失败及看不到时不标删除。mode测试额外核对既有流与无关读。LW12清理保留原错误；C-X2实际故障注入证明锁先释放、两个真实任务drain到succeeded、清理错误注记而主错误仍报告。
3. jsdom核对literal textContent空格，Chromium核对确切可读名字，两者不能互代；62先等普通Actions按钮避免提前卡在装饰名字查询。57/63按实际两种变体记录。
4. 没有实现P2来源只读/去重、P3健康度、P4虚拟行或M11升级；确认文字按已批准设计描述M10完成后的后果，未以这些未来能力作P1已验证的结论。

## 反向对照

原始候选a0aa88dd的完整重放（修复前实际历史）：13个逻辑变体、20次指定永久测试catcher。每个相同命令在原样通过、改坏后运行时断言失败、外部备份恢复原字节后通过；63a/b各重新构建并在LW1英文、LW1中文、LW2三条都断言失败。多角色或多故事catcher不另算逻辑变体。

| 编号 | 改坏的行为 | 指定永久测试 / catcher | pristine / mutant / restored |
|---|---|---|---|
| 54 | 实际表去掉 mode | a mode event reads the workspace's notebooks again, through the app's handlers | 0 / 1 / 0；恢复原字节 |
| 55 | 绕过 inTurn | a downgrade waits for the same notebook's renaming | 0 / 1 / 0；恢复原字节 |
| 56 | not_llm_wiki 重读后仍拒绝 | a downgrade already done reads the notebook again and resolves | 0 / 1 / 0；恢复原字节 |
| 57a | 成功答复不 put | a read answered after a downgrade keeps it | 0 / 1 / 0；恢复原字节 |
| 57b | put 不计 changed | a read answered after a downgrade keeps it | 0 / 1 / 0；恢复原字节 |
| 58 | 对话框随管理员按钮卸下 | a downgrade forbidden after losing admin keeps its dialog, then focuses Mode when cancelled | 0 / 1 / 0；恢复原字节 |
| 59 | 语言取 navigator | a Chinese preference creates an LLM Wiki with its Chinese template | 0 / 1 / 0；恢复原字节 |
| 60 | AGENTS 菜单留下 Delete | an LLM Wiki's editor has only New subpage on AGENTS, whose row does not drag；an LLM Wiki's admin has only New subpage on AGENTS, whose row does not drag | 0 / 1 / 0；恢复原字节 |
| 61 | 受保护行仍登记 draggable | an LLM Wiki's editor has only New subpage on AGENTS, whose row does not drag；an LLM Wiki's admin has only New subpage on AGENTS, whose row does not drag；an LLM Wiki's editor has only New subpage on raw, whose row does not drag | 0 / 1 / 0；恢复原字节 |
| 62 | 去掉 sr-only 外真实空格 | an LLM Wiki's tree gives its two kinds a text equivalent after a space | 0 / 1 / 0；恢复原字节 |
| 63a | 后缀 aria-hidden | LW1 (page): an English LLM Wiki starts with its source area and instructions, named in the tree；LW1 (page): a Chinese LLM Wiki starts with Chinese instructions and its template's history；LW2 (page): an LLM Wiki's tree protects AGENTS and its source root; an admin renames the source root, while index stays reserved | 0 / 1 / 0；恢复原字节 |
| 63b | 后缀 hidden | LW1 (page): an English LLM Wiki starts with its source area and instructions, named in the tree；LW1 (page): a Chinese LLM Wiki starts with Chinese instructions and its template's history；LW2 (page): an LLM Wiki's tree protects AGENTS and its source root; an admin renames the source root, while index stays reserved | 0 / 1 / 0；恢复原字节 |
| 64 | effect 不依赖 canDragRow | a downgrade makes the same mounted AGENTS and source rows draggable | 0 / 1 / 0；恢复原字节 |

汇总证据：`s4-final-mutations/final-a0aa88dd-replay1/proof.json`，head a0aa88dd，complete/executed/all_tracked_state_restored均为true；其各子proof包含实际命令、日志、时间、失败断言与原/恢复字节哈希。54–59和62/64各1次catcher，60有2次、61有3次，63a/b各3次，共20次。审查新增两个永久逻辑变体：89（A-X1，根下 AGENTS 限制，由已有测试抓到）与90（B-X3，拒绝 reconciliation 不计 changed，由新增六个参数化测试实例核对）。54–64原编号保持；最终15逻辑变体/27次指定 catcher 的重放结果另记。

独立审查的原始反向证据均发生在 `a0aa88dd`：A-X1 去掉根下 AGENTS 的 parent 条件，被已有 “only the root AGENTS is protected, by its normalized title” 运行断言抓到，追加为89。B-X3 保留 reconciliation 的列表过滤/排序及队列，只直接赋值而跳过 changesAnswered；仅外部六个参数化测试实例失败，暴露 B-I1，后补永久测试并追加为90。修复头的89/90正式重放与原13变体放在同一最终证据范围。

A-X2 是63a、B-X1是55、B-X2是57b、B-X4是58、C-X1是54的独立重复；C-X1还给54增加真浏览器最后一跳证明，不加逻辑编号或永久指定 catcher 数。A-X3 的 drop-target 改坏由外部树探针抓到，没有运行全部已提交测试去判断它是否存活；原生控件探针也是外部核对。B的真实403/409与队列占用探针、C-X2清理故障注入单列为附加证据，不算新的永久产品变体。

C-X2 保留原 LW12 的 try/catch/finally/allSettled 结构，在 verified busy 后注入主错误，首个真实 endedJob 完成后注入清理错误。同一外部 scratch 故事按无故障/故障/无故障退出0/1/0；trace是 released→两个drain-start→两个drain-done:succeeded，独立SQL恰好两行且都succeeded。清理错误写入annotation，Playwright失败仍为主错误。原LW12源文件未改，scratch已删除；`s4-review-evidence/c/c-x2-fault-proof.json` 与恢复证明保留。这是实际故障核对，不是产品变异。

修复候选的完整重放：精确修复头 `13ee0bc8ab4a302d654aa58ad994cbc8cedd6c24` 已重新执行54–64（57a/b、63a/b）与89、90，共15个逻辑变体、27次指定永久catcher；逐项为原样0→运行断言失败1→恢复0，全部受跟踪文件恢复原字节。第89条由已有根下AGENTS测试抓到，第90条的六个永久实例都失败后恢复通过。原13条证明保留在 `s4-final-mutations/final-13ee0bc8/planned/`，89/90各有独立子proof，15个逻辑变体/27次catcher的完整汇总为 `s4-final-mutations/final-13ee0bc8/proof.json`；原a0重放与审查者外部探针不冒充这次重跑。

先前`final-a0aa88dd`运行在63a的未变异英文LW1基线停下：notebook-name小依赖的Chromium请求报net::ERR_NETWORK_CHANGED，初始页面模块装载就失败，尚未到创建或可读名字断言。workspace-layout自身200，另两个worker成功；浏览器/主机网络事件有时间关联，但没有NetLog/逐socket证据定位具体原因。原proof complete=false与trace保留，不计为13变体成功；经独立只读诊断，主持者只授权一次相同a0代码的完整重放，未改超时/重试/断言，得到上述完整证据。`network-baseline-diagnosis.md`保留边界与不确定性。此前任务开发中的类型/夹具错误、62 JSON解析器漏认和最初LW1请求体观察失败同样单列排除，不当作产品门禁通过或变异成功。

## 门禁、分份与 CI

原始候选的记录：

原始候选的完整门禁在 `a0aa88dd647e7579022b6c0e5b54ed22c8a0c8ff` 串行执行，`s4-evidence/full-gates-a0aa88dd/verified-summary.json` 保存 13 项 argv、head、退出码、耗时及日志 SHA-256。静态的 lint/gen-check/knip/build、race、bootstrap 三份、costs、guards、web、short、完整 E2E 均覆盖，三份另各测第二次；最后树干净。

| 门禁 / 命令范围 | 命令时间（s） | 结果 |
|---|---:|---|
| `static-build` | 26.85 | exit 0 |
| `race` | 330.183 | exit 0 |
| `test-go-bootstrap-1` | 306.375 | exit 0 |
| `test-go-bootstrap-2` | 275.769 | exit 0 |
| `test-go-bootstrap-3` | 272.838 | exit 0 |
| `costs` | 319.387 | exit 0 |
| `guards` | 444.74 | exit 0 |
| `web` | 102.067 | exit 0 |
| `short` | 56.668 | exit 0 |
| `e2e` | 141.038 | exit 0 |
| `test-go-bootstrap-1-second` | 308.088 | exit 0 |
| `test-go-bootstrap-2-second` | 274.306 | exit 0 |
| `test-go-bootstrap-3-second` | 269.363 | exit 0 |

命令总计 3127.672s；Vitest 166 文件/3,409 测试、完整 E2E 291 测试。最终 web 日志实数 22 条已知 TimeoutNaNWarning；构建/浏览器既有提示照日志保留，不把任务阶段数量推断成最终数量。这里报告的是读取已保存并核对哈希的实际门禁，文档起草者没有另跑测试。

原始候选 `a0aa88dd647e7579022b6c0e5b54ed22c8a0c8ff` 的 [分支 CI 40354593712](https://github.com/open-nerve/NerveWiki/actions/runs/40354593712) 十个作业全部成功；bootstrap 1/2/3 的 Test 步为378/370/322s（作业402/396/343s），E2E步232s（作业303s）。`s4-evidence/branch-ci-a0aa88dd/{run,jobs,timings}.json` 保存原始记录。这是修复前的实际历史，不能代替 `13ee0bc8` 的新分支CI或合并后的两个main样本。

修复候选的覆盖记录（新跑三项与实际旧头的十项Go证据分别列）：

最终覆盖头为 `13ee0bc8ab4a302d654aa58ad994cbc8cedd6c24`，实际新执行与旧头复用分别如下。`s4-evidence/combined-gates-13ee0bc8/verified-summary.json` 保留每项实际head、argv、退出码、耗时、原日志与SHA-256；对应input-equivalence.json核对完整Git差异只有51行测试增加，排除此文件后整棵受跟踪树逐项相同，服务端Git树对象相同。起草脚本又独立计算完整差异/树哈希，并核对每份日志和十项复用记录与原记录逐字段一致。

| 门禁 | 实际受测头 | 来源 | 命令时间（s） | 结果 |
|---|---|---|---:|---|
| `static-build` | `13ee0bc8` | 新执行 | 8.049 | exit 0 |
| `race` | `a0aa88dd` | 原记录复用 | 330.183 | exit 0 |
| `test-go-bootstrap-1` | `a0aa88dd` | 原记录复用 | 306.375 | exit 0 |
| `test-go-bootstrap-2` | `a0aa88dd` | 原记录复用 | 275.769 | exit 0 |
| `test-go-bootstrap-3` | `a0aa88dd` | 原记录复用 | 272.838 | exit 0 |
| `costs` | `a0aa88dd` | 原记录复用 | 319.387 | exit 0 |
| `guards` | `a0aa88dd` | 原记录复用 | 444.74 | exit 0 |
| `web` | `13ee0bc8` | 新执行 | 100.737 | exit 0 |
| `short` | `a0aa88dd` | 原记录复用 | 56.668 | exit 0 |
| `e2e` | `13ee0bc8` | 新执行 | 137.873 | exit 0 |
| `test-go-bootstrap-1-second` | `a0aa88dd` | 原记录复用 | 308.088 | exit 0 |
| `test-go-bootstrap-2-second` | `a0aa88dd` | 原记录复用 | 274.306 | exit 0 |
| `test-go-bootstrap-3-second` | `a0aa88dd` | 原记录复用 | 269.363 | exit 0 |

三个新命令为8.049s、100.737s、137.873s，合计246.659s；网页166文件/3,415测试，E2E291测试。新web原始日志实数22条TimeoutNaNWarning。十项Go记录的实际执行头仍为a0，三份包时间与命令时间也保留原受测头；这份组合覆盖不是声称13ee重新执行了全部13个命令。最终状态干净，精确修复头的分支CI另记。

原始候选 `a0aa88dd` 的本机各份两次实测如下；修复头只在完整输入等价核对后复用这些读数，不声称新跑了一次Go门禁；Go 包与命令包装层分别记，不充作 GitHub CI 的 Test 步。S4 没有新增 bootstrap 测试，不能拿 S3 新测试的 7.335s 说成 S4 新测。

| 组合根 | Go 包两次（s） | 包平均（s） | 命令两次（s） | 命令平均（s） |
|---|---|---:|---|---:|
| test-go-bootstrap-1 | 300.355 / 307.297 | 303.826 | 306.375 / 308.088 | 307.2315 |
| test-go-bootstrap-2 | 274.975 / 273.525 | 274.25 | 275.769 / 274.306 | 275.0375 |
| test-go-bootstrap-3 | 272.018 / 268.567 | 270.2925 | 272.838 / 269.363 | 271.1005 |

S4 合并后的两次完整 main CI 均已通过，实际 Test/E2E、作业时间与各份平均见下方；7.5–7.6 的核对与 P1 完成记录见 [P1 10.5](../01-P1-contract-frame.md)。本机与分支读数不替代这两个 main 样本。

最终修复候选 `13ee0bc8ab4a302d654aa58ad994cbc8cedd6c24` 的 [分支 CI 40362256209](https://github.com/open-nerve/NerveWiki/actions/runs/40362256209) 十个作业全部成功；bootstrap 1/2/3 的Test步为370/391/360s（作业392/415/383s），E2E步190s（作业247s）。其他Test步/作业为web214/250s、short182/201s、race624/653s、costs532/714s、guards770/799s；Image smoke为73s（作业81s）。原始 `s4-evidence/branch-ci-13ee0bc8/{run,jobs,timings}.json` 的run、head、branch、push事件、workflow、十个job与计时逐项核对，B另独立核对同一CI。该分支运行不能代替未来合并后的两个main样本。

任务阶段的已知读数：Task1最终网页166文件/3368测试，Task2为3374，Task3为3408，Task4为3409；Task5 E2E的两条定向故事通过，复用输入相同的Task4网页证据；R-M1另核对20项。任务1–4完整网页日志各有已知22条TimeoutNaNWarning，构建有既有chunk提示；这不推断最终门禁的警告数量，最终数值以上方完整日志为准。Task5正常清理由通过的API故事覆盖；完整Step审查C-X2又实际注入主失败与清理失败，核对释放、drain和原错误保留。没有声称每位审查者或每个中间提交都重跑全 Step 门禁。

S3的三次postmerge记录和所有两两平均已转写到[01-P1 10.3](../01-P1-contract-frame.md)和[S3审查](P1-S3-prechecks-refusals-numbering-review.md)，保留失败40325348498。S4的样本另记如下。

## 合并与 main CI

S4 于 2026-10-27 合并为 `f261952f757bb486908210955f3b679a407b71e7`，第一父提交 `f2fe618040945923c5d32fb438d78632645210fd`，第二父提交为分支文档 `8556ad07576485e5ed6f902eb6137067ac324a4c`；合并树与该分支头完全相同。分支文档的 [CI 40367549675](https://github.com/open-nerve/NerveWiki/actions/runs/40367549675) 在精确头 `8556ad07576485e5ed6f902eb6137067ac324a4c` 上十个作业全部成功；bootstrap 1/2/3 的 Test 步为 393/384/247s（作业 415/405/269s），E2E 步 223s（作业 281s）。合并后的首次main的 [CI 40370315130](https://github.com/open-nerve/NerveWiki/actions/runs/40370315130) 在精确头 `f261952f757bb486908210955f3b679a407b71e7` 上十个作业全部成功；bootstrap 1/2/3 的 Test 步为 291/393/397s（作业 309/413/416s），E2E 步 229s（作业 280s）。合并记录文档 `767b69a6aeae4ada095de137bf0996083fae193e` 触发的第二次 main [CI 40374066614](https://github.com/open-nerve/NerveWiki/actions/runs/40374066614)（attempt 1）十个作业全部成功；bootstrap 1/2/3 的 Test 步为 420/387/334s（作业448/431/361s），E2E 步 192s（作业 738s）。 两次 bootstrap Test 步平均依次为 355.5/390/365.5s，三份均严格低于 450s（7:30）。 两次都是精确 main 头的正常 push CI（attempt 1/1），各十个作业全部成功；主持者核对原始 run/jobs/timings 与 postmerge-pair.json 后接受；完整作业时间及原始证据存于树外的 S4 postmerge 记录。 S3 的样本不充作 S4 样本。

| main 样本 | run / 精确 HEAD | bootstrap-1 Test / job | bootstrap-2 Test / job | bootstrap-3 Test / job | E2E step / job |
|---|---|---|---|---|---|
| 第一次 | `40370315130` / `f261952f757bb486908210955f3b679a407b71e7` | 291s / 309s | 393s / 413s | 397s / 416s | 229s / 280s |
| 第二次 | `40374066614` / `767b69a6aeae4ada095de137bf0996083fae193e` | 420s / 448s | 387s / 431s | 334s / 361s | 192s / 738s |
| 两次平均 | 仅比较 bootstrap 的 Test 步 | 355.5s | 390s | 365.5s | E2E 不设此平均门槛 |

## 接受与推后

P1 已完成，核对见 [P1 10.5](../01-P1-contract-frame.md)。[M5 事件移交](../handoffs/M5-events.md)已改为 done；M8/M9 的后续来源与系统说明事项保持 open。新建的 [M11 模式契约移交](../../M11-upgrade/handoffs/M10-mode.md)保持 open，只记录扩展点与契约，不开始 M11。M10 收尾继续核对 IndexItems 与 grep 的稳定性；本次没有改变成本阈值。
