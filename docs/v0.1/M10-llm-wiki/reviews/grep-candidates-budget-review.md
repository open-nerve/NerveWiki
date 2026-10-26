# M9 候选成本预算维护（M10 期间发现）：审查记录

代码及三份独立审查、本机门禁和代码分支 CI 已核对；本次文档的 CI、合并及 main CI 尚待进行，P2 入口保持关闭。

| 项 | 值 |
|---|---|
| 起点 | `b112450f72e863bf35a1cc4288c431f444bc723e`，P1 完成文档 |
| 分支 | `m10-grep-budget`，P1/P2 之间的 M9 维护，M10 Step 之外 |
| 当前代码 | `789b407147a78e49c645f74d79b379a13a4c19ec`，两文件提交；输入哈希与实际门禁逐字相同 |
| 审查头 / 最终代码头 | `789b407147a78e49c645f74d79b379a13a4c19ec` / 同一提交，无需修复 |
| 日期 | 2026-10-27 |
| 授权入口 | [M10 总设计](../00-M10-design.md)第 7 节的 grep 复发义务、[工作流](../../claude_to_codex/02-workflow.md)第 5 节“旁支” |

## 复发、库存与计时口径

P1-done main [run 40378004872](https://github.com/open-nerve/NerveWiki/actions/runs/40378004872) 的 generic 候选查询三次为 195.1798、389.483274、415.468253 ms，中位数 389.483274 ms 超过 270 ms；无统计与 ANALYZE 后的中位数分别为 194.228692、185.684096 ms，完整调用为 420.822970 ms，仍在 1 s 内。7 个作业成功、guards 失败、e2e/image 跳过。这次不是绿色 main 门禁；没有重跑 CI 或用文档推送替代维护。

此前 [run 40123373818](https://github.com/open-nerve/NerveWiki/actions/runs/40123373818) 的无统计候选三次为 195.789808、388.272425、393.072655 ms，中位数也超过旧界。完整 14-run 库存、后续 3-run 补充及这次 P1-done 补充共覆盖 18 次 workflow，其中 17 次有此守卫；旧 m6-p3a 的一次没有该测试，不能记通过或 0 ms。17 次共 51 个三次组、153 次调用，10 个单次超过旧界、分布在 8 组，只有 2 个阶段中位数超过旧界。最大阶段中位数为 389.483274 ms，其余通过的中位数最大 201.353936 ms；最大单次为 420.250536 ms。慢单次、失败及 annotation 的重复打印都区分保存，不把后者再算一个样本。

候选计时从 `GrepCandidates` 调用到读完全部 ID 返回，含连接池获取、SQL、传输、客户端解码/分配及调度等待。SQL fixture、模式准备和 ANALYZE 不在其中，树读取、glob 取交和正文走查属于完整调用。不能用完整调用通过代替候选界，也不能从同一源码树推断 CI 的二进制、运行机或运行状态相同。

原始 guards 失败日志 SHA-256 为 `ad62ec1ed361973bcf69edd297b40f5ede3bc98fffcf9b4e313e7d163a4a1493`。外部证据清单保存库存 JSON、原日志出处/哈希、诊断、变异、门禁及本文输入：`evidence-snapshot.json`；固定证据清单为树外 `grep-budget-doc-implementation/bound-789b4071/evidence.json`，SHA-256 `3dc2e8619bff769f4483f2da759be37b5d3404d534e6fd87a2f4edf5102a4211`；它保留原证据路径和逐文件哈希。

## 实际修改

`server/internal/bootstrap/grep_cost_test.go` 只把 `grepCandidatesBound` 从 270 ms 改为 600 ms，并改注释说明 CI 阶段中位数、余量和未归因的限制。计时循环、三次中位数、每次 100000 IDs、完整调用的走查断言都没有改。Makefile 只改预算注释：工作界仍是 S1 本机测量的三倍，候选为 600 ms，完整调用为 1 s；原来约 10.5 秒乘 2.4 的估算标为历史值。

提交相对 b112 的差异恰好两文件、20 行新增和 8 行删除；提交树为 `e4d87fec6db899c6601f18ed84495b66ef4b20a5`，父提交为 b112。生产 SQL、Store、用例、计划/范围测试和工作、glob、IndexItems 的界都没有改；反向对照结束后的生产 `grep.go` 与起点逐字相同。完整调用仍三次中位数、1 s。本次没有部署、配置或客户端行为变化。

## 本机基线与一次有界诊断

主持者在未修改的 b112 上经 `heavy.sh` 运行一次 `go -C server test -count=2 -run '^TestGrepOfAHundredThousandCandidatesTakesItsTime$' -v ./internal/bootstrap`，退出 0，含构建墙钟 21.020 秒。两轮阶段中位数如下；本机通过不解释 CI 失败原因。

| 轮次 | 无统计 ms | ANALYZE 后 ms | generic ms | 完整调用 ms |
|---|---:|---:|---:|---:|
| 1 | 96.811916 | 93.146625 | 97.985000 | 213.830208 |
| 2 | 98.853375 | 94.048917 | 99.702292 | 217.001459 |

之后只做一次临时探针，使用原 10 万页/32 键 fixture、阶段次序与 generic pool 构造。临时 helper 从生成代码逐字取 SQL，每阶段连续五次，记录 Acquire、Query 返回、读完/释放与进程级 GC/分配；每阶段之后在同一 backend 上以新 PREPARE 名称读 EXPLAIN。经 `heavy.sh`，进程组上限 180 秒、`go test -timeout 3m`，实际墙钟 16.258 秒、测试 9.54 秒、退出 0。环境为 Go 1.27.1、linux arm64、4 logical CPUs/GOMAXPROCS=4、PostgreSQL 18.6，当时 loadavg 约 0.29/0.22/0.14。

| 阶段 | 五次 wall，按执行顺序，ms | 中位数 ms | prepared EXPLAIN 执行 ms |
|---|---|---:|---:|
| 无统计 | 105.913250 / 95.555666 / 98.773250 / 94.988333 / 96.392292 | 96.392292 | 97.572 |
| ANALYZE 后 | 92.649208 / 93.295959 / 96.521125 / 93.380084 / 92.913959 | 93.295959 | 97.776 |
| generic | 117.852167 / 93.833875 / 97.855917 / 95.618708 / 98.041084 | 97.855917 | 101.148 |

十五次都返回 100000 IDs。首次 generic Acquire 为 3.121 ms、创建一个连接，其余 0.001–0.011 ms、没有新连接。Query-return 阶段 76.868–97.700 ms，之后 drain/close/release 13.615–18.603 ms。每次进程级分配约 12.14–12.24 MB、约 20 万次 malloc，1–2 次 GC、STW pause 合计 0.060–0.211 ms；没有测 GC 的并行 CPU 成本，低 pause 不证明 GC 没有成本。

三份外层 hash aggregate 都返回 100000 行、单 batch、峰值 7449 kB、Disk Usage 0，根节点没有 shared read 或 temp read/write，未见 spill。队列在后两阶段是空表 Seq Scan（0 行、0 块）；不能据此说发生了大队列范围退化，大队列的保护另由已有 `TestGrepPlans` 承担。generic 外层 EXPLAIN 有 17 个 JIT functions、总 3.451 ms；这是原 generic 测试 pool 只设 plan_cache_mode、没有常规连接的禁 JIT 设置所产生的既有差异，hits 函数内部仍禁 JIT。本次没有改设置，也没有把它认作 CI 原因；它尤其解释不了 S2 无统计阶段的失败。

本机没有复现 CI 的双慢样本。探针绕过 Store/gen 薄包装，增加到五次并在阶段之间插入 EXPLAIN/ReadMemStats，会影响缓存和调度。Query-return 与 drain 都混合服务器、传输和客户端工作，EXPLAIN 不返回原结果，不能相减归因；外层计划也没有展开函数内部 GIN。五次都快不证明 CI 五次中位数会通过，不能估尾部概率。因此保留正式守卫的三次中位数，预算依据是已测 CI 阶段中位数。

临时 helper 已删除，探针恢复时测试文件前后 SHA-256 均为 `a9996cf5b05c3ffff35f852daad871cfb485698f08e103d76c02e8b4321fd13b`；原日志 SHA-256 为 `05695957912c1f9d3835c8864faf061b8770ac7a122b31c218614ce28e8dc6b8`。之后的 600 ms 修改是主持者另行实施的维护差异，不与探针混记。

## 真实退化的反向对照

先保存未改 baseline，再临时在真实 `Store.GrepCandidates` 得到 IDs 后给每个 ID 做一次点查，仍返回原 IDs；小 fixture 的 `TestGrepCandidates` 比较成员，大成本测试继续检查每次 100000 IDs。这里没有 sleep、硬编码失败或改大候选数。

第一种点查调用已有 `FindNode` 读完整节点。baseline 的成员/成本测试和 mutant 的成员测试均通过；候选三阶段中位数 11.792810255、13.437359756、11.719332172 秒，确实到达并超过 600 ms 断言。但整个测试在后续完整 MCP 调用期间触发这次实验指定的 `-timeout=120s`，墙钟 123.573 秒；它只保留为部分证据，**不记作完整的预算反向对照通过**。`partial-proof.json`、原日志和 patch 留存，生产文件已精确恢复。

为让同一类真实 N+1 查询能完整结束，主持者另行授权一次较轻的 `SELECT EXISTS (SELECT 1 FROM nodes WHERE id = $1::uuid)`：每个 ID 仍做真实主键查询并 Scan，若不存在或查询出错就失败，返回原 IDs。实验命令仍是上述两个测试，`-count=1 -v`，只把实验的 Go 时限设为 180 秒、外层上限 210 秒；生产和门禁的时限没有改。结果如下：

| 阶段 | 成员测试 | 无统计中位数 | ANALYZE 后中位数 | generic 中位数 | 完整调用中位数 | 成本测试 |
|---|---|---:|---:|---:|---:|---|
| baseline | 通过 | 95.790583 ms | 91.840000 ms | 97.600084 ms | 213.531708 ms | 通过，墙钟 9.802 s |
| EXISTS 变异 | 通过 | 11.354374797 s | 11.452923088 s | 11.310458089 s | 11.519901755 s | 三个候选成本断言及完整调用断言失败；测试 145.60 s、墙钟 149.247 s，无超时 |
| 恢复 | 不声称另跑成员测试 | 94.121583 ms | 93.234792 ms | 97.725125 ms | 206.076626 ms | 通过，墙钟 9.317 s |

变异成本原输出只有四个时间断言失败，没有成员、候选数或走查错误；三个候选断言分别超过 600 ms，所以这里的候选预算证明不依靠完整调用同时失败。此反例证明该真实逐候选查询退化仍被抓到，不能推广成所有复杂度退化必被此绝对界捕获。

EXISTS 变异日志 SHA-256 为 `d6d44f6014e880f0e6a54056b2b74b6728f4fd38f0902fa2721fa9c5561c8ab0`。两次实验恢复后的生产 `grep.go` SHA-256 都为 `4031a47c2d42baa0301c1110b25ed7f3f25bf86438e47f4cd6f4530df15e92aa`，与 b112 原件相同。独立审查者另在精确 789b 上完成下述三组真库反向对照，均从备份精确恢复并以同命令通过；见“独立审查与修复核对”。旧 M9 完整调用的 10 µs sleep 和 IndexItems 的整值读取不算本次候选预算证明。

## 主持者的决定（负责人可以改判）

编号沿用 P1 实施记录之后的实施与维护序列：S3 到 34，S4 无新决定并为下一条保留 35；与 [00-M10](../00-M10-design.md)第 11 节设计审查 1–40 独立。

35. **候选查询三次中位数的界改为 600 ms**。主持者按原始 CI 样本、完整 EXISTS 反向对照及三个独立审查接受此方案；初始接受时的 `review_pending: true` 保存在外部草稿的证据快照中；当前 `root-budget-acceptance.json` 已为 false，`root-final-review-acceptance.json` 明确接受三份审查与本决定。最慢的已测阶段中位数为 389.483274 ms，600 ms 是它的 1.5405026 倍，余量为 **54.05%**；用的不是最大单次或跨 run 平均。保留每阶段三次中位数，因为当前没有 CI 的第四、第五次分布来证明五次更稳定，不以本机五次探针替它作证。每次候选数、成员、计划和范围保护保留，真实逐候选点查仍被三个候选成本断言抓到；完整调用 1 s、其他守卫和生产代码不改。局部诊断没有建立 CI 根因，未来仍可能复发；M10 收尾继续核对。负责人可以改判。

接受的代价是绝对时间守卫放宽：600/270 = 2.2222 倍，阶段中位数落在 `(270, 600] ms` 时从原来拒绝变为通过；独立的完整调用 1 s 守卫不能覆盖整个新增容许区间。选择是依据已观测 CI 校准、仍能抓到已演示的严重逐候选点查退化，并保留成员、候选数、计划和范围检查；不能宣称性能保护完全不变。

主持者全文读取 A/B/C 报告并复核各原始运行凭据，接受三份独立结论：没有未结 Important、Minor、Nit 或 Q，无需修复。代码 CI 的结果与后续文档/合并门禁分别记录，不由审查结论替代。下一条实施记录为 36；日常修复、门禁记录和 CI 成绩不另发新决定编号。

## 独立审查与修复核对

三位审查者在各自独立 worktree 上检查同一精确提交 `789b4071`，起止 HEAD 相同、状态干净，未读彼此推理；真库命令经共享 `heavy.sh` 串行执行。完整原报告、proof、日志/patch 哈希及主持者的接受独立保留。

| 审查者 | 范围与实际反向对照 | 结果 |
|---|---|---|
| A | 校准、计时口径、真实逐候选成本；在每个候选后执行 `SELECT id FROM nodes WHERE id = $1::uuid`、Scan UUID 并断言相等 | baseline、mutant、restored 的成员测试全部通过。成本 baseline 中位数 98.675292/93.416750/99.267875 ms；mutant 三阶段 11.355759298/11.331459589/11.369445589 s 各自超过 600 ms，完整调用另为 11.636508048 s；每次候选数与走查正确，测试完整结束、无超时。恢复成本中位数 97.270917/93.320875/98.988916 ms，通过 |
| B | 计划；编译进产品的队列谓词由 `q.notebook_id = $1::uuid` 改为 `q.notebook_id::text = $1::uuid::text`，未跑生成器 | 同一 `TestGrepPlans` 命令 baseline/恢复通过（Go test 2.640/2.554 s），mutant 2.592 s 明确在无统计、generic=false 时拒绝 `page_search_queue` 的 Seq Scan 及缺失指定索引。其余阶段未报错，不声称每阶段都抓到 |
| C | 成员与实际读范围；编译查询只删除队列的 notebook 谓词 | 同一成员+范围命令 baseline 7.151 s 通过，mutant 6.292 s 失败，恢复 5.564 s 通过（墙钟）。成员多出其他笔记本的一个准确队列 ID（6 对 5、5 对 4）；A→B→A 的外笔记本内容变化使候选查询队列块数为 19→29→19，10 个范围比较失败；无编译错误或超时 |

A 的原文件与恢复 SHA-256 为 `4031a47c2d42baa0301c1110b25ed7f3f25bf86438e47f4cd6f4530df15e92aa`；B/C 的生成查询文件均恢复为 `498c02500d943966db9a05b981570937fb9d68e3853b470a13f2432127a0b2fa`。A 的六份、B/C 各三份日志哈希均由主持者逐份核对。三个报告都没有核实发现、未采纳建议或需要修复的条目；无需修复轮次。B 曾静态怀疑 residual Filter 检查较宽，实际 cast 变异被抓到，没有升级为缺陷；CI 慢样本原因仍未知。C 另核对过七文件草稿的十五锚点、历史保留与门禁证据，其报告未冒称验收当时尚未生成的最终 updater/CI。

## 门禁、环境失败与 CI

第一轮 `make -j1 lint-go gen-check-go` 在安装工具时因 `curl: (35) TLS connect error: ... unexpected eof while reading` 退出 2，墙钟 5.014 秒；尚未运行实际 lint 或测试。这是工具下载的环境失败，保存在 `grep-budget-gates`，不算通过也不记作产品失败。网络恢复后，主持者的进展记录说明复用了原仓库中按哈希核对的 golangci-lint 2.14.0；恢复过程另存 `grep-budget-gates-resume`，不覆盖原失败。

恢复后的六项本机门禁全部通过，原始命令、退出码、输入文件和日志哈希已核对：

| 命令 | 退出码 | 墙钟秒 |
|---|---:|---:|
| `make -j1 lint-go gen-check-go` | 0 | 12.111 |
| `go -C server build ./...` | 0 | 0.822 |
| `go -C server vet ./...` | 0 | 2.239 |
| `make -j1 test-go-short` | 0 | 57.184 |
| `make -j1 test-go-guards` | 0 | 457.815 |
| `go -C server test -count=1 -v -run '^(TestGrepCandidates\|TestGrepPlans\|TestHiddenNotebooksLeaveTheSearchsReadsAlone\|TestTheSearchReadsUseTheirIndexes)$' ./internal/modules/page/adapter/postgres` | 0 | 10.852 |

这些命令先作用于 b112 加两文件差异，提交后逐字核对输入与 `789b4071` 相同（`grep-budget-code-commit-proof.json`）；不声称提交后重新跑了一遍。全部重活经 `heavy.sh` 串行。Makefile 可执行行未变，Go 改动仅在 `!race` 守卫文件；生产、网页、API、workflow 和依赖与先前绿色 main `767b69a6` 的相关输入相同。未改范围只沿用原受测头的证据，不冒称本次重跑 race/三份 bootstrap/web/E2E；完整分支与 main CI 仍是各自实际门禁。`grep-budget-input-scope-proof.json` 的“当时正在运行”由上述六项实际结果补充，原记录不改写。P1-done `40378004872` 的失败继续保留，不能被更早成功替换。

代码分支 [CI 40388475778](https://github.com/open-nerve/NerveWiki/actions/runs/40388475778) 在精确提交 `789b407147a78e49c645f74d79b379a13a4c19ec` 上正常 push 触发，attempt 1，十个作业全部成功。主持者接受原始 run/jobs/timings；本包逐项从原时间戳重算并核对下表。此处没有声称后续文档 CI、合并或 main 已完成。

| 作业 | 实际步骤 | 步骤秒 | 作业秒 |
|---|---|---:|---:|
| web | Test | 137 | 164 |
| server (short) | Test | 180 | 199 |
| server (race) | Test | 623 | 648 |
| server (costs) | Test | 525 | 696 |
| server (guards) | Test | 477 | 494 |
| server (bootstrap-1) | Test | 443 | 466 |
| server (bootstrap-2) | Test | 396 | 426 |
| server (bootstrap-3) | Test | 381 | 401 |
| e2e | E2E | 233 | 302 |
| image | Image smoke | 85 | 92 |

| 后续阶段 | 当前状态 |
|---|---|
| 本次文档提交及其分支 CI | 待进行；不预写本提交自己的 SHA 或 CI 成功 |
| no-ff 合并 | 待分支文档 CI 全绿并由主持者验收后执行 |
| main CI | 合并后再读真实 run/jobs/timings，当前没有样本 |

父提交或 S4 先前的成功不能替代维护头的结果；触发某 CI 的提交不能预先写它成功。发生后的合并与 CI 记录另补，当前 P2 入口保持关闭。

## 文档与仍开放的义务

预算说明同步到 [M9 总设计](../../M9-mcp/00-M9-design.md)4.13、M12 移交与历史，[M9/P4](../../M9-mcp/04-P4-grep-search.md)6.1，[总体设计](../../v0.1-design.md)13.4/历史及 [M12 性能移交](../../M12-release/handoffs/M4-performance.md)第 18 项。[M10 总设计](../00-M10-design.md)记录复发、维护及实际阶段；[S4 审查](P1-S4-web-review.md)只澄清编号的后续使用。

原 M9 测量、决定 102 的完整调用历史及原 270 ms 的由来保留；旧完整调用的 191 MB/GC 探针不移作本次候选归因。P1/S2 和本次失败不改写成通过。IndexItems 的 30 ms 守卫及独立稳定性义务不变。P1 已完成、M5 已关闭，M8/M9/M11 的相应移交仍开放；P2–P5 和 M10 收尾没有因此开始或完成。真正 M10 收尾继续审计维护后的运行和余量。
