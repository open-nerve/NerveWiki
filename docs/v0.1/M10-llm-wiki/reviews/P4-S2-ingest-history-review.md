# M10/P4 S2（ingest 与历史）：审查记录

| 项 | 内容 |
|---|---|
| 对象 | 分支 `m10-p4-s2`，base `c6dc643a`。前 11 个提交由 Codex 完成，到 `02567298`；之后的修复由 Claude 完成 |
| 方法 | 三位独立审查者，各在自己的 worktree 里、在真库上跑、改坏代码。A 管服务端的正确性，B 管数据、迁移与契约，C 管测试、完成度与工程。修复之后由原审查者核对 |
| 结论 | 生产行为上发现一处与设计矛盾（A-I1，已修），没有错标、漏标、回滚或锁序的缺陷。其余发现都在测试缺口、文档和工程上，全部处置 |

S2 建成了三件事。一个 `edit` 变更集新引用了原本未消化的来源时，在同一个写入单元里记下 ingest 标记；REST 与 MCP 的历史读出标记并能按它筛选；`log.md` 显示标记。P4 §3 是设计，这里只记审查。

Codex 的进度账本、门禁回执和每个决定的原始证据都不在 git 里，放在本机的 `~/.local/share/nervewiki-m10-evidence/m10/p4-s2/`。这份记录不复制它们的格式，只写结论。

## 发现与处置

| 编号 | 级别 | 发现 | 处置 | 提交 |
|---|---|---|---|---|
| A-I1 | Important | 已引用来源的页每次普通重存都全本扫描：候选没有减去旧 D_W，只要页里留着 `[[raw/S]]`，每次保存都读完全部 raw 节点，最后什么也不标。100k 节点时约 570 ms，无链接时 12 ms，都在 index 锁里 | `finish` 在 coverage 之前减掉 `allDirect`、`possibleDirect` 的候选，这些候选本来就不可能 novel | `0a669e0a` |
| A-M1 | Minor | 未改的 stale 来源：同一个旧状态，Q 未改时标 `[T, A]`，Q 在 W 时标 `[T]`；问设计取哪边 | 不改实现，补用例，写清 §3.9（决定 151） | `0a669e0a` |
| B-M1 | Minor | `log`、`changeset` 的工具描述没写来源是不分页的预览 | 改描述，两个 scope 同步 | `a6065120` |
| B-M2 | Minor | 00054 的 LZ4 前提、00053/00054 的锁没有写进部署与升级说明 | 补 README、总体设计 7.1、00 §4.2、P4 §3.10，用实测数字 | 文档提交 |
| B-Q1 | Q | 00054 对应哪个决定 | 决定 144，见下文 | — |
| C-I1 | Important | 设计 4.14 对根 AGENTS 与来源根的排除，三条腿六个分支改坏后测试抓不到 | 并入 `TestIngestBefore/exclusions`，`first_memo` 补 AGENTS 腿 | `ec542f7d` |
| C-I2 | Important | coverage 的 UnknownFParents 与"无 current 证明时 D_U=false"没有测试 | 同 A-M1 的两个用例 | `0a669e0a` |
| C-I3 | Important | 已知空目标、F 排除 frontmatter、非字符串槽三条分支改坏后存活 | 并入 `exclusions` | `ec542f7d` |
| C-I4 | Important | 替身接受真实存储拒绝的输入，分批逻辑没人测 | `fakeStore.Now` 加同样两条校验 | `ec542f7d` |
| C-I5 | Important，核对时撤回 | 5 个提交的 e2e 或 costs-lint 单独为红 | 不压缩提交，记下范围（决定 152） | — |
| C-M1 | Minor | 第四份两次分支样本 449、455 秒，均值过线；9 个新顶层落在它的补集 | 按名字移到第五份 | `cf510743` |
| C-M2 | Minor | costs-1、costs-2 的时限 25 分钟低于约两倍的规则 | 提到 35 分钟 | `cf510743` |
| C-M3 | Minor | `ingest-profile` 步骤只盖一份、上传二进制，已被决定 150 和 `Runner CPU` 替代 | 删除 | `cf510743` |
| C-M4 | Minor | ingest 的诊断日志字段比 §3.12 少，也没有测试 | `debugEvidence` 补字段，`diagnostics` 用例按子测试起点截取 | `ec542f7d` |
| C-M5 | Minor | M12 移交里一条相对路径错了 | 改路径 | 文档提交 |
| C-N1 | Nit | 注释引用了不存在的 `dependency-path.md` | 删掉这句 | `ec542f7d` |
| C-Q1 | Q | 00054、11 个提交和几项容量优化不在 P4 设计里，有批准吗 | 都是主持者的决定，没有负责人批准，负责人可以改判；时间先后与代价见下文 | — |
| C-M6 | Minor | 核对时发现：`Makefile` 里分片数的注释随 C-M1 过时 | 改数字 | 文档提交 |

