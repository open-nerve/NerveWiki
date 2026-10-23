# M9 MCP：收尾审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | `main` 的 `1f7fa54c`（M9 的五个 Phase 全部合并、P5 的文档写好之后），M9 的改动是 `fff7c560..1f7fa54c`（333 个提交，317 个不是合并；530 个文件，+87,520/−1,642）。对照[文档约定](../../../README.md)的"M 完成"、[M9 总设计](../00-M9-design.md)（第 3 节的十三条、4.1–4.16、第 5、7、8、11 节）、五个 Phase 文档与 17 份审查记录（M9 的设计审查，P1–P5 的设计审查 5 份，Step 的 11 份）、收到的 9 份移交、[真实客户端的人工清单](../manual/P5-mcp-clients-checklist.md)、[输入法清单](../../M4-pages/manual/P6-ime-checklist.md)与[总体设计](../../v0.1-design.md)第 12、13 节。收尾的代码、测试与 README 的修复在分支 `m9-close`（10 个提交 `6b174115`…`741519d0`），合并在 `f214cb77` |
| 审查方式 | 三位独立审查者（Opus）并行（A 服务端、B 网页与端到端与 README、C 完成标准、移交与文档），各在自己的分离的快照上做探针与变异，结束时删掉快照，仓库没动过；Docker、PostgreSQL 与 e2e 都经 `heavy.sh`，不运行任何 MCP 客户端，不读负责人的客户端配置，不下载任何东西：<br>• **A**：`server/` 与 `api/` 里 M9 的每一处改动。`make lint-go`（server 与 tools 都是 0）、两个模块的 `go mod tidy -diff`、`gen-check-go`、`gen-check-web`、架构测试、deadcode（带 `-test` 为零；不带时 M9 只多 7 个只给测试的函数，都是接缝或测试的帮手）；`make test-go` 一遍（838 秒，组合根三份的 `go test` 313.1、182.9、218.4 秒）；M9 改过的 38 个包 `-short -race -count=3`；4.15 的 14 个交错 `-race -count=10`（140 次）；性质测试另跑别的种子（路径 12 个、batch 与索引的重建 6 个、带 batch 的撤销 6 个，domain 的取键、`schedule()` 与任务的重放 8 + 40,000 + 6,000 次）；6 个跨 Phase 的探针（`AGENTS` 经 batch 与撤销、轮次中的撤销期间的读、每个写的工具写下的截掉的文字与 `grep`、撤销了的 PAT 在两个入口、两次调用之间失去访问、batch 的版本令牌）；4.14 的上限处与过一的 34 种；组合根与跨 Phase 的接线的 48 个变异；三个 Opus 助手把 4.1–4.15 与第 5、8 节对着代码读，A 逐条复核<br>• **B**：`web/` 与 `e2e/` 里 M9 的改动（46 个文件，网页 7 个）与 README。`make lint-web knip`、`make test-web` 三遍（166 个文件、3,342 个测试）、M9 的 vitest 文件打乱次序 3 个种子、`make build-web`；`make e2e` 两遍（282 个；第二遍 S1 的一次摆动，下面"没能验证的风险"）；M9 的 33 个故事 `--repeat-each 3`（99 个），另在 CDP 降速 6 倍、`GOMAXPROCS=1` 下一遍；真实浏览器的探针（batch 的轮次撤销与撤销的撤销、途中的错误、后台撤销的状态、只用键盘、375 px、历史的 MCP 筛选与中文）；网页的 18 个变异与 e2e 的 4 个；两个 Opus 助手把 README 的 MCP 各节逐句对着 `api/mcp/tools.json`、代码与 05-P5 读，B 逐条复核<br>• **C**：九个 Opus 助手并行（反引号里的标识符与测试名、总体设计、写出的与收到的移交、流程对着 git、第 11 节对着记录、完成标准的证据、4.x 对着代码），每条发现由 C 在快照上复核。第 3 节的十三条与 12.5 逐条找证据；收到的九份移交逐项对到代码与测试，写出可以照贴的处置；写给后面的 M 的逐项对到目标的 `handoffs/`；约 500 处可以核对的说法（数、常量、码、次序、字段、注解、迁移、引文、测试名）与 30,093 个反引号的片段由脚本核对；`docs/` 的 3,190 个相对链接；15 个合并、§12、§13 与 17 份记录对着 git；第 11 节的 104 条与 13 条取舍对着记录。没跑测试与 Docker |
| 日期 | 2026-10-23 |
| 结论 | 没有行为上的 Important。A：中低 2、低 1（F1–F3；F4 同 C-I1，记为高）与设计的文字的不一致 28 条（中低与低 6、细节 22），存疑 2；B：中高 1（README 一句客户端的说法没有标出来处，B-I1 = C-M11）、中低 2、低 2、细节 12，存疑 1；C：高 1（M9 给 M10、M11 的守卫与回滚的事实不在任何移交里，C-I1 = A-F4）、中低 7、低 6（另一条同 B-I1）、细节 14，存疑 5。处理完之后 M9 是**实现完成，待人工验收**：<br>• 门禁、全部故事（两遍 282 个，一次是 M0 的冒烟故事 S1 的摆动）、M9 的故事三遍与降速的一遍、`main` 在 `1f7fa54c` 上的持续集成（run 39927680178）全绿；第 3 节的十三条满足（第 1、7、8、9、11、12 条有收尾补上的测试或改正的文字，第 13 条的人工的一半待执行）；12.5 满足，M0/P1 的移交照记下的例外留着 `open`；<br>• 代码、测试与 README 的修复在 `m9-close`（10 个提交 `6b174115`…`741519d0`，`f214cb77` 合并）：`grep` 的命中留着自己的文字的副本、不再留着整页（A-F2）；组合根的 `Lead.Ambiguous`、`grep` 的名额与解析的轮次、`batch` 的客户端名称、`AGENTS` 的交错的结尾的检查、更新的协议版本的协商、撤销的层级的等待在真的库上各有测试（A-F1、A-F3、A-D3、A-D6、C-M10、C-M9）；`startRevert` 的契约照建成的（C-M12）；网页的三处测试的缺口（B-N1）；README 的 MCP 的句子照代码改，两句新标"未核对"（B-I1 = C-M11、B-M2–B-M4、B-N2–B-N12）。A 的另一个存疑（久等解析的队列之后的第一行）以探针量过，接受（第 11 节第 106 条）。核实修复由 A、B 各在自己的快照上做，没有留下 Important 与 Minor（A 另记一个细节，第 107 条）；分支的持续集成 run 39945210705 与合并之后 `main` 的 run 39950787055 全绿；<br>• 文档（这次的提交）：八份收到的移交改为 `done`、M0/P1 写明进展而照旧 `open`；写给 M10、M11、M12 的补齐（C-I1、C-M1）；00 号文档、Phase 文档、审查记录与总体设计照代码改；主持者的 7 个决定（第 11 节第 105–111 条）；<br>• 人工验收：真实客户端的清单（50 步）由负责人在 v0.1 的最后与输入法清单一起执行，通过之后 M9 改为已完成（下面"人工验收"）；<br>• 负责人可以推翻的决定汇总在下面一节 |

## 完成标准核对

证据是 C 在 `1f7fa54c` 上读代码与测试得的（路径在 `server/internal/` 之下），测试由 A、B 与持续集成跑；收尾改过的写明提交。

