# M8 搜索守卫参考校准（M10/P3 前维护）：审查记录

日期：2026-10-30。起点是 P3 设计提交 `9e38a38f648ffa19130afb8a8b468b792dad1381`；维护分支 `m10-search-guard-calibration`，代码提交 `ccac8c5d9e741721965e07c2b053f04dbe446e61`，父提交即起点。属于 M8 story、在 M10 发现的测试与注释维护；P3 设计已经接受，S1/S2仍待实施。

**当前结论：范围内实施、真实反向对照、七项本机 Go 门禁、独立代码审查、精确代码分支CI、文档分支CI及维护合并后的首次main CI已由主持者接受。** A/B 在各自规范与质量范围 PASS；C 第1轮保留当时的 `PENDING_ACTUAL_CODE_CI`，第2轮对实际代码 CI 的有界复核以规范/质量均 PASS 关闭该项，四份代码报告均无核实的 Important/Minor/Nit/Q。代码 CI `40913568907` attempt 1 已按精确head、11个作业和全部必需步骤验收；文档aa860be1 / CI40921122246 attempt1，以及no-ff合并b957fdbe / 首次main CI40925905708 attempt1亦已分别验收，见第7节。本记录自身提交/CI仍待产生；新增空名称属性链接来源设计缺口仍开放，决定62的具体修订与三位独立审查待完成，P3尚未放行。合并见00号文档第12节（[进度表](../00-M10-design.md#12-phase-进度表)），维护单列，不占 P3/S1 的合并位置。

## 1. 对象、方法与证据边界

按[推进方式](../../claude_to_codex/02-workflow.md) §3.4 的小轮测试/注释维护范围，由主持者限定变更、核对真实变异与 CI，另安排同模型的 A/B/C 独立分工。作者提交外部实施证据，文档起草者只提交树外精确替换；主持者负责应用、门禁、提交和集成。原始证据保留，不用后来结果改写原 attempt。

唯一代码差异是 `server/internal/modules/page/adapter/postgres/search_guard_test.go` 的 60 个参考数和三块来源/历史注释，38 行增加、43 行删除；目标文件 SHA-256 为 `3b5a87a6420524adaea6959a86174aee1c637349959cb90bb8800ac6ef168668`。去掉整行注释、只将 `msOf` 数值参数替为占位后，base/code 字节相同。20 行身份和顺序，包括 `中国 eng / building=false` 与 `中国 nil / building=true`，均保持。

未改 `msOf`、`searchLimit`、40 ms floor、97 ms trap、元测试断言、每格八次取中位数、fixture/阶段/游标/building、生产 SQL、build/QueueSearch 预算、超时或 CI 配方。没有 P3 产品实现。本轮未重跑无关的本机 Web/E2E 全套；过去 P2 的已验产品门禁保留其原受测对象。本代码分支、文档分支与维护合并后的首次main完整CI已分别按精确对象验收（第6–7节）；本记录自身提交/CI仍待产生，P3入口另受第7节所述来源设计修订与放行条件约束。

本文所称外部证据以 `search-cost-investigation/` 为根，以文件名和 SHA-256 标识；本次CI成本清单在上级M10证据根，另明确标出。它们不是仓库相对链接的依赖。实施原件 `calibration/implementation-report.md` SHA-256 `f5ad3bc1f1897c450954549715dba2a251c8db5758dab2aaac2e584e092a6d07`；初始文档提案 `calibration/documentation-note.md` SHA-256 `f2d5045b456db666b9fc3a2ff2148cf80eb7c863eac79f4a7298123c7f707fe9`。四次作者测试固定于提交前候选；其 3,202 项输入与稍后代码提交逐项相同，不把提交时间倒写成测试时 HEAD。

## 2. 原失败、历史计数与诊断审查

P3 设计的 [CI 40897835721](https://github.com/open-nerve/NerveWiki/actions/runs/40897835721) attempt 1，精确 head `9e38a38f648ffa19130afb8a8b468b792dad1381`，仍为 **FAILED、未验收**。11 作业是 8 success、costs failure、e2e/image skipped。原搜索 generic `go` 首页中位数为 **40.621258 ms > 40 ms**；八次中至少四次超过 40 ms，不能归结为一个最长值。costs 前八条命令完成，第九条搜索失败，后八条 **NOT_RUN**；缺失结果保留 null，不追认通过。

原失败清单 `failure-inventory-report.json` SHA-256 `c50cd2919df4b785c68faa63cd1532ebe9e6d51a0756e7b5af065cfb87e19d99` 保存 13 costs + 6 guards 的已取得指标；缺少 `grep_glob`，不是完整 20 项成本验收。正常解析器的 14 costs/6 guards 完整性要求没有放宽。此前范围清单累计 **47 个 observed logical runs，包含这次失败**；不是 47 次成功或完整运行。同一 run 的新 attempt 也不能再算一个 logical run。

搜索历史原日志另有 39 份匹配记录，含 **38 份此前记录 + 本次失败**；此前 generic `go` 最大打印中位数 35.012 ms。这个集合不是完整 GitHub 历史，也不是失败率估计；“47 个逻辑运行”和“39 份匹配搜索日志”分别计数。

校准前的独立诊断审查 `diagnostic-review.md`（SHA-256 `4a50511aa9988ce9f48e9a778b16e3653578600fa938fcacbac353f47e2a2ce2`）给出有条件的整表校准建议，CI 成因结论仍为 inconclusive。以下是该轮发现及主持者决定61的处置，**不冒称本轮代码 A/B/C 新报了这些缺陷**：

| 原诊断项 | 已核实的事实或问题 | 决定61的处置 |
| --- | --- | --- |
| I-1 Important | 换参考使 39/60 个有效时限增大，不能声称旧敏感度全保留 | 接受并公开检测取舍；全表逐格采用三轮数，不统一乘倍率、不单抬 `go` |
| I-2 Important | 原失败及后八项未运行不能被新界追认为通过 | 保留 FAILED/NOT_RUN/skipped/null，后续 CI 单独验收 |
| M-1 Minor | 本机 generic 额外成本与计划差异不等于 hosted 失败现场的归因 | 保留已观测事实，不改 SQL，不称产品根因已修复或噪声已证实 |
| M-2 Minor | 39 份日志包含这次失败 | 明记 38 + 1，与 47 个逻辑运行分开，不推算故障率 |
| Nit | 原 Mac 来源和“最慢”等现在时注释不适用于新表 | 更新 Linux 来源与当前表注释；旧 Mac/P5A 全表和历史保留 |
| Q | 失败 hosted AMD64 上额外成本究竟来自哪里，当前本机证据未回答 | 作为明确证据限制接受；按下节触发条件决定下一次有目标的观测 |

## 3. 主持者的决定（负责人可以改判）61

决定沿 M10 的**实施序列**接49–60，不覆盖总设计审查1–40；原裁定 `calibration/root-decision-61.md` SHA-256 `3ac20b19c8ff5dad51bee73064f40743a2330c8c230ae5818055b4f8e803bfa5`。

采用 [M8/P5 §3.9](../../M8-history-search/05-P5-search.md) 的完整20读×3阶段重测方法：每格八次中位数，三个原运行再取中位数。来源为未改的 `9e38a38f...` 上三轮 Linux ARM64 / Go 1.27.1，原日志显示 Ubuntu 26.04 LTS / Docker 29.9.2；瞬时负载未记录。180 个源值按阶段、原表行号逐项核对；原日志先舍入到微秒，再将三值中位数舍入为一位小数 ms。汇总 `three-run-search-remeasurement.json` SHA-256 `df2f53e841107739b301c7004a90cfcfcf4bd21ae0ce82c446437a1891185c42` 的旧 `MEASURED_CANDIDATE_NOT_ADOPTED` 状态保留为历史，本决定授权采用而不覆盖旧原件。

完整新表、原命令及三轮 run/log 哈希只列在 M8/P5 §3.9；全60格新旧实际限值在实施原件。公式仍为 `max(3 × median, 40 ms)`。**39 格升高、21 格不变、0 格下降**；`go` 首页为 **60.3 / 60.6 / 62.7 ms**，未来 generic 中位数落在 **(40, 62.7] ms** 不再被此计时比较拒绝。这是接受的检测取舍。原行7、8、11、12、14的五个无统计陷阱格仍均为 **40 ms < 97 ms**。原 `msOf` 的浮点乘法/整数截断使新16.4 ms实际为16,399,999 ns，时限49,199,997 ns；展示49.2 ms不改变执行语义。

本机 generic 额外约2 ms及真实计划差异已经观测；失败 hosted AMD64 的额外成本成因仍未证实。当前参考维护不扩成产品 SQL 修改，不验收300 ms/30万页产品目标。若校准后再次越界或出现新的规模异常，停止沿用这次处置，按新证据做**一次有目标的 hosted 观测**，保存原样本、实际调用规模及计划；不继续重标或重跑挑绿。[M12 性能移交第17项](../../M12-release/handoffs/M4-performance.md)继续 open，保留目标硬件、真实语料、尾延迟、规模与并发义务。

P3/S1仍未放行。维护经完整分支验收、按工作流集成且后续精确 main 自己的完整 CI 成功并被接受后，主持者重新绑定 P3 放行回执与 S1-0 候选。原 `9e38...` 的失败 attempt 不要求、也不可能由这次维护改写为成功；P3 冻结设计49–60、容量阈值与全部实现准入条件继续适用。没有 M11 工作。

## 4. 实际验证与反向对照

四次作者运行串行使用同一 `heavy.sh` 锁、`GOFLAGS=-p=1` 和显式 `-p=1`。从仓库根运行的 race 元测试命令在 baseline/mutant/restored 三轮完全相同：

```sh
docs/v0.1/claude_to_codex/tools/heavy.sh env GOFLAGS=-p=1 go -C server test -p=1 -race -count=1 -json -timeout 10m -run '^TestTheSearchGuardsLimitsKeepItsTrapCaught$' ./internal/modules/page/adapter/postgres
```

负控只将第一条 `the` 的无统计参考 **10.5→33.0 ms**，使限值为99 ms，未改原元测试/断言。实际报 `"the" page 1 without statistics: limit 99ms, want under the trap's 97ms`，唯一断言失败、exit 1；不是编译失败、超时或 race error。恢复备份后目标 SHA 回到上述候选，原命令通过。此负控验证参考的数值分离，不声称重演历史坏 SQL。

完整非 race 读守卫只运行一次：

```sh
docs/v0.1/claude_to_codex/tools/heavy.sh env GOFLAGS=-p=1 go -C server test -p=1 -count=1 -json -timeout 10m -run '^TestTheSearchReadsInTimeAtAnyStatistics$' ./internal/modules/page/adapter/postgres
```

| 外部运行目录（`calibration/` 下） | 实际结果 | test.log SHA-256 |
| --- | --- | --- |
| `meta-baseline` | 目标与包 PASS，exit 0 | `4c3248c3687dc20a8515cdd0eca7da16aacf47f88d670bcff1a962163190c488` |
| `meta-mutant` | 原 trap assertion FAIL，exit 1 | `d27a967cad9146453537d8daded3a91ecdebe0195f9e9b9015c66f0c16cb2165` |
| `meta-restored` | 目标与包 PASS，exit 0，精确恢复 | `a9e2df08c6a23829988667531f77945ca31bd54a7a93db3d71abc4fed487207c` |
| `full-search-read` | 全60格 PASS；测试49.58 s，包49.611 s，墙钟51.119710 s；exit 0 | `ca77bc2824c7b0e2a28f0bb9bcc2a6c96ef97c7952686375dfea103e0acdffe5` |

每轮保存完整argv、UTC时间、输入/恢复后清单、patch、Go版本、exit和日志哈希。候选输入3,202项清单 SHA-256 `46f8574f26f046696d1ac2453164d85911027cacf2b1569b1efdc0463c1e313c`，baseline/restored/full相同；mutant独立留档。完整读仍为原5,000已建页、500准确入队页和另5,000批量排队节点，每格八次。原build最小 **12.484 s < 26 s**、QueueSearch最小 **23.945 ms < 57 ms**，两者预算/取最小值语义不变。完整60格打印结果及精确限值保存在 `calibration/full-search-observations.json`，不在本记录重复整张表。

主持者另执行七项逐提交 Go 门禁。以下各命令实际经 `docs/v0.1/claude_to_codex/tools/heavy.sh` 串行运行，runner环境为 `GOFLAGS=-p=1`，工作目录是仓库根：

| 门禁 | 传给 heavy.sh 的实际命令 | 已验结果 |
| --- | --- | --- |
| server build | `go -C server build -p=1 ./...` | exit 0，11.731 s |
| tools build | `go -C server/tools build -p=1 -o /dev/null ./...` | 修正输出路径后 exit 0，4.760 s |
| server vet | `go -C server vet -p=1 ./...` | exit 0，17.894 s |
| tools vet | `go -C server/tools vet -p=1 ./...` | exit 0，0.898 s |
| 全模块 lint | `make -j1 lint-go` | exit 0，16.470 s；含两模块lint和tidy-diff |
| server short | `go -C server test -p=1 -short -count=1 -json ./...` | exit 0，160.584 s；8,208 run均有终结（6,915 pass、1,293 skip） |
| tools short | `go -C server/tools test -p=1 -short -count=1 -json ./...` | exit 0，1.142 s；27 run/27 pass |

首轮 tools build 的实际命令是 `go -C server/tools build -p=1 ./...`，exit 1，日志为 `go: build output "bodyshapegen" already exists and is a directory`；默认输出名撞上源码目录。原 `per-commit-gates/run.json` 保持 FAILED（SHA-256 `eea7173960dd14dc95ed2620cf97e0a1b6f050868e14f0d7e1a0c9787e71cbab`），原错误日志 SHA-256 `70b5fbd6eb147638246adc6b9200b2115416b9df095b43bea7f911d90c22f7af`。主持者只修正 runner 输出为 `-o /dev/null`，保留已成功的 server build并运行其余六项；`per-commit-gates-resume/run.json` SHA-256 `b9f1f120869141f8fb89190d9122964cd6f6b461dfa0e94e47cc5cda06b80609`。两轮输入清单与候选一致，没有仓库输入变化；这是调用错误及其修正，不是产品编译/测试缺陷。

主持者本机接受原件 `calibration/root-local-gates-acceptance.json` SHA-256 `499b711f2bd4a69a66f526fad6241b2fde5f2da5298dfcb74e2d9987337ca1fe` 固定各原件与提交前核对回执。七项门禁和作者四次范围内运行分别记账；不声称当前重新跑过全部本机 Web/E2E 门禁。

## 5. 本轮独立代码审查

各席在自己的隔离工作树读精确 base/code 与分工原件，不读同行本轮报告或作者私有推理。以下结论只授予该席已经核对的范围，不把 PENDING 算成通过。

| 席位及原件（`calibration/reviews/` 下） | 已核范围和结论 | I / M / Nit / Q | SHA-256 |
| --- | --- | --- | --- |
| A，`a-round1.md` | 规范与质量/证据在参考/算术范围 PASS；180/180原值、60/60新参考、120/120新旧限值、3,202/3,202 base输入哈希；无数据库重跑 | 0 / 0 / 0 / 0 | `d8d3e447f19dbd910c6eb4b085848b02d46532a5ad6b72372cea54c6c4c6440b` |
| B，`b-round1.md` | 规范与质量/证据在守卫语义/负控范围 PASS；原480次读路径、作者trap负控与恢复已核；独立去倍率负控实际PASS→FAIL→PASS | 0 / 0 / 0 / 0 | `a507953c83d339e3eb200dba39ce211cddfff292f25291202b890aba41ac9650` |
| C，`c-round1.md` | 已核本机四次运行/恢复、七项Go门禁、输入等价及CI助手严格逻辑；规范 PASS；实际code CI质量/证据 PENDING_ACTUAL_CODE_CI | 0 / 0 / 0 / 0 | `bbade7a30f7d14e8a8d7308c847f9f4f59d4249ca9aeb17ea549f60a981f8071` |
| C，`c-round2.md`（实际code CI复核） | 规范与质量/证据 PASS；关闭第1轮实际code CI待验项；后续文档分支/main与P3放行仍不在本轮接受范围 | 0 / 0 / 0 / 0 | `92a06229bdd57a91c356aab59f581df8b3880fd50a8c5b40f0f880f7eab45500` |

A 独立重建的值/限值映射在 `a-independent-audit.json`（SHA-256 `3d52fe14fb35b424308304e7c31a27647bb9bce18c2f300167fb97e20de9aa50`）；其首次审计脚本只因提案代码块数的解析假定失败，修正脚本后通过，不是仓库测试失败。作者审计最初假定所有float转换均精确，因16.4 ms一格断言失败，修正外部审计的精度假定后通过；同样不计作产品负控。

B 在独立工作树另执行一次公式负控，baseline/mutant/restored都使用第4节完全相同的race元测试命令。唯一临时修改为 `max(3*median, searchFloor)` → `max(median, searchFloor)`，表、floor和元测试均不动；2026-10-29 16:44:01–16:44:12 UTC实际完成：

| B独立运行（`calibration/reviews/b-evidence/` 下） | 实际结果 | test.log SHA-256 |
| --- | --- | --- |
| `formula-baseline` | PASS / exit 0；墙钟6.728 s | `d3d370c5d270b279e63043be577d96cc40af5fa7057f5dcf4b1653e927c2949b` |
| `formula-mutant` | FAIL / exit 1；墙钟2.578 s；恰39条原line88三倍下界断言失败 | `4ad27bd3c7708a68f81b42442c68cabc3d60f0eb61503ec8d2ddc4c932f93d8e` |
| `formula-restored` | PASS / exit 0；墙钟1.684 s；3,202项恢复清单与baseline一致 | `ab8f3626498845860603e1423656386d3c48daacd16910684a5f1f2fce4e93c5` |

39条失败逐项对应全部非floor格，其余21格仍取40 ms；没有trap断言失败、编译失败或异常终止。恢复的文件与备份SHA均为候选的 `3b5a87a6...`，审查工作树最终干净。主持者独立读取三份原日志核对以上事实。这个控制验证三倍下界，作者的99 ms控制验证trap上界，两者分开记账；都不冒称历史坏SQL复现。

C 独立重算20项门禁原件/回执哈希，逐项核对3,202个code输入并重解析short事件与完整读日志；CI助手保全失败前驱，另用先前完整20项模板，要求新run精确head/branch/workflow/attempt、11个作业及全部必需步骤、20个成本指标、45个来源HTTP样本、60个搜索读和真实PASS。这是第1轮当时的助手静态核对；该轮实际CI待验状态保留为历史，由第2轮按下列真实完成证据关闭。已读报告未提出扩大SQL/性能调查或新增镜像测试的要求。

C第2轮只读完成的run/jobs/timings、完整costs/guards原日志、202项保护清单及实际collector/verifier产物，重新核对身份、样本与算术后给出规范/质量均PASS，没有新增发现。原C1文件及其哈希不变。C2审计最初将整数文本与带小数尾零的文本直接比较，随后以Decimal数值比较确认相等；这是审查脚本表示差异，不是产品测试或CI失败。主持者全文读完C2并接受代码审查，`calibration/root-review-acceptance.json` SHA-256 `85f458127ea5adf6ceab0b802aba6ae1147e71dc8c7beaf135050768165d1bcb`绑定A/B/C1/C2、本机门禁及精确代码CI。

## 6. CI、集成与尚待验收

代码分支的实际接受原件 `calibration/ci-code-ccac8c5d/root-ci-acceptance.json` SHA-256 `394260e989b5e8251d63c9052705571255b2ffd3a67e8758cdf15068555316eb`；本次成本清单位于上级M10树外证据根，`search-calibration-code-ccac8c5d-ci-cost-supplement.json` SHA-256 `184ebaaebd783bee7e70d691126461aa05ecb07bd021f8c3adb409718a6db6be`。实际收集/验收所用助手与C第1轮审过的字节相同，完成证据由C第2轮独立复核。

本轮取得完整20个成本指标、45个来源HTTP样本、60个搜索读及真实PASS；202个受保护旧文件哈希不变，原失败报告仍为时间线前驱、原19项与后八命令NOT_RUN未回填。有界all-branches push清单无中间缺口，累计47→48个observed logical runs，包含失败，不是成功数或稳定性统计。

搜索测试实际60格PASS，用时60.13 s；build最小14.713 s、QueueSearch最小31.748 ms，仍分别守26 s/57 ms。`go`首页无统计/ANALYZE/generic为 **21.296 / 20.982 / 23.034 ms**，相对新界60.3/60.6/62.7 ms通过；**这三格也都低于原40 ms界，因此这次绿不能证明校准修复了原失败的成因，或原失败只是噪声。** 原40897835721 attempt1仍按第2节保留FAILED。

本代码分支单次CI用时如下；步骤和作业时长分别记录，不混作单次测试样本：

| 作业 | Test / E2E / Image smoke 步骤秒 | 作业秒 |
| --- | ---: | ---: |
| web | 262 | 297 |
| server (short) | 183 | 203 |
| server (race) | 690 | 712 |
| server (bootstrap-1) | 440 | 458 |
| server (bootstrap-2) | 358 | 377 |
| server (bootstrap-3) | 356 | 374 |
| server (bootstrap-4) | 370 | 394 |
| server (costs) | 550 | 697 |
| server (guards) | 1159 | 1178 |
| e2e | 265 | 315 |
| image | 74 | 79 |

bootstrap四片Test的440/358/356/370 s只是这一次代码分支CI，**不是两次main样本、均值或P3容量准入验收**。e2e仅按必需步骤成功验收，没有下载或声称逐leaf报告；后续分支/main配对预算另验。

| 阶段 | 精确对象与运行 | 事实与接受状态 |
| --- | --- | --- |
| 原设计 main | `9e38a38f648ffa19130afb8a8b468b792dad1381`；[40897835721](https://github.com/open-nerve/NerveWiki/actions/runs/40897835721) attempt 1 | FAILED，8 success / costs failure / e2e+image skipped；后八条costs NOT_RUN，永久保留 |
| 本维护代码分支 | `ccac8c5d9e741721965e07c2b053f04dbe446e61`；[40913568907](https://github.com/open-nerve/NerveWiki/actions/runs/40913568907) attempt 1 | completed/success；11个作业、32个必需步骤均成功，恰23个既定条件skip；实际collector/verifier与C2复核完成，主持者已接受 |
| 本维护文档分支 | `aa860be1b32bb98410e6a6811d2d87569de78b25`；[40921122246](https://github.com/open-nerve/NerveWiki/actions/runs/40921122246) attempt 1 | 文档应用/链接检查及完整CI已由主持者接受；11个作业/32个必需步骤成功，恰23个既定条件skip，回执见第7节 |
| no-ff 合并与main | `b957fdbe0367a9a6ff0f65d2afc303db49a755c2`；[40925905708](https://github.com/open-nerve/NerveWiki/actions/runs/40925905708) attempt 1；合并另列00号文档第12节 | 两父及树已核，main自己的完整CI已由主持者接受；11个作业/32个必需步骤成功，恰23个既定条件skip；本记录自身提交/CI仍待产生，回执见第7节 |
| P3/S1入口 | 后续集成后的精确main与S1-0候选 | 主持者重新绑定放行回执前仍未释放；S1/S2结果与实测不得预填 |

当前已接受的是决定61的处置、对应候选的局部验证、七项Go门禁、A/B/C各自审查与C2实际代码CI复核、精确代码分支CI，以及文档分支CI、实际no-ff合并与首次main自身CI。本记录自身提交/CI、新增来源设计修订及其独立审查/实际设计提交CI、P3重新绑定放行仍待完成。未来CI、审查复核或修复另按真实对象追加，不覆盖本记录保留的失败。M12开放义务见第3节；P3原设计49–60及原容量准入仍由[P3设计](../03-P3-lint-health.md)和[设计审查](P3-design-review.md)约束。

## 7. 维护文档、合并与main CI

2026-10-30（UTC+8），文档提交 `aa860be1b32bb98410e6a6811d2d87569de78b25`（父为代码 `ccac8c5d9e741721965e07c2b053f04dbe446e61`）的 [CI 40921122246](https://github.com/open-nerve/NerveWiki/actions/runs/40921122246) attempt 1，精确分支 `m10-search-guard-calibration`、push事件、`.github/workflows/ci.yml`，completed/success。文档投影、链接及空白检查的接受原件为 `calibration/doc-package/root-live-acceptance.json`，SHA-256 `9860b458442a46708ea9f11bc85a50f5ec979a3b99fada6e46b4a133c0352d19`。该文档CI全11个作业、32个必需步骤均成功，恰23个既定条件skip；主持者接受原件 `calibration/ci-docs-aa860be1/root-ci-acceptance.json`，SHA-256 `0ab6de2dca111e0f0854c9655de1bb582f4836173eddcfedcde1143fc0654ccd`。

实际no-ff合并 `b957fdbe0367a9a6ff0f65d2afc303db49a755c2` 的第一父为 `9e38a38f648ffa19130afb8a8b468b792dad1381`，第二父为已接受文档头 `aa860be1b32bb98410e6a6811d2d87569de78b25`；树 `8ded1428ce9cfe42a24ab80dd374c32c10b4dfd3` 与文档分支树相同。合并对象核对原件 `calibration/root-merge-object.json`，SHA-256 `c50499adae7985fe782cd0fa05d54c677c3df40dbdb351059bcc4a2a27135425`，保留其中当时main CI尚待的历史字段；随后完成的main运行由下一段独立回执接受。该维护合并在[00号文档第12节](../00-M10-design.md#12-phase-进度表)单列，P3/S1合并格仍为“—”。

首次main的 [CI 40925905708](https://github.com/open-nerve/NerveWiki/actions/runs/40925905708) attempt 1绑定上述完整合并SHA、`main`、push事件及同一workflow，completed/success；全11个作业、32个必需步骤均成功，恰23个既定条件skip。主持者接受原件 `calibration/ci-merge-b957fdbe/root-ci-acceptance.json`，SHA-256 `0010ddfa01a9ac82e19f5ef92c394036073f3f0bfc363c410c487e1b5870ecef`。这是该精确main运行的工具核验与主持者接受，不扩展第5节A/B/C原代码审查的受测对象。

两份成本清单均在上级M10树外证据根：文档CI为 `search-calibration-docs-aa860be1-ci-cost-supplement.json`，SHA-256 `1297d77a46aefd959165e8c974114c0a64f7eea9672fde0494744ae25bff8ca0`；首次main为 `search-calibration-main-b957fdbe-ci-cost-supplement.json`，SHA-256 `ad8e07bf8f87cf47f2cb2b0bade6d90fdfd157b7bc015f9894405825c75475f4`。两次各取得完整20项成本指标（14 costs / 6 guards）、45个来源HTTP样本（9个201、36个200）、60个搜索读及真实PASS；两次收集各保全210个受保护旧文件。按各自有界all-branches push清单，无中间缺口，observed logical runs由代码CI时的48增至文档CI时49、首次main时50，包含原失败；原第2节47、第6节48及38+1搜索日志均保留各自历史口径，不将累计数写成成功数或稳定性统计。

文档CI搜索测试PASS用时78.35 s，首次main为70.89 s。`go`首页无统计/ANALYZE/generic分别为文档CI **31.393 / 29.733 / 33.855 ms**、首次main **25.356 / 24.601 / 25.541 ms**；这些样本也都低于原40 ms界，仍不能证明校准修复了原失败成因或原失败只是噪声。原40897835721 attempt1的FAILED、后八条costs NOT_RUN、e2e/image skipped及缺失null不变；39/21检测取舍与M12真实语料/目标硬件等开放义务仍按第2–3节保留。

以下均为各自一次运行的实际秒数；“步骤”指Test、E2E或Image smoke，与整个作业时长分列：

| 作业 | 文档CI步骤秒 | 文档CI作业秒 | 首次main步骤秒 | 首次main作业秒 |
| --- | ---: | ---: | ---: | ---: |
| web | 252 | 288 | 253 | 289 |
| server (short) | 192 | 211 | 138 | 157 |
| server (race) | 682 | 701 | 441 | 460 |
| server (bootstrap-1) | 432 | 456 | 282 | 301 |
| server (bootstrap-2) | 316 | 334 | 447 | 471 |
| server (bootstrap-3) | 298 | 322 | 244 | 263 |
| server (bootstrap-4) | 358 | 378 | 368 | 389 |
| server (costs) | 775 | 963 | 665 | 825 |
| server (guards) | 1234 | 1258 | 1507 | 1529 |
| e2e | 252 | 320 | 245 | 298 |
| image | 79 | 85 | 86 | 93 |

首次main第二片的Test为447 s、作业为471 s，二者口径分开。文档分支不计为main样本；本记录自身CI尚未发生，因此这里不计算或宣称指定两main均值，也不构成S1-0候选或P3容量验收。e2e仍仅按必需步骤成功验收，没有新增逐leaf报告；没有新本机产品运行、300 ms/30万页验收或失败成因证明。

**P3入口仍未释放。** 本记录自己的提交号与CI待实际产生，后续另以精确对象补回执。新增合法空名称frontmatter链接来源丢失问题仍OPEN，影响拟定D/F/P4的正文边判定；决定62已核定处置方向，具体设计修订、三位独立审查及后续复核/实际设计提交CI尚待，维护CI成功不能解除该缺口。方向原件为上级M10树外证据根 `p3-preparation/origin-amendment/root-decision-62.md`，SHA-256 `245f24b9052d5f890b2bbcc326fd0e867c6dd5e8cb88424af16462618e172829`；此处仅记当前待办，不展开未审设计语义。主持者须在这些前置接受后重新绑定P3放行回执与S1-0候选；S1-0结果空缺，S1/S2未实施，M11未开始。