修 A-I1 时带出一处新的缺口：`allDirect` 提前返回之后，§3.12 要的"有候选且证据不足"那行诊断走不到了。`ec542f7d` 在提前返回处补上，`diagnostics/unknown_old_page` 守住它。

## 修复核对

三位原审查者都在 `cf510743` 上核对，全部关闭，没有新的 Important。

- **A**：`0a669e0a` 不可能改变 novel 集合。A 走读了 `fact.Direct`、`FollowedUntouched`、`parents` 的来源，又加了影子调用，用旧的完整候选再问一次 `NovelSources`：四组 ingest 测试加 14 个定向形状，36 组对比零差异。100k 节点重存已引用来源从 558–577 ms 降到 12–15 ms，Heads 与 NovelSources 都是 0；新引用仍约 550 ms。唯一的差别是只和被减掉的候选有关的防御性报错不再触发，这只会少回滚，不影响标记。A 更正了自己的报告：§3.9 的后半句已经写了不抹掉 D_W，A-M1 不是设计冲突，并认为决定 151 是对的。
- **B**：工具描述与 `tools.json` 四处逐字节相同。升级说明里每条关于锁、Down 与 LZ4 的陈述都在 PostgreSQL 18.6 上实测过，"没有 LZ4 时迁移失败"除外，它没有在不带 LZ4 的构建上跑过，依据是 PostgreSQL 的构建文档。B 指出 WAL 数字来自 CHECK 扫描写 hint bit 的整页镜像、不随行数成比例，文字已改。
- **C**：重跑了自己原定义的变异（它的 C2 与修复时脚本里的 C2 不同），全部杀死。Important 5 撤回级别，同意不压缩。核对时新发现一条 Minor（C-M6）。

## 主持者的决定（负责人可以改判）