| 标准 | 结论 | 证据 |
|---|---|---|
| 所有 Phase 完成，各有审查记录 | 满足 | P1–P5 的 11 个 Step 合并（`384d9f96`、`a36e92df`、`c9468e5b`、`804c3561`、`f3854b9a`、`f19d0b6f`、`43b03b36`、`56ef34c8`、`7e7e414d`、`ed9e149f`、`053d3ba1`）与 4 个只改测试的合并（`cfb48a50`、`c0636ec3`、`0c934bb9`、`9d991781`），17 份审查记录。C 以脚本对着 git：15 个合并各是它的分支的 `--no-ff`、Step 的标记与日期对，每份记录的起点、头、提交数与第二个父节点对，1,987 个反引号里的提交都在；00 号文档第 12、13 节漏写的 `c0636ec3`、`9d991781` 收尾补上（C-M5） |
| 本 M 的故事全部通过（本地与持续集成）；之前各 M 的故事仍通过 | 满足 | `make e2e` 两遍 282 个（B；第二遍 281 个与 M0 的冒烟故事 S1 的一次摆动，它单独 `--repeat-each 20` 20 个都过，见"没能验证的风险"）；M9 的 33 个故事 `--repeat-each 3` 99 个，降速的一遍 33 个；修复之后 `make e2e` 282 个（实施者与核实修复 B 各一遍）；持续集成：`053d3ba1` 的 run 39922678794、`1f7fa54c` 的 run 39927680178、分支的 run 39945210705、`f214cb77` 的 run 39950787055 全绿 |
| 从 M9 起碰页面的故事另有 MCP 版本（12.5；M9 总设计第 9 节） | 满足 | B 逐行对着第 9 节：MC1–MC8 都在，"MCP、页面"的几个有页面的部分；M4–M8 的 49 个页面的故事里 22 个有 `<故事> (MCP)`，27 个豁免的与表的 14 行相符，表外没有页面的故事；16 个工具每个至少在一个故事里调用；MC1 经脚本化的客户端与 TypeScript 官方 SDK 的客户端比较两个入口的 `tools/list` 与 `api/mcp/tools.json` |
| 第 3 节第 1 条：接入与说明 | 满足（收尾补上一处测试） | `bootstrap/mcp_test.go` 的 `TestTheMCPEdgeAnswersInOrder`（会话的令牌 403，没有、无效、撤销的 401，`notebook` 的几种 404 在 SDK 之前）、`TestTheMCPRefusesABatch`、`TestTheMCPOutlivesARestart`、`TestEveryMCPAnswerIsNotStored`、`TestTheMCPListenEndsAtOnce`、`TestTheMCPHasNoOAuthMetadata`，`mcp_instructions_test.go` 的 `TestTheInstructionsFollowTheAGENTSPage`（G16 的六种）。"更新的版本照 SDK 的协商降到它"原没有服务端一侧的测试（C-M10）：`ed7e70e4` 加 `TestTheMCPNegotiatesALaterVersionDown`（2026-07-28 与 2099-01-01 都答 2025-11-25，之后的调用照常） |
| 第 2 条：工具清单 | 满足 | `TestTheToolListsAreTheContract`（两个入口的整个工具对象）与它的反向对照，`TestThePermissionMatrixCoversEveryTool` 与 `TestMCPMatrixViolationsCatchesEachGap`，4.7 的形状 `TestTheToolErrorsAreTheTables`，panic 的 `TestTheMCPRecoversAToolsPanic` 与子进程里的 `TestAPanicDoesNotStopTheProcess` |
| 第 3 条：路径 | 满足 | `TestParsePath`（规则、四种情形、段的检查）、`TestFindByPath`、`TestParseGlob`（被拒的写法，另以正则作对照的差分）、性质测试 `TestEveryNodesPathFindsIt`（A 另跑 12 个种子） |
| 第 4 条：读 | 满足 | 每个读的工具有表格测试（`list`、`read` 的切行与 48 KiB 与附件、`versions`、`log` 的 `actor: "me"`、`changeset` 限这本笔记本、`backlinks`、`index`、`search`、`notebooks` 的"列表等于逐项判定读取"、`open_notebook`），码的次序 `TestTheMCPToolsErrorsCarryTheirCodes` |
| 第 5 条：写 | 满足 | `write` 的补建、已有的名称、令牌覆盖；`edit`（`TestReplace` 与用例的测试）；`move` 一步与 `update_links=false`（`TestAMoveWithoutItsLinksLeavesThemAndTheIndex`）；`delete` 的 `recursive`；令牌的节点与路径的节点、换了的页、别的笔记本的令牌；`mcp:<名>` 的四个写（`TestTheWritesPassTheirArguments`）与库里的变更集 |
| 第 6 条：锁 | 满足 | `TestTheEditLockReachesEveryMCPWrite`（含"你"、不结束会话），`linking.pages_locked`，batch 整个回滚带 `op`（`TestTheMCPBatchIsRefusedWhole`），C4 (MCP)。batch 与分开调用在锁上的一处不同写进 4.11（C-M13） |
| 第 7 条：batch | 满足（文字改正） | 一个单元、一个变更集与说明 `TestTheMCPBatchReachesItsUseCase`；等于分开调用 `TestTheLinksOfABatchAreThoseOfSeparateCalls`（收尾加第 25 例：歧义的链接经组合根的 `Lead.Ambiguous`，A-F1，`2017c402`）与 `TestTheWaysABatchDiffersFromSeparateCalls`；约束、规范化与界线；合并在真的库上；索引与搜索"等于重建"。这一条原写"写完立即搜得到"，第 44 条改为批量入队、答 `building`，测试断言的是后者：第 3 节改正（C-M8） |
| 第 8 条：整组撤销 | 满足（收尾补上一处测试） | `TestTheRevertOfEachBatchShape`（九种形状与上下相套的 409）、性质测试带 batch 与定向的生成器（A 另跑 6 个种子）、后台撤销的整个程序的测试、HI5。"恢复要等父节点一线先上移"原只在 domain 里测（C-M9）：`14dec63c` 在 `TestTheRevertOfEachBatchShape` 里加 `MaxDepth` 10 的 `placeLine`、`restoreLine`，另加后台每批一个节点的 `TestTheRevertsJobWaitsForAParentsLineToMoveUp`（变异 M7a 只由 domain 抓到，第 107 条） |
| 第 9 条：grep | 满足（收尾补上一处测试） | 取键与它的性质测试（A 另跑 8 个种子各 2 万次）、块数的测试、glob 在界线之前取交、行号同 `read`、不取键的走查与游标、截掉的页总被走查、程序的大小、一行与一次的工作、软期限、名额、计划测试与时间的守卫。组合根给的名额与解析的轮次原没有测试（A-F3）：`5dad1549` 加 `TestTheModulesGrepTakesTurnsByItsCores` |
| 第 10 条：上限 | 满足 | 4.14 的每一行有上限处的测试；A 另在上限处与过一跑了 34 种（`list`、glob、路径、`grep`、`index`、`log`、`versions`、`search`、`edit`、`write`、`batch`），在上限处答、过了被拒；48 KiB 的填法除了 `edit`、`move`、`delete` 的成功（4.7 说它们到不了）每个工具有测试 |
| 第 11 条：交错 | 满足（收尾补上一处检查） | 4.15 的交错都有确定性的测试、以 `checkInterleaved` 结尾（`checkPages` 自己跑 `checkLinks` 与 `checkSearch`）；A 跑 M9 的 14 个各 10 遍 `-race`，140 次都过。`AGENTS` 的那一个原没有结尾的检查（A-D6 的三处只有它成立）：`8a251f14` 让 `TestTheInstructionsFollowTheAGENTSPage` 以 `checkPages` 结尾 |
| 第 12 条：最后一跳与扩展点 | 满足（收尾补上两处测试） | `TestTheMCPToolsBuildBothServers` 与每个工具的到达的测试；`mcp:<名>` 到 `changesets.client`；附件读、路径端口、工作区端口；prompt 的替身 `mcpserver/prompt_test.go` 的 `TestThePromptDoubleIsListedAndReached`（第 8 节的文字改正，C-N2）。`batch` 的客户端名称只以握手的名称核对过（A-D3）：`5c236ba6` 让 `TestTheClientNameIsTheTokens` 每一步另发一个 `batch`；组合根交给 `BatchLinks` 的 `Lead.Ambiguous` 由 A-F1 的第 25 例钉住；A 的 48 个接线的变异见"反向对照" |
| 第 13 条：真实客户端 | 实现的一半满足；人工清单待执行 | MC1 经 `@modelcontextprotocol/sdk` 1.32.1 连接；[人工清单](../manual/P5-mcp-clients-checklist.md)写好（50 步，Claude Code 与 Codex），用 README 的样例。README 的两句客户端的说法没有标出来处、也不在 05-P5 4.2（B-I1 = C-M11）：`741519d0` 把 :209 改写、客户端怎样显示标"未核对"，:208 改说服务端的；4.2 收尾加两行（:209 与 B-M4 新标的 :341）；之后 README 里客户端的每一句在样例里、标着"照文档"或"未核对"，或由仓库的记录核对过（B 的助手对过 4.2 的每一行与它点名的每一步）。清单在 v0.1 的最后执行（第 11 节第 98 条、取舍第 12 条） |
| 12.5：用 PAT 完整操作 | 满足 | MCP 只收 PAT；M9 没有新的 REST 操作（`git diff fff7c560..1f7fa54c -- api/`：新的 `api/mcp/tools.json`，`page.yaml`、`asset.yaml`、`common.yaml` 的描述与 `transfer.yaml` 的 `revert_too_large`；收尾另改 `startRevert` 的描述）；`TestEveryOperationAcceptsAPersonalAccessToken` 照旧由契约推导 |
| 12.5：扩展点建好，有测试证明注册者能挂上；注册的已注册 | 满足 | 建立：MCP 的工具与 prompt 的注册表（`mcpserver.Tool`、`mcpserver.Prompt`、模块的 `Tools()`、组合根的 `mcpTools` 对着 `tools.json`）：`TestTheToolListsAreTheContract`、`TestTheMCPToolsBuildBothServers`、`TestUnnamedToolsCatchesEachGap`、`TestNewRefusesItsWiringFaults`、prompt 的替身（整个程序的最后一跳交 M10）。注册：变更集的客户端 `mcp:<名>`（上面第 12 条） |
| 12.5：先写描述，再写代码 | 满足 | 每个 Phase 文档（写全它的工具在 `tools.json` 里的条目）早于它的第一个代码提交：P1 `7f03403c` < `d2955e9f`，P2 `79895f87` < `c87ab688`，P3 `c661ee97` < `1995157d`，P4 `f4e2da46` < `5608b274`，P5 `171b739d` < `14287a43`；`tools.json` 随 Step 长出，由整个程序的测试比较，没有 `-update` |
| 12.5：架构测试、depguard、前端静态检查 | 满足 | SDK 的导入规则（`archtest/rules_test.go` 与它的反向对照）；A 的 `make lint-go`、`gen-check`、架构测试、deadcode，B 的 `make lint-web knip test-web build-web` 都过；修复之后实施者的 `make lint knip gen-check test-web build`、核实修复 A 的 `lint-go`、`gen-check-go`、deadcode 与 B 的 `pnpm check:format`、`lint-web`、`knip` 都过 |
| 12.5：本 M 没有 `open` 的 handoff | 满足（收尾后，一个照记下的例外） | 收到的九份里八份改为 `done`（下面"handoff"）；M0/P1 照负责人的规则留着 `open` 到人工清单跑过（第 11 节第 98 条），那也是 M9 改为已完成的时候，所以 12.5 与文档约定的"一个 M 完成前，不能留有 open 的 handoff"都成立；写给 M10、M11、M12 的补齐（C-I1、C-M1） |
| 人工验收：真实客户端的清单 | **待执行** | 结果记在下面"人工验收"一节；通过之前 M9 是"实现完成，待人工验收"，不改为已完成（第 11 节取舍第 12 条）；输入法清单没有 M9 的步 |
| 文档约定的"M 完成" | 满足（收尾后，待人工验收） | 五个 Phase 各有审查记录；本记录、00 号文档、移交、总体设计在收尾时完成；README 在 `741519d0` 照代码改 |

## 发现与处置

级别照 M9 各 Step 的记录：Important 记为高或中高，Minor 记为中低或低，Nit 记为细节。代码、测试与 README 的发现照主持者的决定（负责人可以改判）在分支 `m9-close` 上改，提交写在各格的处置里；文档的发现在这次的提交里改（见下"文档与代码的不一致"）。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| C-I1 = A-F4 | 高（移交） | M9 给后面的 M 的守卫与回滚的事实不在任何 `handoffs/` 里：00 号文档"写给后面的 M"说由发现它的 Phase 写进目标 M、收尾核对，而 M10 的移交只有 P1 的几条（它自己写着"之后的 Phase 写进来，M9 收尾时核对"）：来源区的守卫要认得的几种写（一步里同时的移动与改名、batch 的合成的正文、撤销的临时名称只凭名称认得、先恢复再写回）与 `lint`、`pending_sources` 两个工具都没有；给 M11 的（`upgrade_verify`、`upgrade_submit`，冻结的守卫放行发起人的 MCP 的写与回滚的撤销，回滚经 `schedule()` 与 `ErrRevertUnordered`）没有一份。M10、M11 照自己的移交写守卫时会把撤销的临时的一步、batch 的合成的一步当作用户的改名与写。同 M8 收尾的 C-I2 | 文档：[M10 的移交](../../M10-llm-wiki/handoffs/M9-P1-mcp-registry.md)加第 7 条（来源区的守卫）、第 8 条（两个工具），开头一句改写；新的 [M11 的移交](../../M11-upgrade/handoffs/M9-mcp-upgrade.md)（四条）；三份交出它们的审查记录（P3 的 S2、S3，P4 的 S2）链过去；00 号文档"写给后面的 M"写明收尾补上 |
| B-I1 = C-M11 | 中高（完成标准第 13 条） | README.md:209 说每个客户端把名额的 503 显示成传输的错误，没有标"照文档"或"未核对"，不在 05-P5 4.2，而 README.md:337 说同一件事对 Claude Code 与 Codex 未核对；README.md:208 的"MCP 客户端不会续期"同样没有标（照构造成立：服务端没有 OAuth 续期的路） | `741519d0`：:209 改为"这是 HTTP 的答复，不是工具的结果（官方 TypeScript SDK 的客户端把它当作传输的错误；Claude Code、Codex 怎样显示未核对，见下文'HTTP 的答复'）"；:208 改说服务端的（"服务端没有 OAuth，经 MCP 续不了期"）。05-P5 4.2 收尾加两行（:209 由清单 F1 核对；B-M4 新标的 :341 由 A4、B4 与 E1 核对） |
| A-F1 | 中低（测试） | 组合根在 linking 的 `Resolution` 与 page 的 `Lead` 之间抄 `Ambiguous`（`bootstrap/linking.go` 的 `linkLinksBefore`、`pageLinksBefore`），没有测试走到：去掉任一处（变异 C31、C31b），名字带 batch、链接、改写、索引、反链、撤销的组合根的测试全过。那时 P 写下同时指向 `A/Notes` 与 `B/Notes` 的 `[[Notes]]` 再把 A 改名为 C 的 batch 写成 `[[C/Notes]]`（分开的调用不动它），之后以 `old_string "p [[Notes]]"` 编辑它答 `validation_failed`。P3 的设计审查 A-M3 加了这个字段，它的变异"`Lead` 只留 id"瞄着 domain 的 `TestBatchPlanContents`，那里的 lead 是测试自己建的 | `2017c402`：`TestTheLinksOfABatchAreThoseOfSeparateCalls` 加第 25 例，即 A 的探针的三种（写下之后改它的文件夹的名、改另一个文件夹的名、写下改名再写），各断言一个变更集；C31、C31b 各让三种都失败。03-P3 的第 51 条变异一行加注 |
| A-F2 | 中低 | `grep` 的命中的文字是行的一段，行是 `GrepContents` 读出的整页的一段：至多 1,000 字节的命中让它的整页（至多 5 MiB）一直活到答复写完，一次调用至多留着 `MaxGrepRead`（32 MiB），不是 4.13、P4 与 M12 的移交说的"同时至多一批正文"；名额是 `max(1, GOMAXPROCS/2)`，18 核上 9 个调用约 330 MiB 对设计的约 45 MiB。A 的探针：8 页各 4 MiB、各一个命中，结果留着 32.0 MiB，加 `strings.Clone` 之后 0.0 MiB。同 P1 的 S3 审查 A-I1 在说明的缓存里修的 | `6b174115`：`GrepHitOf` 的两支都 `strings.Clone`；`TestGrepsHitsHoldTheirTextsAlone`（page/app，8 页各约 4 MiB，内容照 pgx 读出的那样各是新的字符串，整行与截断各四个，结果留着的在一批的四分之一之下）与 `TestGrepHitOf` 的"不在行的内存里"；变异 F2a、F2b 各留着 16 MiB、两个测试都失败。A 的存疑（`search` 的摘录、`index` 的摘要）核对过：两者都由 SQL 先截，不留着正文，不改 |
| A-F3 | 低（测试） | page 模块在组合根不给 `GrepSlots` 时自己定的 `max(1, GOMAXPROCS/2)` 个名额与 `GOMAXPROCS` 个解析的轮次（4.13 与 P4 的设计审查 A-I1 的进程的界）没有测试读：改成 65,536（C37、C38），名字带 grep、名额、解析、界的 page 与组合根的测试全过 | `5dad1549`：`app.Grep.Turns` 经 `page/export_test.go` 的 `Module.GrepTurns` 读；`TestTheModulesGrepTakesTurnsByItsCores` 在 `GOMAXPROCS` 1、7、18 下得（1, 1）、（3, 7）、（9, 18）；C37、C38、名额不减半、轮次减半四个变异都失败 |
| A-D3 | 低（测试） | 4.5 说最后一跳的测试以两种名称的来源各写一页、各做一个 `batch`；`mcp_names_test.go` 只做 `write` 与测试的 `echo`，batch 的客户端只以握手的名称核对过 | `5c236ba6`：`TestTheClientNameIsTheTokens` 每一步另发一个写一页的 batch（握手的、新的应用读回的、`_meta` 里说的 Codex、清理过的 128 个字符），断言变更集是 `mcp:<名>`；变异 C43 从握手那一步起每步都失败 |
| A-D6 | 低（测试） | 4.15 说每个交错以 `checkPages`、`checkLinks`、`checkSearch` 结尾；A 点了三处没有 | 三处里两处是 A 看错：`checkPages` 自己在末尾跑另两个（`interleavings_page_test.go`，M8/P5A 起），`mcp_writes_test.go` 的改名再新建一例与 `revert_job_test.go` 的六个子测试本来就有三个（核实修复 A 认了）。`AGENTS` 一例一个都没有：`8a251f14` 让 `TestTheInstructionsFollowTheAGENTSPage` 以 `checkPages` 结尾，假件上的对应没有数据库，注释写明 |
| A-D1、A-D2、A-D4、A-D7 | 低、中低、低、低（文档） | 4.1 的许可证漏了 BSD-3-Clause 与 MIT No Attribution；4.3 说停机的 20 秒长于一次调用的 15 秒，`/mcp` 的大的请求体至多 45 秒（D2）；4.10 说 `edit` 先判定 `page.write` 再找路径，代码先找路径（404）再判定（403），4.15 与代码一致；第 8 节漏了 M9 改的 M4、M6、M5 的三处（00042、00047、页面事件） | 00 号文档照代码改（下"文档与代码的不一致"）；D2 另是第 11 节第 108 条：停机时还在上传的调用被切断、回滚，M12 的部署一条写明 |
| A-D5、A-D8–A-D28 | 细节（文档） | 22 处措辞、遗漏的成员、例外与过时的引文（各条在 A 的记录里；D28 是 4.1–4.13 的过时的行号与已删的名字） | 00 号文档照代码改；行号照下"文档与代码的不一致"的规则 |
| B-M1 | 中低 | 一个 128 个字符的 MCP 客户端名称在 375 px 上把历史页撑到 1,027 px（"客户端"的下拉 1,003 px 宽，`history-filters.tsx` 的 `NativeSelect` 没有宽度的界线）、把它的变更集的页撑到 712 px（`changeset-page.tsx` 的"Client: MCP (…)"一行没有 `break-words`）；真实浏览器复现两次。代码是 M8 的（P3B），M9 让任何 agent 给出这样的名称。B 试过的改法让两页都放得下 | 主持者的决定（第 11 节第 105 条）：照负责人对小的体验问题的规则，进 [M12 的打磨移交](../../M12-release/handoffs/M5-polish.md)第 32 条，连同 B 试过的改法与 MC3 的一个 375 px 的检查；分支上不改 |
| B-M2 | 中低（README） | README.md:212 把 batch 的 100 页、8 MiB、20,000 条链接写成跟着改名、移动时才有的；`BatchPlan.CheckUnit` 与 `UnitTooLarge` 对任何写了不止一页的 batch 都数（两页各 4.5 MiB 的 `edit` 被拒） | `741519d0`：照 B 的写法改：写了不止一页时这些页最后的正文合计至多 100 页、8 MiB、20,000 条链接，跟着改名、移动的计划的解析也算在里面，另合计至多 10 MiB |
| B-M3 | 低（README） | HTTP 的答复的表把一个 SDK 不认得的请求体（空的、方法名写错、少了 `id`、`jsonrpc` 不是 "2.0"、`tools/call` 没有 `params`）的纯文字 400 也说成"请求头不对" | `741519d0`：纯文字的一行加这些情形，"照下面的 `curl` 写这几个头与请求体"；JSON 对象的一行说空的请求体见下一行；另照 B-N12 写 `Accept` 的通配 |
| B-M4 | 低（README） | 没有令牌的 401 也是空的或形状不对的 `Authorization`（`Bearer` 之后是空的、别的方案、令牌里有空白），README.md:341 没写；令牌的变量在那个 shell 里没设时客户端照样连、服务端说去加已经配了的头 | `741519d0`：照 `bearerToken` 写全，处置加"确认启动客户端（或执行 `claude mcp add`）的 shell 里设了这个变量"，"变量没有设时客户端发出什么"标"未核对"（05-P5 4.2 的一行，清单 A4、B4、E1） |
| B-N1 | 细节（测试） | 网页对着 vitest 的 18 个变异里三个存活，都是 M8 的代码：地址里的 `client` 被转成小写（w07，所有测试的名称都是小写）、中文的"全部 MCP"（w10）、没有改名的变更集不该有"改名回去"的提示（w12） | `703993cf`：以 `?client=mcp%3AZed` 读到并选中 `mcp:Zed`；中文的测试读客户端下拉的六个选项；只移动、不改名的变更集没有那条提示；三个变异各在新的断言处失败（核实修复 B 重跑） |
| B-N2–B-N12 | 细节（README） | README 的 MCP 各节的十一处：:208 的续期（B-N2，同 B-I1）、batch 整体的界线不带 `op`（N3）、没有键的模式走查每一页、`building` 为假（N4）、内容相同的 `write` 不看令牌与锁（N5）、`edit` 重做时又被写过答 `page.revision_mismatch`（N6）、去掉 frontmatter 之后是空的 `AGENTS` 页（N7）、删除至多列 20 个被持着的页（N8）、nginx 的三行（N9）、限额表里正文与 `new_string` 合计（N10）、48 KiB 只数文字、内联的图另算（N11）、名额满了的日志只在有人被拒时记与 `Accept` 的通配（N12） | `741519d0`：各照代码改；核实修复 B 把改过的 26 行逐句对着 `1f7fa54c` 与代码（16 行改内容，10 行只是 oxfmt 的对齐），每个去掉的说法都换成改正的，别的没有少 |
| C-M1 | 中低（移交） | P4 第 8 节交出的三条只写了一部分：给 M10 的 `index.md` 承袭 `ListIndex` 的界线与代价、`grep` 看不到虚拟的文件；给 M12 打磨的 `index` 的三个筛选的 `""` 的 422；给 M12 性能的被标出的比例在真实的库上的持平点（每 10,000 页约 25 页带 200 KB 的 base64 图片） | 文档：M10 的移交第 3 条、[打磨移交](../../M12-release/handoffs/M5-polish.md)第 30 条、[性能移交](../../M12-release/handoffs/M4-performance.md)第 18 项各补上 |
| C-M2 | 中低（文档） | 00 号文档"总体设计的修订"列的几项没落到总体设计：13.1 第 4 条 batch 的码的次序与 `page.batch_too_large`，第 11、21 条 `linking.NewBatchLinks` 与 `BatchLinks` 的最后一跳，第 3 条 `page.batch`、`index.list` 的规则行，第 27 条 `grep` 的 `limit` 与游标，13.4 第 4 条 P3 的交错 | 总体设计补上（下"第 13 节"） |
| C-M3 | 低（文档） | 总体设计与 M9 的代码相反的三句：3.8 的类型里有"批量"（batch 的变更集是 `edit`）；3.11 的"SSE（以及 MCP）"（`/mcp` 经 `API.Wrap`，只有事件流是长连接）；13.4 第 8 条说成本那一份 20 分钟（`ci.yml` 是 25）、10 万个候选的调用 450 ms（P5 期间改为 1 s） | 总体设计改正 |
| C-M4 | 中低（文档） | 00 号文档"总体设计的修订"的 11 一项写着 00046 之外的行"由升级之后的 reindex 重建，代价同 00041（约 16 分钟、130 GB 的 WAL）"；同一份的 4.2、第 10 节与 README 说不入队、不要 reindex、之后第一次 reindex 重建版本 4 的行约 26 分钟、290 GB：照前者计划的运维人员少算一半多的 WAL | 照 4.2 与第 10 节改写 |
| C-M5 | 低（文档） | 00 号文档第 12、13 节漏写两个合并：`c0636ec3`（M1 的故事的摆动，P1 时发现）与 `9d991781`（守卫的时限，P5 期间）；第 12 节 P1 一格也没有 `0c934bb9` | 第 12 节的合并一栏补上三个，第 13 节 P5 的 S1 一行写出 `9d991781` |
| C-M6 | 中低（决定） | 两个负责人可以推翻的决定不在第 11 节：CI 的"Go 的默认时限的一半"的规则（P2 的设计审查的决定 14，写在 02-P2 6.5 与总体设计 13.4，不在第 11 节与 P2 的 2.1）；接受最坏的 batch 约 16 秒、过了期限、答 500（第 10 节说"接受（主持者的决定）"，出自 03-P3 2.1 第 16 条与 P3 的设计审查） | 第 11 节第 109 条 |
| C-M7 | 低（文档） | "移交的落实"的 M8/P5 一行写"整词不取"，4.13 与代码（`grep_keys.go` 的 `classWord`，`TestGrepKeys` 的 `^ai$`）在模式给出两边的边界时取 1–2 个字符的整词：照它抄进移交就写反了 | 那一行先改，再写进移交 |
| C-M8 | 低（文档） | 第 3 节第 7 条仍说 batch"写完立即搜得到"，第 44 条与 4.11 改为批量入队、答 `building`，`TestTheMCPBatchReachesItsUseCase` 断言的是后者 | 第 7 条改写 |
| C-M9 | 中低（测试） | 第 3 节第 8 条把"恢复要等父节点一线先上移（层级）"列在真的数据库上测的形状里；`TestTheRevertOfEachBatchShape` 没有这一行，等待只在 domain 的 `revert_schedule_test.go`（`MaxDepth` 3）；4.12 自己的数据库的清单也没有它 | `14dec63c`：先核对，真的没有。domain 加 `MaxDepth` 10 下的两行，`TestTheRevertOfEachBatchShape` 经请求撤销两者（请求是一个单元、不逐步检查，分不出等待），`TestTheRevertsJobWaitsForAParentsLineToMoveUp` 以每批一个节点撤销两者，任务成功；变异 M7c、M7d 让任务停在 `too_deep`，M7b 任务与 domain 都失败，M7a 只有 domain 的失败（第 107 条）；4.12 的清单补上 |
| C-M10 | 低（测试） | 第 3 节第 1 条的"更新的版本照 SDK 的协商降到它"没有服务端一侧的测试：`TestTheMCPNegotiatesItsVersion` 用 Go 的客户端（它自己退回）与 2025-06-18 | `ed7e70e4`：`TestTheMCPNegotiatesALaterVersionDown`（组合根第三份）以 2026-07-28 与 2099-01-01 握手，各答 2025-11-25，之后的调用答 `echo`；变异 M6b（最新的版本降到 2025-06-18）失败，M6a（服务 2026-07-28）不让它失败（SDK 的 `initialize` 从不答 2026-07-28 起的版本），由 `TestTheMCPNegotiatesItsVersion` 抓到，注释写明 |
| C-M12 | 中低（契约） | `api/modules/transfer.yaml` 的 `startRevert` 仍说 M8 的后台撤销（每批重新排步、深度优先恢复、停下的原因里没有 `revert_too_large`）；P3 的 S3 起开始时排一次、只在段之间切批、每批先 L 再按次序核对前提、一组一次恢复、一段过了一批以 `revert_too_large` 停下 | `c2f0bcd8`：照建成的改写，`make gen`（没有变异）只改了这一段描述（`api/dist/openapi.yaml`、`schema.gen.ts`）；核实修复 A 对着 `RevertBatch` 与任务核对 |
| C-M13 | 中低 | batch 的锁的预检对一页被编辑的页的第一个正文操作答 `page.locked`，而计划里写一页已有的页总是正文操作；单独的 `write` 写这一页已有的字节答 `changed: false`、不看锁（`mcp_lock_test.go` 的第 7 格与 C4 断言它）。agent 在 batch 里重发一页没改的正文而有人在编辑它时整个 batch 被拒，分开调用都成功；安全（从不写错），但 4.11 的那一条规则没列它 | 文档：照 C 的第一种改法写进 4.11 的规则，作为写明的不同；代码不改（第 112 条；C 的另一种，为计划里没有改动的 `write` 跳过预检，负责人可以要）。`tools.json` 的 `batch` 的说明是代码，收尾的分支没有带它：下次改 `batch` 的说明时一并写进；README 的 batch 一条在收尾的文档提交里改为"与分开调用的不同在两处"，补上这一处 |
| C-M14 | 低（文档） | 00 号文档的四处实施之前的说法：4.1 的 `mcpTools(pg, ln, nb)`（是 `(pg, as, ln, nb)`，P1 的 S2 的偏离 12）；4.2 的迁移的表没有 00042 的一行；第 5 节说 `api/` 只多 `revert_too_large`（另有 `common.yaml`、`page.yaml`、`asset.yaml` 的描述）；4.16 引的 `README.md:365-367` 在 P5 之后是别的 | 照代码改 |
| C-N1 | 细节 | 05-P5 7.2 引的总体设计的替换文字里三个相对链接按 05-P5 的位置解析不到（它们是给 `v0.1-design.md` 的） | 写成代码 |
| C-N2 | 细节 | 第 8 节说 prompt 的替身在"模块根上"；它在 `mcpserver/prompt_test.go`，没有模块给 `Prompts()` | 改为 mcpserver 上 |
| C-N3 | 细节 | 第 11 节第 64 条引的 `BOOTSTRAP_THIRD = …|Every[M-R]` 是 P4 之前的目标；P4 之后是第四份，`Makefile` 里两种都没有（这次挪动没有发生） | 去掉过时的正则，写明没有发生 |
| C-N4 | 细节 | M10 的移交第 5 条写 `maxOtherBytes`，常量是导出的 `MaxOtherBytes` | 改正 |
| C-N5 | 细节 | 4.11 与"移交的落实"的 M6/P4 一行说"M6/P4 实测 5,000 页到 15 秒的期限""提速……在 M12 的性能移交第 3、18 项"：第 3 项是 M6 的解析预算，没有这次计时，也没有提速 | 两处改为指第 18 项，写明没有提速（移交许可） |
| C-N6 | 细节（流程） | M9 的提交写成 `<类型>(M9/P<n> S<m>): …`（277 个代码提交），31 个文档的提交用文档约定留给 M5 之前的 `docs(M9/…)`，不是"Git 工作方式"的写法；M5–M8 同样 | 主持者的决定（第 11 节第 111 条）：记下、不改写历史，文档约定加一句，从 M10 起在它的设计里定一种 |
| C-N7 | 细节 | P4 的 S2 的审查记录把日期写成 2026-10-23（实为 10-22 至 10-23）；04-P4 的基线写 `56ef34c8`，设计的提交的父是 P3 的文档的提交 `bbd18d82`（代码相同） | 两处改正 |
| C-N8 | 细节 | 状态的写法：M4–M8 写"进行中（待负责人执行输入法清单）"，M9 定的是"实现完成，待人工验收"；00 号文档的头、第 12 节与总体设计 12.6 要一起用一种 | 三处都写"实现完成，待人工验收"；文档约定的"进度追踪"加一句这种状态 |
| C-N9 | 细节 | 第 11 节第 52 条的"第 2 轮之后 3.0–3.4 秒"（记录是 3.1–3.4，03-P3 两种都有）；第 9 条说 `list` 照树的次序"同网页"，没写附件按 `sort_order` 穿插、网页按名称列的不同（P1 的 S2 审查记下的） | 两条改正，03-P3 随记录 |
| C-N10 | 细节 | 总体设计 13.1 第 10 条没有 4.15 说的"batch applied"一行 info、"changeset reverted"的 `aside` 计数与"revert segment past a batch"的 warn | 补上 |
| C-N11 | 细节 | "移交的落实"的 M8/P4 一行说"四个整个程序的测试"（P3 的 S3 的修复加了第五个）；4.12 引的 `store_test.go:757` 现在是 `TestADeletionsItemHoldsItsPagesRevision` | 改为六个（收尾加了第六个）；4.12 写测试名 |
| C-N12 | 细节 | 已不存在或从没有过的名字：4.12 的 `batchOps`、`stepKey`（P3 的 S3 删去）；03-P3 的 `holderOf`/`holder`、`errReplan`；04-P4 的 `riskyTails`；01-P1 的 `ParseVersion`、`api.gen.go` | 照代码改名，或写明原来的 |
| C-N13 | 细节 | 文件末尾之外的引文（00 号文档一处、01-P1、02-P2、04-P4 各一处）与 00 号文档 54 处过时的行号；约 55 处写"今天"的引文在 `fff7c560` 上对、在 HEAD 上指别的代码 | 见下"文档与代码的不一致"的行号一段 |
| C-N14 | 细节 | 00 号文档 4.x 对着代码的九处措辞（`NewBatchLinks` 的签名、期限一行的"request id"、客户端走了一行、`ToolError` 的 `Fit` 与 `At`、各工具的答复少的成员、`edit` 的两句文字、内置的说明的"约 1–2 KB"、`list` 的 422 也在游标之前、清单的慢的发送者的步） | 照代码改，与 A 的同一处的一起写一次 |