**决定 150，两个极端用例的请求期限。** 见 [00 §11](../00-M10-design.md#11-负责人确认的决定)。

**决定 151，未改 stale 来源的旧 F。** 见 [00 §11](../00-M10-design.md#11-负责人确认的决定)。

**决定 152，不改写分支历史。** 前 11 个提交里有 5 个不能单独过 e2e 或 costs-lint，C 都实跑过：

| 提交 | 结果 |
|---|---|
| `ad1261ef`、`5d4607eb` | e2e 红：HI3（API）、MC8 |
| `aa2a34cf` | e2e 红：HI3、MC8、LW9 的 MCP 两个 scope（`message: required`） |
| `65c662b0` | 同一组通过 |
| `6d1549fc`、`c90ecf22` | costs-lint 红：`version=54, want 53` |
| `b86019d9` | 同两条通过 |

[工作流](../../claude_to_codex/02-workflow.md)和 [P4 §7](../04-P4-virtual-ingest.md) 对每个提交要求的是 build、vet、模块的 lint、短测试，改到网页时再加 tsc、knip、vitest，这 11 个提交全部通过，e2e 与 costs-lint 在这条线之外，所以不算违规。合并用 `--no-ff`，用 `git bisect --first-parent` 不会落到这些提交上；压缩会让决定 150 和 Codex 证据引用的提交号全部失效。

但有两条具体的文字没有做到，这里记下：

- 矩阵和 00 第 11 节写严格期望与 S2-1 同提交，HI3、MC8 的严格比较到 `65c662b0` 才对齐。
- 04-P4 第 6 节写不加第四个代码提交，00 第 11 节写原三个代码提交保持，实际到 `02567298` 是 11 个，加审查的修复 15 个。

**决定 153，恢复与撤销的时限。** 12 秒改成 14 秒，见 [00 §11](../00-M10-design.md#11-负责人确认的决定)。

**Codex 的决定 77–149。** 都是 2026-11-07 接手之前的主持者决定，负责人都可以改判。多数是 S2 为过成本门槛做的局部优化或测试修复，下表只列负责人可能想看的。决定 146 落实负责人 2026-11-06 叫停性能优化，它改的是时限；决定 150 又在它之上放宽。

决定 144 与叫停的先后，git 看不出来：迁移 00054 和叫停的记录是同一个提交 `6d1549fc`（11-06 22:05）。证据目录的文件时间给出次序：决定 144 的计划与实现在 18:15–18:51，决定 146 的计划在 21:12。所以 00054 是叫停之前做完、之后一起提交的。

| 决定 | 内容 | 影响 |
|---|---|---|
| 144 | 迁移 00054：`page_contents.content`、`page_revisions.content` 之后写入的值用 LZ4 压缩。样例上 TOAST 内核约 1071 ms 降到约 160 ms，存储多约 22.9% | **部署须带 LZ4 构建**，存储变大。这是 P4 设计之外加的一条迁移。改判的代价：Down 不改已存的值，它们之后仍要带 LZ4 的数据库才读得出，所以不是干净可撤销的 |
| 143 | `LinksReached` 改成四条互斥的 `UNION ALL` | 审查者 B 新旧 3000 组随机参数零差异 |
| 134 | `costs` 分三份（13/9/12） | CI 作业数变了 |
| 133 | bootstrap 分六份，新增 `TestIngest` 单列 | 同上 |
| 118–122、124–132、135、136、140、141 | prepared source、有界 rewrite 准备、link arena、uuid 扫描等局部优化，以及 M6 通用 `Rebuild` 的修订 | 都不改语义。代码多出一些，见"接受与推后的" |
| 137、139 | 两个实验被拒绝，恢复 | 不在代码里 |
| 146 | 两个极端用例的 unit 守卫放宽到 14 秒 | 已被决定 150 取代 |
| 147、149 | 测试观察器和旧迁移夹具的修复 | 只改测试 |
| 148 | 网页测试共享 cleanup 等一次零延迟释放 | 审查者 C 用变异证明这段等待有测试把守 |

## 变异

C 跑了 79 行变异结果（含重跑和对照），基线未改时全绿。被杀死的覆盖矩阵的主要控制：去 index 锁（U01）、late bytes 冒 Known（B09）、软停认 Complete（C09）、满批当 EOF（C04）、无 Kind（U10，LW9 的 API 与两个 MCP scope 全败）、按 node 排序（H03）、去 SKIP LOCKED（P05）、锁别人的 content（U02，PostgreSQL 观察到锁环）、Capture 后移（B03）等。判为等价的两条：`followed_untouched` 去 `JOIN current_sources`（adapter 保证 untouched 是 current 的子集），`first_json` 去 `frontmatter_valid`（解析失败时 `page_properties` 没有行）。

C 报告存活的，修复之后全部杀死（20 条，含新加的）：

| 变异 | 改动 | 杀死它的用例 |
|---|---|---|
| A1、A2 | 写入腿去 AGENTS、来源根的判断 | `exclusions/new_emitter_agents`、`new_emitter_root` |
| A3、A4 | coverage 腿同上 | `exclusions/untouched_agents`、`untouched_root` |
| A5、A6 | Before 腿同上 | `exclusions/before_agents`、`before_root` |
| B1 | 去已知空目标 | `exclusions/before_known_empty_target` |
| C1、C2、C3 | coverage 不收 parent、不传、不升级 | `stale_parent_q_written`、`diagnostics/suppressed_asset` |
| M18 | `digested` 不要求 current | `stale_parent_q_untouched` |
| S1 | F 含 frontmatter | `exclusions/f_frontmatter_not_followed` |
| S5 | 非字符串槽 | `exclusions/nonstring_element_no_slot` |
| M20、M21 | `Now` 不分批、need 反序 | `page/app` `delete_all_nodes` |
| — | `IsRootAgents` 翻转 | `page/app` `first_memo_agents` |
| L1–L4 | 早退不记、reason 固定、suppressed 不减、heads 为 0 | `diagnostics/*` |

A-I1 的修复另有三条：去掉 `allDirect` 或 `possibleDirect` 的减法各由 `already_referenced` 杀死（现在是错标，不只是多扫一次）；coverage 不收 parent 由 `stale_parent_q_written` 杀死。决定 150 的时限：5 秒的变异在目标断言处 5.00 秒失败，已按原样恢复。

核对时 C 又抽查了新代码：前置过滤去 `allDirect`、去 `possibleDirect` 都被杀死（后者 24 项失败）。五条诊断标签的变异存活：old-evidence 分支的 `heads` 改 0，`old_targets_possible`、`missing_index`、`revision_mismatch`、`work_budget` 四个标签没有用例走到。§3.12 写明诊断是内部辅助，不影响判定，接受。

局限：修复后的这组变异跑的是窄过滤，没有跑全量 `make test`。

## 门禁与 CI

在 `cf510743` 上：`make lint gen-check knip build test-go-short` 全过，`make e2e` 361 passed。分支 CI [42308764984](https://github.com/open-nerve/NerveWiki/actions/runs/42308764984) 17 个作业全绿，含 e2e 与镜像。本地没有跑全量 `make test`，它的各份都在这次 CI 里。

这次 CI 第一次把两个极端用例之一放在最慢的机器上：`guards-core` 落在 EPYC 7763，最大形状三次 17.65–17.90 秒，低于 29 秒的守卫；`guards-sources` 落在 EPYC 9V45，最大字节三次 9.1–9.5 秒。17.7 秒也证实了 M12 那条容量边界：生产的 `request_timeout` 是 15 秒。

文档提交 `55f4f8ab` 的分支 CI [42313012300](https://github.com/open-nerve/NerveWiki/actions/runs/42313012300) 里 e2e 红了一条：LW6 的页面被跳回登录页，`POST /auth/refresh` 答 401，服务端记 `refresh token reused: session revoked`。这个问题早于 S2，代码与上一次绿的 CI 相同。故事 `goto` 版本页之后，否定断言不等页面加载就过，下一个 `goto` 打断了在途的续期：服务端已经换了令牌，浏览器没收到，再用旧令牌就算重复使用。这是 [M1 设计](../../M1-auth/00-M1-design.md)第 10 节接受的后果，产品不改。本机 30 次复现不出来；把续期的答复延后 500 ms 时，旧的故事 4 次全败、现象相同，改后 4 次全过。改法是照 LW4 先等版本的内容出现，这也让那条否定断言真的有意义。

之后的分支 CI [42316471113](https://github.com/open-nerve/NerveWiki/actions/runs/42316471113) 里 `guards-core` 红：撤销 12.05 秒过了 12 秒的线，这次落在 EPYC 7763 上。同样与 S2 无关，处置见决定 153。

Codex 在 `b86019d9` 跑完成本全量：148/148，3868/3868 节点，最慢的指标用到预算的 92.7%。

CI 的 runner 不是同一种机器。`b86019d9` 的分支 CI [42284811629](https://github.com/open-nerve/NerveWiki/actions/runs/42284811629) 里两个极端用例撞到 15 秒的请求期限，同样的代码在快机器上约 10 秒，同等工作慢约一倍。决定 150 因此放宽时限，并在 CI 里加了 `Runner CPU` 一步。

## 分份的数

bootstrap 六份，521 个顶层分为 157/51/116/123/61/13，C 用脚本核过无漏无重。`cf510743` 这一次分支 CI 的 Test 为 417/423/334/409/355/377 秒，都低于 450；规定的两次 main 的均值合并之后核，见 00 号文档第 12 节。

`costs`/`costs-lint`/`guards` 为 34/14/27 条。

## 接受与推后的

- **最大合法 batch 的容量边界**记在 [M12 的移交](../../M12-release/handoffs/M4-performance.md)：约 400 万条旧关系的 batch 在慢的 4 核机器上可能超过 `server.request_timeout` 15 秒，答 `internal_error`、回滚，不留半截数据。
- **过度设计的提醒，不处理。** 审查者列出：`retention_array_encoding.go` 每次 Exec 换 codec，`prepared_source_text.go` 的 ScanBytes 比较，`QualifiedBefore.Capture` 生产不可达，`oldFollowed.possible`、`QualifiedReference.Start` 写入但不参与决策，`linkArena`、`labelSlot`、`writtenRange` 的微优化，`ingest_mcp_preview_test.go` 里多个同体的具名腿。这些是过成本门槛时留下的，负责人已叫停优化，再动它们要重新过门槛。`knownDirect`、`knownFollowed` 两位审查者都点了，已删。
- **未核实的怀疑。** `ingest_mcp_preview_test.go:52` 的 4 秒断言和 `ingest_budget_test.go:92` 的期限窗在 `-race` 加慢 runner 下可能抖动。CI 样本全绿，没有单独量。
- 两页 batch 的 Bodies 范围扫描（约 2M 链接时 1.4 秒）和只删除的 unit 也做完整 Capture，未超预算，负责人已叫停性能优化，不动。