## 疑问与判断

审查者的存疑（没有核实的）里有了判断的：

| # | 疑问 | 判断 |
|---|---|---|
| A 的存疑 1 | `grep` 的第一行与第一批不受预先判断（D17），解析的队列在调用的 ctx 里等：等了大半个期限的调用可能在软期限之后才开始第一行，答 `internal_error` | 修复的第 11 项以探针量：成立，但答的是正常的成功，不是 `internal_error`。一页一行（`MaxContentBytes` − 1 字节，工作 74,998,482），解析的轮次被占到期限只剩 2 秒、1.2 秒、800 ms 时在期限之前 1,303、475、101 ms 答，剩 400 ms、100 ms、20 ms 时在期限之后 293、599、809 ms 答，60 秒的写超时之内写得出去；负载下的界约 1.6–2 秒；要每个轮次都被占约 12–14 秒。核实修复 A 同意不改：改它是改 P4 3.6.3 的进度规则。主持者的决定（第 11 节第 106 条）：接受，界写进 [M12 的性能移交](../../M12-release/handoffs/M4-performance.md)第 18 项，4.13 写明 |
| A 的存疑 2 | 别的工具会不会像 F2 那样以一段留着整页：`search` 的摘录、`index` 的摘要 | 修复核对过：摘录是 SQL 的 `substr(…, 162)`，`index` 读 `value_start`（前 257 个字符）与 `left(tag, 512)`，pgx 为每行建小的字符串，不留着正文；不改（核实修复 A 同意） |
| B 的存疑 | 由从右到左的字母组成的客户端名称可能让行里的"MCP（名）· 5 分钟前"重排（客户端不在 `<bdi>` 里） | 没有探；显示名从 M8 起同样，是全应用的一处，写进打磨第 32 条的末句 |
| C 的存疑 1 | `page.New` 不拒绝空的 `Deps.BatchLinks`：第 12 条的"组合根交空时整个程序的测试失败"可能只经行为成立 | 成立：A 的变异 C21（`BatchLinks` 为空）由 `TestTheLinksOfABatchAreThoseOfSeparateCalls` 与 `TestTheWaysABatchDiffersFromSeparateCalls` 抓到（第一个计划链接的 batch 碰上空的接口、恢复为 `internal_error`）。不在 `page.New` 里另加 panic |
| C 的存疑 2 | `Unit.markMoves` 在一个后台任务的各单元之间取或，00045 按最后合并的条目补值：一个后来的批把前面的批的改名改回时两者会不同 | 段的规则让一个节点的临时的步到它最后的放回在同一批里，任务里的改名与改回不跨批，所以取或与合并的结果相同；没有复现，不改 |
| C 的存疑 3 | Go 的 `unicode.Bidi_Control` 与 00044 的显式范围在用的工具链上是否一致 | `migrations` 的 `TestChecksRejectCounterexamples` 逐码位核对，A 的 `make test-go` 与持续集成都跑了它，绿 |
| C 的存疑 4 | 01-P1:25 把 `fff7c560` 叫作"M8 收尾"，它是 M8 的 Codex 评审的文档的提交（代码相同） | 改正 |
| C 的存疑 5 | 第 13 节 P5 的设计一行说"不先跑清单是主持者的决定"，第 11 节第 98 条说是"负责人的规则" | 两者都对：清单在 v0.1 的最后跑是负责人的规则，P5 不在合并之前先跑是主持者照它定的；第 13 节是历史，不改 |

## 负责人可以推翻的决定

各条的理由在所指的文档里（第 11 节的条号；`取舍 n` 是第 11 节末的设计的取舍）。负责人在 M9 开工时定下四件事（2026-10-17：历史对 agent 只读、batch 的变更集在网页上整组撤销、v0.1 里 MCP 不开后台任务；编辑锁一次性、不能接管；根下的 `AGENTS` 页是说明；`grep` 之外加 `search`，接入写进文档，网页上的地址等打磨；第 11 节开头负责人的四条）。下面是作者、主持者与实施者定下的，C 按主题汇总（117 条，每条只在一个主题里），这里只写要点：

- **传输与协议**：无状态的 Streamable HTTP 与 JSON 答复，JSON-RPC 的批在 SDK 之前拒绝（取舍 1）；协议版本只到 2025-11-25，2026-07-28 等真实的客户端，`listChanged: false`、`private` 的缓存，名称取 `clientInfo.name` 或任何请求的 `_meta`、记在 PAT 上（第 2 条）；`clientInfo.title` 留给打磨（第 3 条）；README 把工具的拒绝写成码、`MCP-Protocol-Version` 照 go-sdk v1.8.0 按版本写（第 101 条）；无状态时通知什么都不做、取消只来自客户端结束请求，`2026-07-28` 起的未知版本过得了 SDK 的头的检查（[P1](../01-P1-platform-reads.md) 2.1 第 16 条，[P1 的 S1 审查](P1-S1-platform-review.md)）。
- **认证、边缘与请求体**：`/.well-known/` 之下都答 404（第 1 条）；`POST /register` 答 OAuth 的错误带 PAT 的提示（第 13 条）、没有令牌的 401 带同一句（第 14 条）；超过 64 KiB 的只能是这个入口登记的工具的调用（第 15 条）；整个服务两个、每个凭证一个大的请求体，等 2 秒答 503（第 16 条 ⚠）；请求体的 JSON 值的上限（第 35 条）；最高一次 +961 MiB 读作约 1 GB 之内（第 36 条 ⚠）；只收 PAT（取舍 3）；每个凭证一个名额的代价与读请求体的 `encoding/json/v2`（P2 的设计审查，第 10 节，总体设计 13.1 第 32 条）。
- **客户端名称与说明**：名称的来源（取舍 2）；说明的缓存存副本（第 24 条）、说出的名称立刻记住（第 25 条）、两个客户端的镜像的交错不加锁（第 26 条 ⚠）、留给打磨的三处（第 27 条）；内置的说明只有英文，几个进程不互相作废缓存（[P1](../01-P1-platform-reads.md) 2.2，P1 的 S3 审查）。
- **路径、`list`、`read` 与附件**：从根精确寻址、四种情形、glob 三种通配（取舍 6）；48 KiB、内联的图片、`server.public_url`、`index` 的字符串 256 字节（取舍 7）；`list` 照树的次序，附件按 `sort_order`（第 9 条，收尾写明与网页的不同）；00042 的两个部分唯一索引（第 17 条 ⚠）、没有统计时的计划（第 18 条 ⚠）；没有 `public_url` 时图片照样内联（第 19 条）、WebP 的宽高（第 20 条）、`MaxGlobWork` 3 亿（第 21 条）、`versions` 的 422（第 43 条）；自己写的 WebP 头的读法、glob 的工作答游标、`open_notebook` 的最后改动（[P1](../01-P1-platform-reads.md) 2.1）。
- **写、版本令牌与锁**：MCP 的删除被任何活着的会话拒绝（取舍 4）；不透明的版本令牌（第 6 条）；`move` 的父页写成页的路径答 422（第 32 条）；CRLF 的规则（第 37 条）；内容相同的 `write` 不看令牌与编辑器、`edit` 的重叠计数与线性的匹配、`move.to` 是完整的路径、一步与两步的显示的不同（[P2](../02-P2-writes-history.md) 2.1）。
- **历史的读**：`log` 的 `actor: "me"`（第 10 条）；按页的日志的代价不改代码、结构的改法交 M12（第 40 条 ⚠）；`log {path: ""}` 留给打磨（第 41 条）。
- **batch 的语义与界线**：不许删除自己新建、改名、移动的，5 MiB、100 项、1 万个条目，重新计划一次（取舍 5）；与分开调用的不同只是 4.11 的一条规则（取舍 13，收尾加 C-M13 的锁的一处）；写了的页的链接按它最后一个正文操作时的树（第 4 条）；规范化先删子孙再删祖先（第 5 条）；422 的字段名（第 33 条）；批量入队与 `building`（第 44 条）；冻住的节点（第 45 条）；改写的页数、三个新的界线与精确的检查（第 46、47 条 ⚠）；读整页的 `write`（第 48 条）、新建的节点不在触及里（第 49 条）、改写碰到被编辑的页只在单元里查到（第 50 条）、计划一次只留一棵树（第 51 条）、接受的几种用时（第 52 条 ⚠）、页面事件只列活着的（第 60 条）；最坏的 batch 约 16 秒答 500（收尾补成第 109 条 ⚠）。
- **多轮撤销与临时名称**：临时名称取 id 的后 8 位，放不进一批的段以 `revert_too_large` 停下（取舍 10）；上下相套的回收站项照旧拒绝（第 56 条）；(T1) 的判断与构造的测试（第 57 条）；两份正文在最小的预算下（第 58 条）；空的主干照页的名称（第 59 条）；轮数的上限从下面钉住（第 62 条）；守卫只凭名称认得临时的步（第 65 条，收尾写进 M10 的移交第 7 条）；第三条后备只作防御（[P3](../03-P3-batch-revert.md) 2.2）。
- **后台撤销**：S2 的临时的拒绝（第 61 条）、记下不改的两件（第 66 条 ⚠：00045 在启动时持着 `changesets` 的 ACCESS EXCLUSIVE）；紧凑的计划（第 67 条）、段里的恢复截到放回的目标父节点（第 68 条）、各条界线的哨兵（第 69 条）、按计划的大小在读之前拒绝（第 70 条）、照 S\* 的名称称呼冲突（第 71 条）、开始时列全部、之后只列第一个（第 72 条）、子树的次序（第 73 条）、前提的反向对照（第 74 条）、等笔记本的批持着预算（第 76 条）。
- **grep、search 与 00046**：`grep` 在 Go 里以 RE2 按行、各种界线、不取键的有界地走查（取舍 8）；一次一本笔记本（取舍 9）；没有"穷尽"模式（第 8 条）；被截掉的文字与 00046（第 11 条 ⚠）；被标出的比例的规则与结果（第 12 条，持平点收尾写进 M12）；"满负载"是短时的（第 77 条 ⚠）、`MaxGrepLineWork`、`GrepLineOverhead`（第 78、79 条）、队列与名额是 channel（第 80 条）、一行一次 NFC（第 81 条）、取键读 8,192 个原子（第 82 条）、最坏的解析记下不拒（第 83 条 ⚠）、00046 的 Down 再 Up（第 85 条 ⚠）、`GrepInsts` 的上界（第 86 条）、模式按 NFC（第 88 条）、每单位的计时与预热（第 92 条）；解析的队列只等、不答 503（[P4](../04-P4-grep-search.md) 2.1）；久等之后的第一行在期限之后答（收尾第 106 条）。
- **index、backlinks 与 00047**：读的界线在写的地方，00047 的 Up 重写 `page_properties`（第 90 条 ⚠）、生成列与 `char_length`（第 91 条）、建成的偏离（第 94 条）、记下不改的两件（第 95 条）。
- **CI 与测试的规则**：每个工具的码一张表（第 22 条）、守卫那一份的时限与第一份的界线（第 23 条）、组合根的分份只凭两次运行的平均（第 28 条）、任务的等待（第 30 条）、第三份（第 31 条）、`-short` 的门禁（第 38 条）、C4 的摆动记在记录（第 39 条）、两处摆动在 Step 里改（第 42、53 条）、现在不做的（第 54 条）、River 的 leader（第 55 条）、测试的加强（第 63 条）、第一份过线时挪哪些（第 64 条）、`TestStartRevertAtMaxJobItems` 的红（第 75 条）、块数的测试（第 84 条）、`main` 的链接的时间测试在 `-short` 下跳过（第 87 条）、不加第四份（第 89 条）、`index` 与 10 万个候选的守卫（第 93、102 条）；"Go 的默认时限的一半"的规则（收尾补成第 109 条）；第一份不挪（收尾第 110 条）；只改测试的 `c87caad7`、下一个往 `page/domain` 加性质测试的先在竞态下量、`ci.yml` 的 pnpm 的存储的路径（P1 的 S1、P4 的 S1、S2 的审查记录）。
- **故事与 e2e**：豁免 MCP 版本的故事（取舍 11）；一个故事经 TypeScript 的 SDK（第 7 条）；MC1 的范围（第 29 条）；样例 047 的 README 一句（第 34 条）；P5 的四处取舍（第 96 条）；第 236 条变异在抓得到的地方抓（第 103 条）；API 版本的洞（第 104 条）。
- **README 与人工清单**：清单与输入法清单一样在 v0.1 的最后由负责人执行，通过之后 M9 才改为已完成（取舍 12）；Caddy 在 P5 定下，早答的拒绝经 Caddy 成 502 交 M12（第 97 条 ⚠）；清单跑过之前 README 的客户端的句子各标来处、M0/P1 照旧 `open`（第 98 条）；Codex 的登录的次序（第 99 条 ⚠）与清单自己的安全（第 100 条 ⚠）；每个客户端一个令牌的变量（[P5](../05-P5-stories-docs.md) 2.1 第 13 条）。
- **收尾新加**（第 11 节第 105–112 条）：
  - 窄屏上长的客户端名称进 M12 的打磨第 32 条，连同 B 试过的改法（第 105 条，B-M1）；
  - 久等解析的队列之后 `grep` 的第一行在期限之后答，接受，界进 M12 的性能第 18 项（第 106 条）；
  - 变异 M7a 只由 domain 的测试抓到，记下、不改（第 107 条）；
  - 停机的 20 秒短于 `/mcp` 一次调用的 45 秒，4.3 改正、M12 的部署一条写明（第 108 条，A-D2）；
  - 补两条可以推翻的：CI 的"Go 的默认时限的一半"、最坏的 batch 约 16 秒答 500（第 109 条，C-M6）；
  - 组合根第一份不挪：`main` 最近两次 7:17、7:07（第 110 条）；
  - M9 的提交写法记下、不改写历史，从 M10 起在设计里定一种（第 111 条，C-N6）；
  - batch 里与页的字节相同的 `write` 碰上被编辑的页整个被拒，写进 4.11 与 README，代码不改（第 112 条，C-M13）。
- **收尾的另几处判断**（没有条号，写在所指的地方）：A-D6 只留 `AGENTS` 的一例（核实修复 A：另两处是审查者看错）；B-M1 不在分支上改（第 105 条）；M0/P1 的移交照旧 `open`（第 98 条）。

⚠ 是运维人员看得到的（升级的锁、5xx、内存的峰值、期限、凭证）。

## 接受与推后

- **推到 v0.1 之后的打磨**（负责人 2026-10-03 的规则；[M12 的打磨移交](../../M12-release/handoffs/M5-polish.md)）：长的客户端名称在 375 px 上（第 32 条，B-M1，连同 B 试过的改法、MC3 的检查与 B 的从右到左的存疑）；`index` 的三个筛选的 `""` 的 422，与 `log {path: ""}` 一起（第 30 条，C-M1）。
- **推后到 M12 的性能移交**（[第 18 项](../../M12-release/handoffs/M4-performance.md)）：久等解析的队列之后的第一行与它的界（第 106 条）；被标出的比例在真实的库上的持平点（C-M1）；组合根第一份的数（第 110 条）与 A 量的守卫那一份的余量；00042 的两个唯一索引在大的实例上的用时（C 的风险）；`grep` 一次调用的内存改正（A-F2）；37,814,272 的算法（加 64 KiB）与一处错字（C 没有评级的两处）。
- **推后到 M12 的转移移交的部署文档**（[第 6 项](../../M12-release/handoffs/M7-transfer.md)）：停机的 20 秒与 `/mcp` 一次调用的 45 秒（第 108 条）。
- **交 M10、M11**（C-I1、C-M1）：见下"handoff"。
- **记下、不改**：C-M13 的锁的一处不同（4.11）；M7a（第 107 条）；M0 的冒烟故事 S1 的摆动（下"没能验证的风险"；仓库没有摆动的清单，同第 39 条记在这里）；`tools.json` 的 `batch` 的说明没有写 C-M13 的那一处（代码，下次改它时写）。

## 第 13 节

写进[总体设计](../../v0.1-design.md)，同时改了别的几节：

- **13.1 后端**：第 1 条 `Unit.Relocate`（MCP 的 `move`，单个与 batch 里都是一步、一条 `RelocateNode`；C 对照总体设计的表的细节）；第 2 条清理的谓词写 `shared.Unshowable`（同 A-D25）；第 3 条 `page.batch` 与 `index.list` 的规则行，第 4 条 `batch` 的码的次序与 `page.batch_too_large`，第 11 条 `linking.NewBatchLinks(pool, maxContent, logger)` 接成 `Deps.BatchLinks`，第 21 条 `BatchLinks` 的最后一跳（`page.New` 不拒绝空的，A 的变异 C21 由两个整个程序的测试抓到），第 27 条 `grep` 的 `limit` 1–500、默认 100 与它的游标（路径的键与行号）（C-M2）；第 10 条"batch applied"、"batch planned again"、"changeset reverted"的 `aside` 与后台撤销的 warn"revert segment past a batch"（C-N10）。
- **13.4 测试**：第 4 条 P3 的五个交错（`interleavings_batch_test.go` 的三个、`TestTheMultiRoundRevertMeetsAWrite`、`TestTheBackgroundRevertStopsAtAWriteBetweenItsBatches`）在 P2 的七个旁边（C-M2），与两次握手之间换掉 `AGENTS` 页的顺序的测试（A-D6）；第 6 条 P2 的 S2 之后的 MCP 矩阵的行（`batch`、`grep`、`search`、`backlinks`、`index`；C 的表的细节）；第 8 条成本那一份的时限 25 分钟、10 万个候选的调用建成时 450 ms、P5 期间改为 1 s，各说一次（C-M3）。
- 同时：3.8（变更集的类型去掉"批量"：batch 的变更集是 `edit`）、3.11（长连接的路由只有事件流，`/mcp` 不是）（C-M3）；12.4（`batch` 把参与者推到全部操作之后一次；工具的清单由测试对着 `api/mcp/tools.json` 核对，组合根只拼接，A-D24；其余的照第 8 节核对过）；12.6（M9 实现完成，待人工验收，C-N8）；第 15 节补上收尾的一行。00 号文档"总体设计的修订"列的别的各项在审查时都已落到（C 的表）。

## handoff

- **收到的**：[M4/P1 客户端的检查](../handoffs/M4-P1-client-check.md)、[M4/P2 一个单元里先建后删](../handoffs/M4-P2-unit-merge.md)、[M5 编辑锁与 MCP 的写](../handoffs/M5-locks.md)、[M6/P4 链接改写](../handoffs/M6-P4-rewrite.md)、[M7 附件与 MCP](../handoffs/M7-assets.md)、[M8/P3 batch 的整组撤销](../handoffs/M8-P3-revert.md)、[M8/P4 MCP 开的后台任务](../handoffs/M8-P4-background.md)、[M8/P5 grep 用搜索的索引](../handoffs/M8-P5-search.md)逐项对照代码改为 `done`，写明各 Phase 的落实（C 的逐项的处置；M4/P1 第 4 项、M4/P2 的空的条目、M8/P3 第 4 项与 M8/P4 第 1 项照设计的决定与移交要的不同，各自写明）。[M0/P1 MCP 会话](../handoffs/M0-P1-mcp-sessions.md)写上处理进展、照旧 `open`：第 2 项（无状态）、第 4 项（`/mcp` 经 `API.Wrap` 的逐路由中间件、只收 PAT、`no-store`、恢复 panic）做完；第 1 项（Claude Code、Codex 在重启之后）等人工清单的 G 部分，第 3 项（各审批选项的效果）等 D 部分，清单跑过之后照记下的关。00 号文档"移交的落实"各行写明收尾，M8/P5 一行先改正（C-M7）。
- **M9→M10**：[M10 的移交](../../M10-llm-wiki/handoffs/M9-P1-mcp-registry.md)加第 7 条（来源区的守卫要认得的四种写：一步里同时的移动与改名、batch 的合成的正文、撤销的临时名称只凭名称认得与 `Step` 上可以带的标记、先恢复再写回）、第 8 条（`lint`、`pending_sources` 照 M9 的每个工具登记）；第 3 条补 `index.md` 承袭的界线与代价、`grep` 与虚拟的两页（C-I1、C-M1）；开头一句改写，第 5 条的 `MaxOtherBytes`（C-N4）。
- **M9→M11**：新的[升级的 MCP 工具、冻结的守卫与回滚的调度](../../M11-upgrade/handoffs/M9-mcp-upgrade.md)：两个工具、冻结的守卫放行发起人的 `mcp:` 的写与回滚的撤销（按 `Kind`，不按名称）、回滚经 `PlanRevert` 的 `schedule()` 而不在 `ErrRevertUnordered` 上重试、MCP 开任务先放开 `transfer_jobs.client`（C-I1）。M8 写的[回滚一次升级](../../M11-upgrade/handoffs/M8-P4-background.md)照旧。
- **M9→M12**：[性能](../../M12-release/handoffs/M4-performance.md)第 18 项加久等之后的第一行（第 106 条）、被标出的比例的持平点（C-M1）、第一份的数（第 110 条）与守卫那一份的余量、00042 的建索引在大的实例上没有量过（C 的风险），改正一次调用的内存（A-F2）与 37,814,272 的算法；[打磨](../../M12-release/handoffs/M5-polish.md)第 30 条加 `index` 的 `""`（C-M1），新的第 32 条（第 105 条，B-M1）；[转移](../../M12-release/handoffs/M7-transfer.md)第 6 项的部署文档一条加停机（第 108 条）。三份交出这些的审查记录（[P3 的 S2](P3-S2-revert-review.md)、[S3](P3-S3-background-review.md)、[P4 的 S2](P4-S2-index-backlinks-review.md)）链到它们。
- `M9-mcp/handoffs/` 只剩 M0/P1 一份 `open`，照第 98 条。

## 文档与代码的不一致

照代码改写：

- **[M9 总设计](../00-M9-design.md)**：
  - 头与第 12 节的状态"实现完成，待人工验收"（C-N8）；第 3 节第 7 条的搜索照第 44 条（C-M8）。
  - 4.1：新加的模块的许可证（MIT、Apache-2.0、MIT No Attribution 与 BSD-3-Clause，照各自的 `LICENSE`，A-D1）；账户入口加上 `notebook`、笔记本入口不是"去掉"（A-D8）；`mcpTools(pg, as, ln, nb)` 与 `NewBatchLinks(pool, maxContent, logger)` 的签名（C-M14、C-N14）；`ListChanges` 是读，不是用例（A-D27）。
  - 4.2：00042 的一行（两个部分唯一索引取代 00012 的兄弟的索引，建时挡住 `nodes` 的写，没有量过，先停旧的服务；C-M14）；00046 的入队不看分词的版本（A-D9）；只从可信的代理读 `X-Forwarded-For`（A-D27）。
  - 4.3：停机的 20 秒短于 `BodyLimits` 的路由的 45 秒，那时还在上传的调用被切断、回滚（A-D2，第 108 条）。
  - 4.4：bodyshape 的计数在 `json.Valid` 之后的第二遍、名额的 503 也是 problem+json、`rateLimit` 在 `limit.go`（A-D27、A-D28）。
  - 4.5：最后一跳的测试点名 `TestTheClientNameIsTheTokens` 与它每一步的 batch（A-D3，`5c236ba6`）；清理在 `shared/client_name.go`，谓词是导出的 `Unshowable`（A-D25）。
  - 4.6：入口的判定只对 POST、"没有"从不缓存、不命中时读存储的 `PageContent`（A-D27）；内置的说明至多 4 KiB，不是"约 1–2 KB"（A-D10）。
  - 4.7：期限一行的文字带"; request id <id>"、错误的 `structuredContent` 也算进 48 KiB、`read` 与 `open_notebook` 以 `TextBytes`、`MemberBytes` 填、客户端走了仍建答复、`ToolError` 另有 `Fit` 与 `At`（A-D11、A-D12、A-D27、C-N14）。
  - 4.8：路径的最后一段找不到时为 `.md.md` 的提示多一次查（A-D13）。
  - 4.9：`read` 自己读存储的 `PageContent`（A-D14）；附件的第一块另有 `kind`、`url`、`expires_at`、`note`（A-D15）；`list` 的 `hint`（A-D26）；`list` 的附件按 `sort_order` 夹在页之间、网页按名称列（C-N9）。
  - 4.10：`edit` 先读笔记本、找路径（404），再判定 `page.write`（403）（A-D4）；`edit`、`move` 答 `changed`，`delete` 答 `path` 与 `node_id`（A-D16）；`edit` 的两句文字照代码（A-D27、C-N14）。
  - 4.11：与分开调用的不同加 C-M13 的锁的一处；"只有一页时不限"去掉（`CheckReads` 没有这一支，A-D19）；单个 `move` 的 5,000 页是 M6/P4 的数，界线的建议在 M12 的第 18 项（C-N5）。
  - 4.12：一批列出 L 的全部冲突，没有时续接的全部冲突，再没有时第一个前提不成立的步（A-D5）；移到根下的临时的一步也记改名（A-D20）；`batchOps` 与 `stepKey` 在 P3 的 S3 删去，步按序号与 `seq`（C-N12）；合并留删除的版本号的测试名（C-N11）；真的库上的形状加层级的等待（`placeLine`、`restoreLine`），M7a 只由 domain 抓到（C-M9，第 107 条）。
  - 4.13：命中的文字是副本，一次调用同时至多一批正文（A-F2，`6b174115`）；第一行与第一批不受预先判断，久等解析的队列之后第一行可以在期限之后答（A-D17，第 106 条）；`x{n}` 给 n 份原子（A-D18）；`edit` 的"not found"的文字（A-D27）。
  - 4.14：`grep` 一行的豁免与读的界线"只有一页时不限"去掉（A-D17、A-D19）。
  - 4.15：重新计划的测试点名（A-D21）；`list` 的模式的 422 也在游标的 400 之前（C-N14）。`AGENTS` 一例的结尾的检查由 `8a251f14` 补上，句子不改（A-D6）。
  - 4.16：README 的上传与导入的一句在 `README.md:564-565`（C-M14）；清单的慢的发送者是 F1、F2 的写法（C-N14）。
  - 第 5 节：答复一栏补上 `list` 的 `updated_at` 与 `hint`、`batch` 的 `changed`、`building`、`results_count`、`search` 的 `changed_at`、`index` 的 `cut`（A-D26）；`api/` 没有新的操作，一个新的取值，几处描述改了（C-M14）。
  - 第 8 节：M9 改的前面的 M 的代码补上 00042、00047 与页面事件只列单元之后活着的页（A-D7）；prompt 的替身在 mcpserver 上（A-D22 = C-N2）；`mcpTools` 只拼接，比较是 `TestTheToolListsAreTheContract`（A-D24）；参与者照 4.11 的 (a)–(e)（A-D23）。
  - 第 9 节：最后一跳与 `grep` 的内存的测试名；第 10 节：停机、久等之后的第一行、00042 的三行（第 108、106 条，C 的风险）。
  - "移交的落实"、"写给后面的 M"、"总体设计的修订"、第 11、12、13 节：见上（C-I1、C-M1、C-M4、C-M5、C-M7、C-N3、C-N5、C-N9、C-N11 与第 105–112 条）。
  - **行号**：第 1–12 节的 232 处 `路径:行` 逐处对过（照写它的那次提交的代码找到所引的文字，再在 `f214cb77` 里找）。说建成的代码的 72 处改到 `f214cb77`（A-D28、C-N13 与另找到的）：三处因代码拆开写成两段（`list_changesets.go`、`backlinks.go`、`content.go` 的 `DecidedAll`），附件的读改为 `asset/app/blobs.go:125-143`、`index` 的查询改为 `index.sql:1-84`（A-D28），文件末尾之外的 `list_notebooks.go:27-50` 改为 `:27-49`，`store_test.go:757` 改为写出测试名（C-N11），README 的一处见 4.16；64 处原就对，C-N13 列的 `workspace/memberships.go:22-26` 与 `platform/httpserver/api.go:221-258` 也在其中（C 的脚本对着 `fff7c560` 比，这两个文件之后改过，引文随着改过）。说 M9 改它之前的代码的 38 处（"今天……""曾……"）照写时留着：31 处在 `fff7c560` 上对，`jobs.go:168` 差一行、改为 `:169`，P3 的设计写的 6 处是 `c661ee97` 的（其中 4.12 的两处随 C-N12 删去）。SDK、Go 的标准库、jsonschema-go 与 Nerve 等仓库之外的 54 处不随仓库变，没有对；头里的 4 处是写法的例子。00 号文档头里的"代码位置的写法"写明这两种行号。C-N13 给的新行号有几处差一两行，照代码，不照它的表。
- **Phase 文档**（各自的行号照它头里写的提交）：
  - [P1](../01-P1-platform-reads.md)：`fff7c560` 是 M8 的 Codex 审查的文档的提交（C 的存疑 4）；`ParseVersionToken`、`asset/adapter/http/gen/server.gen.go`（C-N12）；`sign.go:45-60`（C-N13）。
  - [P2](../02-P2-writes-history.md)：`links_test.go:215-247`（C-N13）。
  - [P3](../03-P3-batch-revert.md)：`domain.HolderOf`（S1 起；设计的 `holder(...)` 没有写过，原引的行在锚的提交上也不对）、`*replanned`（C-N12）；"第 2 轮之后 3.1–3.4 秒"随它的记录（C-N9）；第 51 条变异的一行与 3.27 的同一处加 A-F1 的第 25 例（C31、C31b）。
  - [P4](../04-P4-grep-search.md)：基线是 P3 的文档的提交 `bbd18d82`，代码同 `56ef34c8`（C-N7）；`riskyTail`（C-N12）；`links.ts:8-58`（C-N13）。
  - [P5](../05-P5-stories-docs.md)：4.1 与 5.1 照收尾的 README 改（`741519d0` 改的那几行），开头各一句说明、与 `f214cb77` 的 README 的那几段逐字节相同；4.2 加 `README.md:209` 与 `:341` 两行（B-I1、B-M4），开头与末段各一句；7.2 的三个链接写成代码（C-N1）。
- **审查记录**：[P3 的 S2](P3-S2-revert-review.md)、[S3](P3-S3-background-review.md) 与 [P4 的 S2](P4-S2-index-backlinks-review.md)交出的几条链到 M10 的移交第 3、7 条与 M11 的移交第 3 条（C-I1）；P4 的 S2 的日期（C-N7）。
- **移交**：见"handoff"；M10 的 `MaxOtherBytes`（C-N4）；M12 的性能移交的 37,814,272 的算法与一处错字、转移移交的开头补上 M9 加的（C 没有评级的几处）。
- **[总体设计](../../v0.1-design.md)**：见"第 13 节"。
- **[文档约定](../../../README.md)**：进度追踪加"实现完成，待人工验收"（C-N8）；"Git 工作方式"加 M9 的提交的写法（第 111 条）。
- **契约**（在分支上，`c2f0bcd8`）：`startRevert` 的描述（C-M12）。
- **README**（在分支上，`741519d0`）：B-I1、B-M2–B-M4、B-N2–B-N12。

## 反向对照

审查者做的：

- **A**：组合根与跨 Phase 的接线 48 个变异（计划的 47 个，另加 C31 的镜像 C31b），各经 `heavy.sh` 跑点名的测试：42 个失败；2 个等价（C04 改了模块的次序：go-sdk 按名称排序列出工具；C45 让 `NodePaths.Pages` 也问死了的节点：之后的循环只留 `paths` 里的）；4 个存活，写成 F1（C31、C31b：`Lead.Ambiguous` 不抄）与 F3（C37、C38：65,536 个名额与解析的轮次）；存活的另对名字相关的全部组合根、page 的测试再跑一次（C31 一组 217 秒）。失败的组：`mcpTools` 拼接（C01–C03）、附件的端口的错误映射（C05–C07）、凭证、工作区与名称（C08–C11）、边缘（C12–C15）、大的请求体（C16、C17、C19、C20）、名称的端口（C18、C47）、`BatchLinks`（C21、C23–C27、C32、C34、C39，与界线的 C28–C30）、`NodePaths`（C33、C44）、事件的登记（C35、C36）、搜索的队列（C40）、客户端名称的最后一跳（C41–C43）、说明（C46）。
- **B**：网页 18 个变异对整个 vitest，e2e 4 个对 M9 的故事（各先 `make build`）。网页的 15 个失败（客户端的文字、`mcp:` 的切法、"全部 MCP"、选中的客户端留在选项里、撤销的确认框的链接的提示、`revert_too_large` 的文字与它的"留下了什么"、行与变更集的页的客户端），存活 3 个（w07 地址的 `client` 转小写、w10 中文的"全部 MCP"、w12 移动也给"改名回去"的提示），写成 B-N1，都是 M8 的代码；e2e 的 e01、e02 失败，e03（w07 经 MC3）存活，e04（没有读到的 MCP 客户端不在选项里）MC3 抓不到、vitest 的 w05 抓到。M9 在 `web/` 写的没有一处存活。
- **C**：文档的核对，没有变异。

修复做的（改动全部还原；`make gen*` 从不在变异下跑）：

- **F2a、F2b**（`GrepHitOf` 的整行一支、截断一支不 `strings.Clone`）：各留着 16 MiB，`TestGrepsHitsHoldTheirTextsAlone` 与 `TestGrepHitOf` 都失败。
- **C31、C31b**：每一个都让第 25 例的三种都失败（第三种单独跑也失败）。
- **C37、C38**，名额不减半、轮次减半：`TestTheModulesGrepTakesTurnsByItsCores` 都失败。
- **C43**（batch 的适配器不交名称）：`TestTheClientNameIsTheTokens` 从握手那一步起每步失败。
- **M6b**（最新的版本降到 2025-06-18）：新的测试失败；**M6a**（服务 2026-07-28）不让它失败，由 `TestTheMCPNegotiatesItsVersion` 抓到。
- **M7a–M7d**（`canPlace` 去掉层级的界、`canRestore` 去掉、日程提前放回或恢复 X 而保留前提的检查）：M7c、M7d 让任务停在 `too_deep`、任务的测试失败；M7b 任务与 domain 都失败；M7a 数据库的两个测试都过，只有 domain 的失败（第 107 条）。
- **w07、w10、w12**：各在新的断言处失败。
- A-D6 的一处是加断言、C-M12 是描述，没有变异。

核实修复做的：A 在自己的快照上重跑 12 个（F2a、F2b、C37、C38、M7a 对 domain、C31、C31b、M7c、M7d、C43、M6b 失败，M7a 对数据库的测试通过，写成细节、第 107 条）；B 重跑 w07、w10、w12，都失败。

## 核实修复

收尾审查时的门禁见上（A：`make lint-go`、`gen-check`、架构测试、deadcode、`make test-go` 838 秒；B：`make lint-web knip test-web build-web`、`make e2e` 两遍、M9 的故事三遍与降速的一遍），都通过；持续集成以 `main` 在 `1f7fa54c` 上的 run 39927680178 为据，全绿（A、C 读不到运行的日志：要权限，A 的一次读答 `Server Error`；结果由主持者给出）。修复之后由 A、B 各在自己分离的快照上核对，一轮没有留下 Important 与 Minor。

**修复**（分支 `m9-close`，从 `1f7fa54c` 开始，10 个提交 `6b174115`…`741519d0`）：

- **A-F2，grep 的命中钉住它的页**（`6b174115`）：`GrepHitOf` 给的命中的正文原是行的一段，至多 1,000 字节的一段让整页的内容一直活到答复写完，一次调用至多 `MaxGrepRead`（32 MiB），不是 4.13 说的一批。改为 `strings.Clone`，同反链的上下文。`TestGrepsHitsHoldTheirTextsAlone`（page/app）8 页各约 4 MiB、各一个命中（整行与截断各四个），内容照 pgx 读出的那样各是自己的字符串，答复留住约 10 KB，界是一批的四分之一；`TestGrepHitOf` 另查命中的正文不在行的内存里。变异 F2a、F2b（两处各去掉 `strings.Clone`）各留住 16 MiB，两个测试都失败。A 的疑问另核对：`search` 的摘录与 `index` 的摘要由 SQL 先截（162 个字符；`value_start` 257、标签 512），不钉住内容，不改。
- **A-F1，组合根的 `Lead.Ambiguous`**（`2017c402`）：组合根在 `Resolution` 与 `Lead` 之间抄 `Ambiguous`（`linkLinksBefore`、`pageLinksBefore`），没有测试走到；少了它，批把同时指向 `A/Notes` 与 `B/Notes` 的 `[[Notes]]` 在 A 改名时改写成 `[[C/Notes]]`，分开的调用不动它。`TestTheLinksOfABatchAreThoseOfSeparateCalls` 加第 25 例，即 A 的三种（写了再改它的文件夹的名；再改另一个文件夹的名；写、改名、再写）。变异 C31、C31b 各让三种都失败。
- **A-F3，grep 的进程的界**（`5dad1549`）：组合根不给 `GrepSlots` 时模块给 `max(1, GOMAXPROCS/2)` 个名额、`GOMAXPROCS` 个解析的轮次，原没有测试读它们（各 65,536 时全过）。`app.Grep.Turns` 经 page 的 `export_test.go` 的 `Module.GrepTurns` 读；`TestTheModulesGrepTakesTurnsByItsCores` 在 GOMAXPROCS 1、7、18 下得 1 与 1、3 与 7、9 与 18。变异 C37、C38 与名额不减半、轮次减半的两个都失败。
- **A-D3，客户端的名字经 `batch` 的最后一跳**（`5c236ba6`）：`TestTheClientNameIsTheTokens` 原只做写。现在每一步另发一个写一页的批（批必须带 `message`），断言变更集是 `mcp:<名字>`：握手的、新的应用里记下的、`_meta` 里说的 Codex、清理过的 128 个字符。变异 C43 从握手那一步起每一步都失败。
- **A-D6，交错的结尾的检查**（`8a251f14`）：A 点的三处有两处不成立：`checkPages` 自己在末尾跑 `checkLinks`、`checkSearch`（M8/P5A 起，`interleavings_page_test.go:109-110`），`mcp_writes_test.go:318` 的一例与 `revert_job_test.go` 的六个子测试已经三个都有，不改。`AGENTS` 页在两次握手之间被换掉的一例一个都没有，`TestTheInstructionsFollowTheAGENTSPage` 现在以 `checkPages` 结尾；假件上的对应没有数据库，注释写明。
- **C-M10，协议版本的协商**（`ed7e70e4`）：`TestTheMCPNegotiatesALaterVersionDown`（组合根第三份）以 2026-07-28 与 2099-01-01 发 `initialize`，各答 2025-11-25，之后的调用答 `echo`。变异 M6b（最新版本降到 2025-06-18）失败。M6a（服务 2026-07-28）不让它失败：SDK 的 `initialize` 从不答 2026-07-28 起的版本；`TestTheMCPNegotiatesItsVersion` 抓住它，注释写明。
- **C-M9，层级的等待在真的数据库上**（`14dec63c`）：先核对，没有。原有的测试只撤销一步或只测冲突，等待只在领域里（MaxDepth 3）。加领域的两行，在 MaxDepth 10 下（`placeLine`、`restoreLine`）：九层的 X 等 P 从 Q 之下回到根才放回 P 之下；X 等 B 回到根才恢复到 B 之下。`TestTheRevertOfEachBatchShape` 经请求撤销两者，回到批之前；请求是一个单元、不逐步检查，分不出等待。`TestTheRevertsJobWaitsForAParentsLineToMoveUp`（第三份）以每批一个节点撤销两者，任务成功。变异：M7c、M7d（日程提前放或恢复 X、保留前提的检查）任务停在 `too_deep`，失败；M7a（`canPlace` 去掉层级的界）数据库的测试通过（结尾一样），只有领域的失败；M7b（`canRestore` 去掉）任务与领域的都失败。
- **C-M12，契约**（`c2f0bcd8`）：`startRevert` 原说 M8 的任务（每批重新排步骤、深度优先恢复、没有 `revert_too_large`）。照 P3 S3 起的实现改写：开始时排一次，照次序写，只在段之间切批，每批先比自己的节点、再查各步的前提；冲突照它列的，一段超过一批时 `revert_too_large`，回收站一项的成员在一批里一起恢复。`make gen`（没有变异）只改了这一段描述。
- **B-N1，网页的三处测试的缺口**（`703993cf`）：w07：以 `?client=mcp%3AZed` 读到并选中 `mcp:Zed`。w10：中文的测试读客户端下拉的六个选项。w12：只移动、不改名的变更集不显示"改名回去"的提示。三个变异各在新的断言处失败。
- **README**（`741519d0`；B-I1 = C-M11、B-M2–B-M4、B-N2–B-N12）：各先照代码核对。503 改为"不是工具的结果（官方 TypeScript SDK 的客户端把它当作传输的错误；Claude Code、Codex 怎样显示未核对…）"；15 分钟改说服务端的；另改批的单元的界与解析的 10 MiB、纯文本的 400 与 `Accept` 的通配、没有令牌的 401 与 `Bearer` 的形状、没有键的模式与 `building`、`changed: false` 与 `edit` 的重做、空的 `AGENTS` 页、删除列的 20 页、nginx 的三行、限额表、48 KiB 与内联的图、名额满时的日志。新标"未核对"的两句：503 怎样显示（清单 F1）；变量没有设时客户端发出什么（清单 A4、B4）。`pnpm check:format` 通过。
- **A 的另一个疑问，解析队列里久等之后的第一行**（探针，不改）：疑问成立，但不是 A 担心的 `internal_error`。一页一行（`MaxContentBytes`−1 字节，功 74,998,482，约 0.7–0.8 秒），解析队列一个轮次被占到 15 秒的期限只剩若干时：剩 2 秒、1.2 秒、800 ms 都在期限之前答复；剩 400 ms、100 ms、20 ms 在期限之后 293 ms、599 ms、809 ms 答复，都是正常的成功，`write_timeout` 60 秒写得出去。要碰上它，解析队列要被占约 14 秒，看不出寻常的路（最慢的解析没有量）。不改：第一行的豁免是设计的进度规则（P4 3.6.3、A 的 D17），改它是设计的改动，留给主持者与负责人；超出的至多一行的功，与设计已接受的同一个界。

门禁：本机 `make lint knip gen-check test-web build`（vitest 166 个文件、3,344 个测试）通过；`heavy.sh` 里 `make test-go` 818 秒（组合根三份 177、218、299 秒）、`make test-go-short` 58 秒、`make e2e` 282 个，都通过；HEAD 之下的九个提交各自检出，`lint-go` 与 `test-go-short`（契约的另加 `gen-check`，网页的 `lint-web`、`test-web`）都通过；两个新的整个程序的测试以 `go test -race -list` 核对落在第三份，第一份没有加。分支的持续集成 run 39945210705 在 `741519d0` 上全绿（作业，括号里是测试一步）：web 4:04（3:29）、组合根一 6:25（6:03）、组合根二 6:15（5:57）、组合根三 5:24（5:01）、竞态 9:41（9:22）、成本 9:09（6:52）、守卫 13:13（12:49）、short 3:23（3:03）、e2e 4:10（E2E 一步 3:08）、镜像 1:23。

流程（照实写下）：实施者一次在 `/tmp` 下写了一个暂存文件，立刻删掉；一次误用的 `git stash` 立刻取回；A-D6 那一项在推送之前重做（原先照 A 的三处都改，读了 `checkPages` 之后只改 `AGENTS` 一处，备份的分支随后删掉）；实施者的报告的文件被工具拒绝写入，由主持者照它交回的保存。

**核对**（`741519d0`）

- **A**（服务端）：两个分离的快照，一个跑测试、一个跑变异，都在 `741519d0`，没有取回任何东西。
  - 门禁：`heavy.sh` 里 `make test-go` 通过，16 分 4 秒（开始时别的会话让负载到 26.4，组合根三份的包 281、324、401 秒）；`make lint-go`（两个模块 0 个问题，`go mod tidy -diff` 干净）、`make gen-check-go` 干净、deadcode 不带 `-test` 132 行（同 `1f7fa54c`，`Grep.Turns` 不在其中）、带 `-test` 0 行。新的单元测试：`TestGrepsHitsHoldTheirTextsAlone` 20 遍、与 `TestGrepHitOf` 在 `-race` 下 5 遍，`TestTheModulesGrepTakesTurnsByItsCores` 在 `-race` 下 3 遍，page 与 page/app 整个在 `-race` 下，都通过。
  - 各项：第 1–4、6–8 项照代码修对，变异失败；A-F2 的修复之外没有别的地方留着内容的一段（`Grepped.Found` 只留 `GrepHit`，不完整的行不留文字，游标与路径是建出的字符串）；`TestTheModulesGrepTakesTurnsByItsCores` 不并行、在 `Cleanup` 里还原 `GOMAXPROCS`，Go 只在包的顺序的测试之后才跑并行的，不碰别的测试；第 5 项 D6 的三处有两处是 A 看错（没有打开 `checkPages`），实施者不改那两处是对的，记录里 D6 只留 `AGENTS` 一例；第 7 项另记一个细节：M7a 两个数据库的测试都过（第 107 条），任务的测试可以另断言步的次序；第 11 项同意不改：超出的界是一行的时间加第一批的读，期限只标记 ctx、一次过期的答复仍在 60 秒的写超时之内，碰上它要解析的队列几乎整个调用都满着，改它是改 P4 3.6.3 的规则（审查 A-m3 有意定的），建议把量得的超出与负载下的 1.6–2 秒写进 M12 第 18 项、4.13 写明（照做）。
  - 每个提交：只在 HEAD 跑过数据库的测试（实施者的 `make test-go`、持续集成与 A 的）；每个 Go 的提交只加或改它自己的测试，看不出哪个单独是红的；没有逐个提交在数据库上核对。
  - 结论：Important 0、Minor 0、细节 1（M7a），D6 在记录里更正。
- **B**（网页、端到端与 README）：一个分离的快照在 `741519d0`。
  - 第 9 项：w07、w10、w12 对整个 vitest 重跑，各被新的测试抓到（"地址里有大写字母的 MCP 客户端照原样读、选中"，"中文的历史：行的文字与名称，与可选的客户端"，"只移动、没有改名的变更集没有改名回去"）。
  - 第 10 项：`741519d0` 改了 README 的 26 行，逐行逐句对着 `1f7fa54c` 与代码：16 行改内容（131、208、209、211、212、214、216、273、327、341、346、347、368、369、500、571），10 行只是 oxfmt 重排的对齐（339、340、342–345、348–351）；每个去掉的说法都换成改正的，别的没有少。新加的客户端的说法只有两句，都标"未核对"（:209 怎样显示 503，清单 F1；:341 变量没有设时客户端发出什么，A4、B4 与 E1），在 05-P5 4.2 里还没有行（这次的提交加）；:208 是服务端的，:347 的"`curl` 默认的 `*/*`"是 curl 的。README 的 221–371 与 498–571 行不再与 05-P5 4.1、5.1 逐字节相同（这次的提交照改，见下）。
  - 门禁：`pnpm check:format`（683 个文件）、`make lint-web knip`、`make test-web`（166 个文件、3,344 个测试）通过；`heavy.sh` 里 `make e2e` 282 个、37.6 秒，通过。
  - B-M1 照决定不在分支上（`history-filters.tsx`、`changeset-page.tsx` 自 `1f7fa54c` 没改）；`web/` 只改了两个测试文件与随 `startRevert` 的 `schema.gen.ts` 的一行。
  - 结论：没有留下 Important 与 Minor。

合并 `f214cb77`（`Merge branch 'm9-close': the closeout's code, test and README fixes (M9 closeout)`；已推送）。合并之后 `main` 的持续集成 run 39950787055 在 `f214cb77` 上全绿（作业，括号里是测试一步）：web 3:08（2:40）、组合根一 7:26（7:07）、组合根二 6:32（6:12）、组合根三 5:49（5:25）、成本 11:48（9:01）、守卫 8:47（8:29）、竞态 9:22（9:02）、short 2:15（1:57）、e2e 3:50（E2E 一步 2:56）、镜像 1:18。

## 没能验证的风险

- **CI 上各个测试的用时**：作业的日志要权限，审查与核对都没有读到（A 的一次读答 `Server Error`）。守卫的余量只能按记录里的倍数估：A 在本机量的 M9 的守卫大多是时限的约三分之一（`grep` 最坏的一行 732 ms 对 2.3 秒、一次的工作 1.93 秒对 5.9 秒、10 万个 `GrepCandidates` 的中位数 85 ms 对 270 ms、守卫树上的 `grep` 427 ms 对 1.25 秒、glob 的守卫 663 ms 对 2 秒、界线处的 batch 3.35 秒对 11 秒、`ListIndex` 1.05 秒对 3.5 秒、`TestStartRevertAtMaxJobItems` 643 ms 对 2.1 秒；10 万个候选的一次调用 127 ms 对 1 秒、`edit` 158 ms 对 2 秒宽得多），按 `GrepCandidates` 的约 2.3 倍都过、余约三成，按 P3 的 S3 那台运行机的约 3.3 倍有七个会失败（最薄的是守卫树上的 `grep`、glob 的守卫与最坏的一行）。已记在第 75、93、102 条与 M12 第 18 项，不是新的发现。
- **真实的 MCP 客户端**：Claude Code、Codex、rmcp 都没有运行（照规则）；README 与清单里它们的说法照 05-P5 4.2 标着，由人工清单核对。B-M3、B-M4 的答复在两个客户端里怎样显示、变量没有设时它们发不发 `Bearer `：清单的 A4、B4 与 E 部分。
- **go-sdk v1.9.0** 去掉 `MCPGODEBUG`：那个版本不在模块缓存里，不下载。
- **迁移的用时与 WAL**：00045、00046、00047 在 30 万页或 600 万行上的数没有重量，`make test-go` 跑了它们自己大小的迁移测试；00042 的两个唯一索引的建在 `nodes` 上挡写，在大的实例上没有量过任何地方（C 的风险），README 的升级照"先停旧的服务"（00047 的那一条，一次 `migrate up` 跑 00042–00047），README 的升级说明没有点名 00042；写进 M12 的性能移交第 18 项。
- **A-F2 的堆**是经用例在返回新字符串的假件上量的，没有经 pgx 与 MCP 的适配器从头到尾量；机制相同，修复之后的测试同样是假件上的。
- **Docker 的端口的摆动**：A 的一次运行在测试开始之前因"mapped port 5432/tcp not found"起不来容器，重跑通过；在 CI 上出现会是准备的失败。
- **M0 的冒烟故事 S1 的摆动**（B 的 `make e2e` 第二遍）：`bootstrap/app.go` 在 `ListenAndServe` 旁的 goroutine 里启动任务，`/readyz` 在 HTTP 一听就答 200，River 的 `Start` 还没做完；故事一就绪就拔掉数据库，任务的启动失败按设计停掉服务（M1/P4 审查 M2）。M9 之前就有（`app.go` 的次序与 S1 都早于 `fff7c560`，M9 只加了 `mcpTools` 一个字段）；单独 `--repeat-each 20` 20 个都过，但碰上时 `make e2e` 失败。M9 不改：改法是故事等任务启动了（River 的启动记下，或 `/readyz` 等它）再拔数据库，交给下一个碰 S1 或启动次序的。
- **`1f7fa54c` 的持续集成**：审查时没有记录（A 说 `053d3ba1` 的是绿的、`1f7fa54c` 只改文档），主持者之后读到 run 39927680178 全绿。
- **README 的一句**：README.md:212 原说"与分开调用的不同只有一条规则"，C-M13 的锁的一处不在那条规则里；收尾的文档提交把它改为"与分开调用的不同在两处"，补上锁的一处（第 112 条）。

## 人工验收

[真实客户端的人工清单](../manual/P5-mcp-clients-checklist.md)由负责人在 v0.1 的最后与[输入法清单](../../M4-pages/manual/P6-ime-checklist.md)一起执行（负责人 2026-10-13 的规则；第 11 节第 98 条、取舍第 12 条）。输入法清单没有 M9 的步：M9 在网页上只改了撤销的轮次与 `revert_too_large` 的一条文字，都不涉及输入文字。**待执行。**

清单 50 步、九个部分，各部分写明在哪个客户端上做；"结果"一行定下通过：每一步不是"记下"的预期都成立，或者不成立的旁边写着负责人的处置（照 05-P5 4.2 改 README、交 M12，或改服务端）。跑过之后：

1. 整份结果写进这一节（下表的每一步每个客户端：通过、不通过或记下，原文照抄一两行）；
2. README 里客户端的句子照 [05-P5](../05-P5-stories-docs.md) 4.2 的"核对之后"一栏改，每个提交写明照哪一步；
3. [M0/P1 的移交](../handoffs/M0-P1-mcp-sessions.md)照 G、D 两部分的结果关闭（它的第 1、3 项），"移交的落实"的那一行随之；
4. M9 的状态由"实现完成，待人工验收"改为已完成：00 号文档的头与第 12 节的收尾一行、[总体设计](../../v0.1-design.md)的里程碑进度表与 12.6，第 15 节加一行。

| 部分 | 内容 | Claude Code | Codex |
|---|---|---|---|
| A1–A6 | Claude Code 接上：连接、说明、项目的范围、没有设的变量、账户入口、历史里的名称 | 待执行 | — |
| B1–B4 | Codex 接上 | — | 待执行 |
| C1–C15 | 读与写：`edit` 的语义、长的 `read`、图片与 3 MiB、20 张的上限、`grep` 的 48 KiB 与 `partial_lines`、`search` 与 `grep`、`batch` 与网页的撤销、`cut` | 待执行 | 待执行 |
| D1–D8 | 审批：Claude Code 的三种、Codex 的 `prompt`、`writes`、`auto`、`approve` 与 `codex exec` | 待执行 | 待执行 |
| E1–E6 | HTTP 的答复与提示：401 的提示、`/.well-known/` 与 `POST /register`、撤销的 PAT、会话的令牌 403、不认识的笔记本 404、可选的 429 | 待执行 | 待执行 |
| F1–F3 | 大的请求体的名额与慢的发送者 | 待执行 | 待执行 |
| G1–G2 | 服务重启之后不重新握手（M0/P1 第 1 项） | 待执行 | 待执行 |
| H1–H3 | 编辑锁，含"是你" | 待执行 | 待执行 |
| I1–I3 | 反向代理：nginx 按路由、负责人自己部署上的 Caddy（要 Docker）；I1 在 Claude Code 上，I2、I3 用 `curl` | 待执行 | — |
