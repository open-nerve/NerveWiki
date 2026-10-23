# M9 代码质量评审（Codex）

评审日期：2026-10-23。基线：`main`，`06a5554d3a7051af05e0d7ea997c35b5245b36f4`。本文只含评审，不包含后续处置。

## 1. 结论

发现 **3 项：Critical 0、Important 1、Minor 1、Nit 1**。

- **R1 / Important**：batch 中先移动链接目标、再明确把引用改回原文，最后一次编辑可能被推后的链接改写覆盖。真实 MCP 请求中，`move A.md→B.md` 后 `edit P.md [[B]]→[[A]]` 成功返回，但 P 最后仍是 `[[B]] x`；逐项调用得到预期的 `[[A]] x`。Markdown 链接也一样。这是持久化正文错误，应在 M10 依赖 batch 做 ingest/整理之前修复。
- **R2 / Minor**：batch 的版本令牌指向另一页时，拒绝响应把当前版本写成 `<当前页 id>:0`；同样的单独 `write`、`edit` 返回真实的 `:2`。没有越权写入，但返回的恢复信息不是合法版本令牌。
- **R3 / Nit**：附件读的设计说明和一处注释把 `Content` 前的检查也描述成会识别权限撤销；实际该步只检查 blob 存活及身份。应收窄文字，不把它升级为已证实的权限漏洞。

所要求的基线门禁全部通过。M8 的 R1–R3 修复经代码、回归与反向对照核实，本轮没有确认它们留下的新产品缺陷。另保留两个设计疑问：正文操作的语义身份与实际版本写入混用（D1，关联 R1），以及分离读取下资源预算的接受理由不足（D2，仅静态证据，不计已确认缺陷）。

## 2. 评审范围与方法

### 2.1 基线、隔离与材料

开始时核实原仓库在 `main`，HEAD 为上述基线，工作区干净。主范围 `fff7c560..06a5554d` 实得 **345 个提交、546 个文件、+88,251 / −1,669 行**；附带范围 `4db8503a..fff7c560` 为 **20 个提交**。仅核查 M9 及它触及的 M4–M8 接线，并复核 M8 报告第 9 节的 R1–R3 修复；没有扩成旧模块的全面复审。

全部运行在仓库外 `git archive 06a5554d` 的副本：`/tmp/nerve-m9-review-w8mo5jfk/{baseline,probes,platform,revert,search}`。门禁副本按要求独立 `git init` 并作一次中文基线提交；前端副本补同样的本地 VCS 元数据以验证构建信息。原仓库没有提交、推送、切分支或建立 worktree，没有使用 `gh`，唯一新增文件为本报告。临时因果改动、实现变异均在探针副本，并已恢复；本轮没有交付产品修复。

依赖只取本机缓存：`pnpm install --offline --frozen-lockfile`，Go 使用缓存的 1.27.1 工具链，`GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off`。各 Go 进程 `-p 1`，并发包额度合计不超过 3；Playwright 合计不超过 4 worker，e2e 前先 `make build`。数据库仅由测试自身的 testcontainers 启停 PostgreSQL `18.6-trixie`。没有管理其他 Docker 容器、重启 Docker Desktop、访问兄弟仓库或 Obsidian、操作桌面或用户浏览器；浏览器仅为 Playwright 自己启动的 Chromium。

没有把任何真实 MCP 客户端连接到服务，没有执行 manual 清单，没有运行客户端注册命令或修改客户端配置。协议证据来自 Go HTTP 测试、`e2e/fixtures/mcp.ts` 和故事中的 TypeScript SDK 客户端，服务由测试自己启动。

团队分工阅读并对照了 `docs/README.md`；总体设计 §6.3、§12（含 12.4）、§13.1–13.5、§14–15；M9 总设计的完成标准、关键决定、测试、风险及第 11 节 1–112 条与设计取舍；五份 Phase 的最终“按实现”部分；原有 **18 份 review**（含收尾的发现、可推翻决定、接受/推后、修复核对、未验证风险）；九份收到的 handoff；指定的 M10、M11、M12 移交；两个入口的 `tools.json`、page/transfer/asset 契约改动、迁移 00042–00047、README 两节及反向代理说明、只读的人工清单和 M8 报告。没有声称逐行穷尽审完全部 88,251 行文档、源码及生成物。

### 2.2 门禁与计数

Go 按用户允许的 CI 分法运行；未执行会自行 `-j4` 的总 `make test-go`，避免违反本轮包并发约束。race 与三个 bootstrap 目标仅覆盖 `GO_TEST_RACE` 为 `go test -p 1 -json -race -count=1 -timeout 15m`，保留竞态、禁用结果缓存和原时限；costs、guards 使用原目标。以下均为本轮本机墙耗时，不是 CI 数字。

| 门禁 | 结果 | 墙耗时 |
|---|---|---:|
| `make lint-go`（最终） | 通过；两个模块无问题 | 2.233 s |
| `make gen-check` | 通过；生成物无差异 | 6.993 s |
| `make test-go-short` | 通过 | 176.643 s |
| `make test-go-race` | 通过 | 790.719 s |
| `make test-go-bootstrap-1` | 通过 | 263.293 s |
| `make test-go-bootstrap-2` | 通过 | 180.016 s |
| `make test-go-bootstrap-3` | 通过 | 196.236 s |
| `make test-go-costs` | 通过，含 tools 模块 | 234.825 s |
| `make test-go-guards` | 通过 | 317.105 s |
| `make lint-web` | 通过 | 4.70 s |
| `make knip` | 通过 | 4.30 s |
| `make test-web` | 166 文件、3344 测试通过 | 26.45 s |
| `make build`（VCS 副本） | 通过 | 1.39 s |
| `NWIKI_E2E_VERSION=0.1.0-dev pnpm exec playwright test --workers=4` | 282/282，通过全部 28 个 MCP 标题故事 | 66.58 s |

四份竞态门禁的 JSON：非 bootstrap **2232** 个顶层 PASS、1 个顶层 SKIP；bootstrap 三份分别 **168、109、127** 个顶层 PASS，第二份另有 2 个顶层 SKIP。合计 **2636 个顶层 PASS、3 个顶层 SKIP**；含子测试为 **9643 个 PASS、5 个 SKIP 事件**。执行通过的不同包 **88** 个，另 **22** 个无测试文件（bootstrap 三份算同一个包；此数不混入 costs 的 tools 模块和非竞态重复）。含子测试的 PASS 事件与顶层测试分开统计；重复交错、额外种子、定向回归、变异及恢复控制不加到互异基线测试总数中。`-short` 下按设计跳过的数据库及时间守卫，由后续集成与 costs/guards 覆盖。

本轮基线成本与守卫没有超时失败，未提高阈值，也无需因忙时失败做空闲复跑。部分量值：10 万页 `ListIndex` 调用中位数 **1.046 s**、分配 **158.0 MB**；最慢 glob 走查 **762.985 ms / 2 s**；恢复 10000 节点/400000 历史行 **4.775 s**，同规模撤销 **4.033 s**；batch 在 399 页改写处单元 **3.371 s**，120000 链接处 **3.338 s**；grep 最坏一行 **756.823 ms**，一次工作界 **1.930 s**，10 万候选调用中位数 **141.945 ms / 1 s**。这是测试自身的取样口径，不把中位数/最快值说成 p99，不据此推断 CI 或其他硬件余量。

环境性失败单列：最初系统 Go 1.26.2 的自动工具链校验在 `GOSUMDB=off` 下失败，改用已经缓存的 Go 1.27.1 绝对路径后继续，未下载。初次 `gen-check` 在工具链失败前删掉了副本内生成目录，因此紧接着的 lint 报缺包；重新成功生成、确认无差异后，最终 lint 通过，未改产品源码。第一次完整 e2e 为 **281 通过、1 失败**，唯一失败是归档没有 Git 元数据，S3 读到 commit=`unknown`；给副本建立独立 VCS 基线并重建后，完整重跑 **282/282 通过**，包含全部 **28 个标题带 `(MCP)` 的故事**。不把这些准备问题计为产品缺陷。

### 2.3 加测与证据口径

证据目录为 `/tmp/nerve-m9-review-w8mo5jfk/logs/`，含完整命令、输出、退出码、墙耗时、种子补丁、变异与恢复记录；根目录的三个分工报告保存逐份材料及命令映射。临时目录不是仓库永久附件，所以下文同时写出最小复现和关键结果。所有产品行号均指 **06a5554d**。

| 加测 | 实际范围与结果 |
|---|---|
| 连续整组撤销 | `revert_property_test.go` 保留 1–4，新增 **17、53、97、65537、20261023**；9 个随机世界比较树、正文（含回收站）及索引，含 batch；通过 |
| 后台撤销性质 | 原 0–1999 外增加起点 **2700000、7300000、20261023** 各 2000 个种子；共 **8000 trials、15800 jobs、37433 batches**，保留原覆盖下限；通过 |
| 路径往返 | **17、29、43、61、97、131**，每种 4 棵树，各至多 80 次节点生成；通过 |
| grep 键与行 | 同上 6 种子；键共 **120000 trials**，行每种 300 页、7 种模式；含非 NFC 前段、截取、与 read 行号/column 对照；通过 |
| 链接重建 | 新种子 **6–13**，每种 60 步；完成 54 个 batch，其中 30 个 batch 发生了页改写；重建等价及覆盖下限通过 |
| 搜索重建 | 原 1、2、3 加 **5、7、11、17、29、43、61、97、131**；每种 40 次尝试，逐步比较重建结果；通过，不把尝试数当成功变更数 |
| 确定性交错 | batch/mcp/revert 11 项，lock/rewrite 6 项，background 6 项，background_stops 5 项，search 9 项，另名称环分段/批间写冲突/父链上移等待 3 项；共 **40 个顶层场景各 `-race -count=10`，400 次通过** |
| P1 黑盒补充 | Unicode/BOM/CRLF/长行的游标续读逐字节还原；AGENTS batch 交换→撤销→撤销撤销的双入口缓存；不同 PAT 的名称及入口隔离；**3/3 通过** |
| 新发现探针 | 两个顶层测试、四个子例，按正确行为断言，全部暴露 R1/R2 而失败；正常的逐项调用控制通过，拒绝前后的状态相同 |

交错沿仓库已有握手、持锁与批边界安排执行。部分后台交错仍沿用既有等待者次序假设（P3 S3 **C-S1**、决定 **76**）；本轮重复通过不消除这个已知局限，没有把释放 PostgreSQL 行锁后的顺序当作新增的确定性保证，也没有把 400 次通过理解为所有功能任意组合的穷举。测试扩展没有删改原预言或降低原覆盖下限。

## 3. 发现清单

| 编号 | 级别 | 问题 | 证据 | M10 前 |
|---|---|---|---|---|
| R1 | Important | 净字节未变时丢失最后正文操作的身份，推后改写覆盖明确编辑 | 真实 MCP/数据库，两个链接形状；副本因果对照 | 应修 |
| R2 | Minor | batch 的 wrong-node 版本拒绝返回非法 `current_version=:0` | 真实 MCP/数据库，write/edit 与单独调用对照 | 建议同修，不单独阻断 |
| R3 | Nit | 附件 Content 前撤权检查的文字超出实际保证 | 静态调用链与文档逐项对照 | 不阻断 |

## 4. 发现详情

### R1 — Important：batch 的最后编辑回到原始字节时，被更早移动的链接改写覆盖

**位置**：`server/internal/modules/page/app/apply_batch.go:641`、`:745`；关联 `server/internal/modules/page/app/unit_content.go:65`、`server/internal/modules/linking/app/rewrite.go:287`。计划的正文语义在 `server/internal/modules/page/domain/batch_contents.go:91` 已正确算出，缺陷在执行结果传给参与者的阶段。

**复现**：同一本笔记本已有 A（正文空）和 P（正文 `[[A]] x`），P 的链接索引正常，没有编辑会话。经有效写者 PAT 调用 `batch`：

```json
{
  "message": "move target, then keep the original reference",
  "ops": [
    {"op": "move", "from": "A.md", "to": "B.md"},
    {"op": "edit", "path": "P.md", "old_string": "[[B]]", "new_string": "[[A]]"}
  ]
}
```

在另一本相同初态的笔记本按次序分别调用 `move` 和 `edit`，作为独立控制。第二组把初态换成 `[A](A.md) x`，编辑参数换成 `[B](B.md)` → `[A](A.md)`。

**实际结果**：batch 成功，两个结果均 `changed: true`；P 最后分别为 **`[[B]] x`**、**`[B](B.md) x`**，索引指向 B。逐项调用的 P 分别为 **`[[A]] x`**、**`[A](A.md) x`**，链接悬空。

**应有结果**：保留最后一次显式编辑，正文与控制组相同。总设计 4.11 明确“先移动、后改正文……照 agent 写的”（`00-M9-design.md:273`），而 `changed` 看该操作当时是否改动（`:283`）。链接暂时悬空是本次明确编辑的结果，不能由先前 move 再覆盖。

**根因与影响**：计划先按 move 把 P 模拟成 B，再执行 edit 改回 A，所以该 edit 确实改变了它当时看到的内容。执行时将最后正文一次写入，恰好等于数据库事务起点的字节，`WriteContent` 返回原 revision；`a.wrote` 因此没有 P。`participate` 只把 `a.wrote` 交给 `BatchRewrite.Written`，linking 将 P 当作没有显式正文操作的页，使用批前索引再次跟随 A→B。调用成功却写错正文，可能改变引用目标及显示文本。已实测这两个形状；同一用例由两个 MCP 入口共用，但本轮反例通过笔记本入口运行，没有宣称穷举所有操作排列。

**证据**：外部副本 `review_batch_link_intent_test.go`，日志 `root-batch-probes.log`；两个子例在基线上失败，并比较真正的两个笔记本，不用实现的计划结果当预言。独立分工另按源码和契约复核。因果试验仅在副本保留“计划中确实改变过正文”的参与标记，两个反例及原 `TestTheLinksOfABatchAreThoseOfSeparateCalls` 均通过（`root-causal-link-intent.log`），随后逐字节恢复。这个试验不是已完成、可直接合并的修复。

**与已知项的区别**：P3 S1 **A-I1 = B-M2**（总设计决定 **48**）修的是计划阶段 `write` 如何与前面移动后的字节比较；本例计划已经正确，执行阶段丢失最后 edit。该记录 **W3** 说的是本来“不改动的 write 不算写”，本例 edit 的 `Changed=true`，不是该例外。P3 S1 **A-M1**、收尾 **C-M13** / 决定 **112** 处理锁拒绝次序和同字节 write 的多拒，本例没有锁而成功后写错。4.11 的旧索引、新节点、别名、中间路径规则也没有许可重放已被最后编辑覆盖的移动。

**建议**：分开记录实际产生版本的写与必须按最后正文操作解释链接的页；后者即使净字节等于初态，也要携带正确的 `WrittenPage` 和最后操作之后的 Changes（本例为空）。不要直接把所有无变化 write 都算作实际写入，从而改变 W3、页权重、日志计数或锁规则。补充上述两个整个程序差分用例，并覆盖多次正文操作抵消、后面仍有 move、`update_links=false` 的组合。

### R2 — Minor：batch 的另一页版本令牌拒绝返回当前页的第 0 版

**位置**：`server/internal/modules/page/domain/batch_plan.go:407`；输出由 `server/internal/modules/page/adapter/mcp/text.go:31` 原样转换；合法令牌检查在 `server/internal/modules/page/domain/version_token.go:37`。

**复现**：同一本笔记本建立 A 和 B，把 A 正文写到第 2 版。分别给下面两种单项 batch 的 `base_version` 传 **B 的第 1 版令牌**：`write A.md` 覆盖正文；`edit A.md` 将现有正文替换为另一段。再把相同参数通过单独 `write`、`edit` 调用作控制。

**实际结果**：两种 batch 都正确拒绝为 `page.revision_mismatch`，`op=0`，但 `current_version` 及文字里的“当前版本”均是 **`<A id>:0`**；单独调用均返回 **`<A id>:2`**。拒绝前后笔记本状态不变。工具错误为 HTTP 200 中的 `isError` 结果，不把领域的 409 错写成 HTTP 状态。

**应有结果**：返回当前页真实、可解析的版本令牌；总设计第 11 节决定 **6** 明确要求节点不同也带当前令牌。`ParseVersionToken` 只接受 1 起的 revision，因此 `:0` 连本服务自己的输入校验都不满足。

**根因与影响**：树阶段只有节点信息，构造 `VersionMismatch` 时省略 Revision，零值穿过最终错误适配器。调用者得到误导的恢复信息，照字段重试会先遇到参数格式错误；这不是覆盖另一页、跨笔记本写入或权限绕过。提示要求先 read 可以绕开问题，所以定 Minor。

**证据**：`review_current_version_test.go` 的 write/edit 两子例，真实 HTTP、PAT 与 PostgreSQL；日志 `root-batch-probes.log` 同时记录单独 `:2` 与 batch `:0`，并核对拒绝没有写入。不是仅根据零值推测。

**与已知项的区别**：P2 设计 **B-M3 = C-M2** 及已修同路径换页测试防止错误节点被写；该保护仍有效。P3 S1 **C-N2** 修的是成功 batch 在推后改写后的最终版本，本项是树阶段失败答复的 `current_version`，不是同一修复。第 11 节决定 6 没有接受第 0 版。

**建议**：保留树阶段决定错误位置/优先级，但在最终选定该错误之后由用例读取当前页的 revision，再产生对外 `VersionMismatch`；或在计划结果中明确表示“待补当前版本”的错误，避免零值被当作完整令牌。补测错误字段能被 `ParseVersionToken` 解析、等于当前 read 的 version，并覆盖同路径换页和别的笔记本令牌，继续核对拒绝不写。

### R3 — Nit：附件 Content 前的检查被写成了再次授权

**位置**：`docs/v0.1/M9-mcp/01-P1-platform-reads.md:1494`、`server/internal/modules/page/adapter/mcp/read.go:122`。实际链为 `asset/attachment_reads.go:60`、`:80` → `asset/app/blobs.go:128`。

**静态复现步骤**：沿内联图片的 `read` 跟踪：`ReadPath` 找节点；`AttachmentReads.Read` 经 `reads.Get` 判定访问；返回元数据之后 `Content` 调 `Blobs.Open`，只检查未删除的 blob 行、blob id 与文件。设权限恰在前一步授权成功后撤销、行和文件仍在，后半段没有再次调用授权器。

**实际与应有文字**：实际是元数据阶段授权、字节阶段核对 blob 存活和身份；文档“Read 与 Content 各自之前……失去访问”的表述及注释把第二步也说成能处理撤权。应把两段保证分别写准确。影响维护者理解，不是本轮证明的新未授权请求成功。

**证据、去重及建议**：静态调用链，没有运行撤权竞态。P1 S2 **B-M3/C-M3** 处理 Find→Read 的权限错误映射；**C-N3** 及修复 `988b3423` 处理 Read→Content 间删除附件的带路径错误，两类修复仍成立。本项只指出最终文字把前一种保证推广到后一段。收窄说明和注释即可；不要为修正文案悄悄追加在途读必须即时撤权的新语义。已有签名 URL 失权后仍可用至多 2 小时的接受项也不重报。

## 5. 设计层面的疑问

### D1：计划的正文操作与实际产生版本的写，是否应有不同的状态？

关联 R1，不另计发现。`a.wrote` 同时承担日志数量、改写权重及“这页应从哪个操作时点解析链接”的责任，净字节优化把第三种语义抹掉了。建议明确两个集合及各自不变量，让后续 M10 守卫、批量整理和新增参与者依赖有名字的语义，而不靠 revision 增长反推是否发生过操作。已有单元/参与者抽象不需要整体推倒，问题集中在这一处信息表达。

### D2：正文读取预算是近似预检，还是实际读量的硬界？

仅静态疑问，不计服务中断缺陷。`page/app/grep.go:452` 读 PageStates，`:465` 按旧大小切批，`:483` 先取得完整 contents map 后才累计实际长度。P4 设计审查 **A-n3** 已接受状态可能变旧，但其第 154 行“多出的由大的请求体的名额限着”的理由不能独自推出实际读取上界：名额限制同时在途的大请求体，不限制整个状态→正文窗口里已完成的多次写；`edit replace_all` 的结果大小也不等于参数大小。每页上限仍有效，这不等于一批实际只会多出两页的量。

`ApplyBatch.read` 的状态大小预检与随后逐页正文读取也存在分离窗口（`page/app/apply_batch.go:301` 起），后续单元界线不能证明此前读取分配未超过预检数字。这里不是重报 A-n3 的已知窗口，也不是声称粗上界一定可达；没有执行资源放大、耗尽或 OOM 实验，未证明崩溃、挂住或越权。

建议明确 4 MiB/14 MiB 等读预算在并发写下的保证，若要实际硬界，应在实际读取/版本一致性策略中落实并以受控测试验证；若接受近似界，应以正确的粗界和可达性论证记录，而不是用大请求体名额替代证明。不同于收尾 **A-F2** 的命中文本钉住整页、决定 **106** 的首行进度豁免及 M12 已量过的耗时余量，这里讨论读取前后数据版本不一致的论据。

## 6. 测试缺口与变异结果

### 6.1 新缺口

R1 的现有差分表缺“移动先改写，再显式编辑回到事务初始字节”的交叉情形。撤销后等于初态、索引等于重建都可能通过：它们验证可撤销和索引自洽，不能证明正向正文符合最后编辑。应将本轮反例加入整个程序的正向差分预言。

R2 的树计划测试核对 OtherNode 和节点 id，却没有穿过真实适配器核对 current_version 是当前、合法的令牌；成功答复版本正确也覆盖不了这个失败分支。R3 缺的是契约措辞精确性，不能仅因没有二次授权测试就要求增加二次授权。

### 6.2 本轮实际变异

共 **18 个生产实现形状**，逐个在外部副本改错并运行测试，均被捕获；失败来自行为、返回结果、计划/范围或耗时断言，不以编译错误充数。修改后恢复并重新执行相关控制。R1 的因果修正试验和两个新发现探针另列，不混作这 18 个既有测试变异。

| 变异 | 捕获结果 |
|---|---|
| 生产工具清单删除 read | `TestTheToolListsAreTheContract` 报两个入口均缺 read |
| 清单增加 review_extra | 同测试报两个入口均多出未列于契约的工具 |
| read 说明与契约不符 | 同测试对两个入口逐成员比较失败 |
| 每个 grep 候选固定多做 10 µs 工作 | 既有 10 万候选调用守卫测到中位数 1.149 s，超过 1 s；变异前后均通过 |
| 组合根不交 BatchLinks | `TestTheLinksOfABatchAreThoseOfSeparateCalls` 因工具错误/恢复的 panic 日志及结果断言失败，证明整个程序确实经过最后一跳 |
| 撤销删除不等待待写回 | domain 调度性质测试在 seed 1 失败，计划无可执行步 |
| 后台前提忽略 parent_gone | job 性质测试在 seed 11 失败，本应停止的一批被放过 |
| transfer 桥不传 Joined | 真实后台名称环测试看到批边界临时名并以 changed 冲突失败 |
| grep 忽略 glob | 路径范围外命中使 `the_glob_before_the_bounds` 失败 |
| grep 去掉逐行 MaxWork 判断 | 27 字节三行的小 fixture 已失败；NFC 小行也超过其工作预算 |
| Search 丢弃 PostgreSQL NFC 结果 | app 与真实 PG 的 queued/built 原文查询、摘要断言失败 |
| 00041 的 hits 第二支正文侧不限定 notebook | 隐藏内容 A→B→A 时 GIN 返回行数 3000/0/3000，范围统计断言失败，即使块数可相同 |
| aliases 事件只刷新当前 pages | 历史视图应重读的事件断言失败 |
| 所有 links 事件刷新全部 versions | 普通正文保存不应刷新无关版本视图的断言失败 |
| MCP client 筛选强制小写 | `mcp:Claude` 大小写原样筛选断言失败 |
| 只有 move 也提示改名回去 | 撤销对话框无 rename 的用例失败 |
| revert_too_large 给一般可重试提示 | 不可分段过大的后台任务建议断言失败 |
| revert_too_large 映射成 interrupted | 后台报告终止原因的文字断言失败 |

根侧控制恢复后通过；撤销三项各有变异前后通过记录及文件哈希；搜索恢复后 8 个顶层测试/17 个 PASS 事件通过；前端恢复后 4 文件/156 测试通过。日志分别为 `root-mutations.jsonl`、`root-guard-mutation.jsonl`、`revert-mutations.jsonl`、`search-mutant-*.json`、`platform-mutations.json` 及其完整输出。

工具清单的合成反向检查、真实应用接缝检查和工具权限矩阵覆盖检查也执行通过。没有把只读 JSON 文件与自己比较当契约验证；新工具仍需要注册、权限矩阵和最后一跳行为三种证据。

时间守卫与工作量守卫分开解读：本轮以小 fixture 实际杀掉了工作量判断退化，并运行全部成本/耗时门禁；额外单独运行 `TestGrepOfAHundredThousandCandidatesTakesItsTime`，在候选 map 构建循环每项注入固定 **10 µs** 的额外工作，保持结果及工作量不变：变异前调用中位数 **140.499 ms**，变异后 **1.149310625 s > 1 s**，恰由 `grep_cost_test.go:276` 的耗时断言失败；恢复后 **145.750 ms**、通过（命令墙耗时依次 **8.714 / 14.691 / 10.136 s**；三个 `root-*time*` 日志）。该对照在本轮其余 Go 运行结束后单独执行，源码恢复并核对字节；它证明改为 1 s 的守卫仍能捕获这个明确的候选成本退化，不证明能捕获所有可能的算法变化。。没有重新计数收尾 **M7a**（决定 **107**）只由 domain 捕获的已接受强度差异，也没有将提高到 1 s 的 `9d991781` 本身当新缺陷。

### 6.3 故事及入口对等

完整 e2e 覆盖 282 个用例、28 个 MCP 标题故事，包含 MC1 的 TypeScript SDK 和 MC5 的网页撤销。核查 MCP fixture 的值来自 MCP 答复、共享数据库助手读实际写入；P5 明确的例外仍按决定 **96**：PG7/PG8 以版本替代会话断言，C7 MCP 验证事件帧，PG14 复用活动断言。P5 **C-I1** / 决定 **103** 的相同 pg_notify 载荷去重边界由 publisher 单元测试守，不把 e2e 通过夸大成能区分所有发布次数错误。

## 7. 核实过、没有发现新问题的部分

本节只记录核对范围，不作穷尽正确性的保证。

- **M8 R1 修复**：00041 的笔记本前缀贯穿候选、交集、计数与队列；最长前缀、升级入队、索引/通用计划、当前范围统计测试通过。M8 复核 **C-1** 新补的 tuples-returned 检查和 M9 P4 **B-1=C-I1** 的不同队列数据仍有效；本轮只放宽第二支正文侧的变异确实失败。补跑了长导入、轮转起点、绕回的修复回归。没有重做原 M8 6 万页 HTTP 耗时实验，证据是当前 SQL 范围、计划及读量，不声称有新的时延测量。
- **M8 R2 修复**：正文匹配按 PostgreSQL NFC，名称按 Go NFC；30/31 组合记号、显式 CGJ、已记录 Unicode 空隙、queued/built 两阶段及摘要的当前用例通过；移除 PG NFC 结果使用即失败。第 9 节接受的规范等价、Unicode 版本及词数口径没有被重新计数。
- **M8 R3 修复**：alias 事件触发已挂载历史视图重读，普通正文事件保持原范围；两个相反变异都失败。没有把原文不可变等同于解析 HTML 无需失效，也未发现本轮新增 Promise 或事件刷新缺陷。
- **平台与读取**：PAT/会话令牌区分、入口 notebook 解析与可见性、SDK 前 JSON-RPC batch 拒绝、well-known/register、no-store、panic 恢复、协议版本降级、请求体/JSON 值限制、逐路由 Wrap、日志字段；路径/glob/分页、48 KiB 文字答复与内联图片例外、read 字节及行列、历史 notebook 范围、账户列表与权限矩阵；客户端清理/持久化及最后一跳、AGENTS 缓存。R3 所述文字差异单列。
- **写入与 batch**：补父页、版本令牌节点比较、线性 edit/CRLF、一步 Relocate、recursive 含附件、锁及链接错误、两遍计划/重做/规范化/批量搜索入队；既有大项边界、空写与权限拒绝。新发现 R1/R2 单列，未把 C-M13 的已接受锁语义重报。
- **撤销、后台及并发**：schedule 前提与后备、临时名/附件、恢复和写回成段、跨批冲突、Joined、撤销的撤销、00045 的 moves/renames 及 NULL 处理；确定性交错和扩展性质种子。迁移 ACCESS EXCLUSIVE、解析预算等待及长事务代价按已有决定 **66、76、109** 与 M12 去重。
- **检索与索引**：正则键/partial 超集、glob 先取交、行号与 read 对齐、游标及 incomplete、工作/解析名额、Clone 的命中文本、单本 search、backlinks 路径、index 筛选/截断；00046 入队超集和 00047 前缀生成列、查询计划。未将 A-F2 已修驻留或决定 **106** 的首行例外重报；D2 的理由边界另列。
- **前端、模块与后续接线**：历史 MCP 筛选与名称、撤销建议/revert_too_large、事件重读和 DOM 可访问性；模块导入/archtest、platform SDK 隔离、组合根、M10 工具/prompt 注册及虚拟 index 复用、M11 升级守卫与客户端限制的移交。M0/P1 仍 open，其余收到的 handoff 已按最终实现核对；真实客户端验收未被测试替代。
- **README**：路由、请求体上限、缓冲、超时、nginx/Caddy 样例按代码静态核对。决定 **97** 的 Caddy 早答 502、**108** 的停机 20 秒和 MCP 45 秒、M12 窄屏/空参数/性能项均按已知事项处理。

## 8. 未能验证的部分

- 未运行 Claude Code、Codex 或其他真实客户端，未执行人工清单；SDK/脚本互通不能代替该验收。README 标“照文档”“未核对”的客户端行为没有作为发现，M9 的最终人工完成标准仍由负责人保留。
- 未访问 GitHub/CI、未重跑真实 nginx/Caddy 代理、未验证其他浏览器/真实读屏/IME/移动设备。前端可访问性结论限测试 DOM 和 Chromium。
- 未重跑几十万页/千万历史行的迁移吞吐、WAL、冷缓存和持续满负载，未做真实磁盘满、断电、多实例部署或所有并发功能的任意组合；本机守卫数字不能替代 CI 余量或 M12 的真实规模测量。
- D2 只经源码和既有理由核对，没有动态资源放大证据，不能据此声称 OOM、拒绝服务或确定的超期限 CPU/连接占用。R3 只证明措辞与调用链不同，没有演示新的未授权读取。
- R1 已复现 wikilink/Markdown 两种最后编辑被覆盖，但没有穷举所有别名、显示文字、后续移动排列；副本因果改动没有完整修复的权限/资源计数/并发回归，不能直接当补丁合并。R2 的未写入和错误字段已实测；没有把非法恢复令牌推导成实际跨本写入。
- 要求的门禁、七项性质测试的加种子及上述交错重复已完成；性能变异只抽查指定的一条候选调用守卫，不能证明所有时间阈值都能识别任意算法退化。

总体结论：有必须在 M10 之前修的问题——R1 的 batch 最后编辑被链接改写覆盖；R2 建议同修，R3 不单独阻断 M10。

## 9. 处置（Claude，2026-10-24）

逐项对照代码核实之后全部采纳：三项发现与两个疑问都成立。R1 与 D1 一起修，R2、R3 各自修，D2 照主持者的决定（下面"我的决定"第 115 条）把读的预算改为读到的字节的硬界；6.1 的缺口随各自的发现补上。修复在分支 `m9-codex`（从 `06a5554d` 起，16 个提交）：`4a8c1848` 修 R1 与 D1，`6e8a5a98` 修 R2，`d3a722f2` 修 R3，`2b0b162a` 修 D2；修复核对第一轮的发现由 `dce90fcf`、`0b9c3228`、`817aa52f`、`cbf0ff82`、`eade85cd`（D2）、`f770bb99`（R3）、`e6db420c`、`7c68072a`（R1）修，第二轮的由 `5db7b789`、`2dbbaab4`（R1）、`6a9386ab`（D2 的语句的注释，R2 靠它）、`b2fc6dd3`（D2）修。R1、R2 与 D2 每项都有反向对照；R3 只改文字。

### 处置

| # | 处置 | 反向对照 |
|---|---|---|
| R1 | 照 Codex 的建议把两种意思分开，各在定义处写明不变量（D1，`4a8c1848`）。`batchAttempt` 的 `befores` 改名 `decided`，是正文操作决定的页：有正文操作、批后仍活着的每一页，链接由它最后的正文操作决定，与字节等不等于批前的无关（[总设计](../00-M9-design.md) 4.11）。`wrote` 照旧是正文操作写出版本的页（P3 S1 的 W3：不改动的 `write` 不算写），日志的 `pages_written` 数它，也只有它按 `WrittenRewriteWeight` 计权；推后的改写之后仍可能给其中没写的页写出一个版本（改回之后再移动，日志是 `pages_written 0`、`rewritten 1`）。`BatchRewrite.Written` 改交 `decided`，正文操作没写出版本的一页带 `WrittenPage.Unwritten`，由组合根传给 linking；`pagesCounted` 让它计 1 页，同它在逐项调用里是索引的一页（第 113 条）；链接与字节的界线照旧计它。批后删掉的页不在 `decided`，在 `Gone` 里：这个排除是防御性的，linking 本来就在 `ownMet` 里跳过删掉的页。不变的有：W3、日志与答复的计数、锁的预检（收尾审查 C-M13）、搜索队列、答复的版本；W3 的一处后果：只写同样字节的那一页也在 `decided`，链接从那一步跟随，同逐项调用（之前照批前的索引，是 4.11 的规则那一类不同）。测试：bootstrap 的 `TestTheLinksOfAPageABatchPutsBackAreThoseOfSeparateCalls`（第三份）11 例，两本同样初态的笔记本上 batch 对逐项的 MCP 调用，比每项的 `changed`、页与链接的指向、回收站，两边的索引各等于重建，没有记错误：Codex 的维基链接与 Markdown 链接；以批前的令牌写回原字节（不带 `base_version` 的两边都是 `page.title_taken`，已有 1b）；创建与移动之间两次相抵的编辑；其间 W3 的写；改回之后再移动到它的另一条链接；那次移动 `update_links: false`；先有一次不更新链接的移动；改回之后删掉；带显示文字的链接；旁边一条别名的。`TestTheRewriteOfABatchCountsAPagePutBackAsOne`（界线 3：写过的 P 答 `page.batch_too_large`，改回的做完）；应用的参与者与 linking 的权重的测试。修复核对第一轮补的（`7c68072a`）：`TestTheRevertOfABatchThatPutsAPageBackIsTheNotebookBefore`，撤销改回了一页的 batch 回到批前、撤销这次撤销回到批后，每次索引都等于重建；`TestTheIndexIsItsRebuild` 的定向改回：先改名一页的维基链接按标题指向的节点，再以批前的令牌写回它批前的字节，批后，那些按这一节点的标题键指向它、另一个键（以 `.md` 写的名字的键）又不是新名字的键的链接都不指向它，没有之后的改名时正文就是批前的字节，六个种子做 32 次真的改回；第二轮把核对收窄到改名后改指别处的链接（A2-M1，`5db7b789`），由 `TestThePutBackCheckOfTheIndexPropertyReadsTheLinksTheRenameLeaves` 钉住。注释在第一、二轮改为"正文操作决定的页"与"正文操作没写出版本，推后的改写可能写"（A-N1、A-N2、A2-N1，`e6db420c`、`2dbbaab4`） | 恢复原来的规则（只交 `wrote`）：11 例里 10 例失败（删掉的第 9 例照样通过），第一轮之后属性测试（种子 0、1、2）与撤销的测试的 Codex 的形状也失败；连批后删掉的也交：第 9 例 `internal_error`（交的是零值的 `WrittenPage`，它的 nil 的事实在组合根的 `PageFacts` 处出错）与用例的测试失败；保留删掉的页本身数据的忠实写法（去掉 `linksBefore` 里跳过删掉的页那一步）整个程序的测试照样通过，只有用例的成员测试 `TestABatchsParticipantsFollowItOnceItsOpsDone` 抓得住（核对 A 的 A-N3）；权重不看 `Unwritten`、组合根丢掉它、从不设它，各自失败；第一轮的核对规则在别名与 `.md` 两例上失败 |
| R2 | 保留计划选定的错误与它的项，在错误离开用例之前补上当前的修订（`6e8a5a98`，第 114 条）：`planned()` 经 `withCurrent`，只补修订为 0 的 `VersionMismatch`（树的一遍的另一页），用 `content` 执行时同一个 `currentVersion` 读，在事务之外、不加锁，同计划的别的读；这时页没了算一处差别，重新计划一次，由树的一遍按新的状态答；计划的值不改。领域的构造处注释写明修订由用例补。其余为答复构造 `VersionMismatch` 或 `VersionToken` 的地方逐个看过，修订都已至少是 1：`changedTwice`；单独的 `write`、`edit` 的不符与令牌；`batch_contents.go` 的两处；batch 答复的版本；`read`；`list_versions`、`get_version` 的当前版本；MCP 的 `list` 的条目；`read.go` 与 `versions.go` 各一处的令牌只用来量长度。修复核对第一轮的 B-N1 之后，计划的读里长大的页也带着它此刻的版本（D2 一行）。测试：应用的 `TestABatchsMismatchOfAnotherPageTellsItsVersion`（答 `<A>:3`、什么都不写、不取锁；补之前页没了的重新计划为 `page.not_found`）；bootstrap 的 `TestTheVersionABatchsMismatchAnswersIsThePagesNow`（第三份，经 MCP，用另一页的、同路径换页的（经改名）、另一本笔记本的令牌，各 `write` 与 `edit`）：`current_version` 经 `ParseVersionToken` 解析、等于 `read` 的版本、在消息里、等于单独调用的答复，两本笔记本都不变 | 去掉补的一步，两层都失败；页没了不重新计划，应用层失败 |
| R3 | 只收窄文字，不加第二次授权（`d3a722f2`；修复核对第一轮 B-N4 重新折行，`f770bb99`）：`page/adapter/mcp/read.go` 的 `readAttachment` 的注释写明只有元数据的读（`AttachmentReads.Read` 经 `reads.Get`）判定访问，字节的读只核对行还活着、仍是同一个文件、文件在，两读之间失去的访问它发现不了；`read_test.go` 的对应注释同样收窄；`asset/attachment_reads.go` 的 `Content` 注明"It judges no access: Read did."；组合根的 `pageAttachments` 没有多说，不改。[P1](../01-P1-platform-reads.md) 3.11.2 的"码的次序"（Codex 引的 `:1494`）照改 | —（只改文字；核对 B 逐处对过调用链） |
| D1 | 见 R1：两个集合各有名字与不变量，`decided` 是推后的改写跟随的，`wrote` 是日志、答复的计数与改写的权重看的，锁的预检照旧看计划的页；新的参与者照名字取，不靠版本号的增长反推。[总设计](../00-M9-design.md) 4.11 与第 11 节第 113 条写明 | — |
| D2 | 读的预算是读到的字节的硬界，至多越过设计本来允许单独更大的那一页（第 115 条；`2b0b162a`，修复核对第一轮 `dce90fcf`、`0b9c3228`、`817aa52f`、`cbf0ff82`、`eade85cd`，第二轮 `6a9386ab`、`b2fc6dd3`）。P4 设计审查 A-n3 的理由不成立：大的请求体的名额限的是在途的请求体，不是两条语句之间已完成的写；`edit` 的 `replace_all` 让一页长得远大于请求体。凡是一条语句读大小、另一条语句读正文的地方，读正文的语句自己按请求的次序累加此刻的大小：和不过界线时给正文；第一个过界线的只给大小，其后都不给；过界的正文若在 TOAST 里，只带着指针，不取它的块。三条多页的语句先在 `MATERIALIZED` 的 CTE 里只按键读这些行，再把请求的次序连到这些行上：直接把次序连到表上时，在几万页以内的表上，grep 的 500 个、搜索的 100 个，规划器没有统计时会选整表扫描，有统计时也会（修复核对第一轮 B-M1）；回收站的一条没有统计时会读这一时刻删掉的每一行，2 万行的大组里读一批也是（第二轮 B-N5），而它的 CTE 正是 `TrashContents` 的查询，计划也就是它的。改了四处。一是 grep 的 `GrepContents`：多页一批 `GrepBatchBytes`，单页不限，读过一页之后至多 `MaxRead` 剩下的；过界的页按此刻的大小重新分批，放不下时停在它，游标照旧。二是 batch 计划的读 `PageContentWithin`：比头说的大的页一点也不读，只读它的大小、修订与哈希，算作它版本的差别，带着它此刻的版本，重新计划；再次长大时答 `page.revision_mismatch`，带这一页此刻的版本，why 照单元第二次的差别答（不带 `base_version` 时是"一直在变"；B-N1）。三是后台撤销的一批 `TrashContentsWithin`：界线是单元的字节减去写回的底本，单页不限；之前根被恢复、编辑又删掉时可达 100 × 5 MiB，读完才拒，现在过界即 `errPastPages`。四是搜索任务的一批 `SearchContents`（M8 的代码，M9 改过）：`SearchBatchBytes`，之前同样可达 100 × 5 MiB；现在过界的页与其后的留给下一批，这一批既不删它们的行也不计它们（B-N2）；一页最大的正文放得进一批由测试钉住（B-N3）。不改的有：推后的链接改写（在笔记本的锁下，正文的写都先取笔记本的行，是精确的）、单个移动、`index`（不读正文）、`backlinks`（按读到的计，至多多一页）、请求的撤销与恢复与后台恢复（按不可变的键读）、搜索的摘要（一条语句）、单页的读写、导出（大小与正文在同一个 REPEATABLE READ 的快照里读，是精确的）。测试有三类。一是应用层各处经端口的假实现，在两读之间让页长大，断言实际读的字节与答复（`TestGrepReadsHoldTheirBoundsOnWhatTheyRead`、`TestABatchsReadsHoldTheirBoundOnWhatTheyRead`、`TestTheSearchJobsBatchReadsWithinItsBytes`、后台撤销长大的一例）；修复核对第一轮补了 `MaxRead` 留下的界线、单页任意大、单独一页的恢复加写回、底本占去的字节四个（B-M2），截断的计数与一批放得下最大的一页各一个（B-N2、B-N3）。二是适配器在真的库上的 `TestTheContentsAreReadWithinTheirBound`（四条语句：界线、次序、长大的页、回收站的时刻）与 `TestTheContentsPastTheBoundAreNotFetched`（数 TOAST 的块：1 MiB 的页过界时一块都不取，界线内取它自己的）。三是计划测试，在真实的批量下（第 116 条）：grep 500 个（`TestGrepPlans`），搜索 100 个与回收站 `MaxUnitPages` 个（`TestTheContentsWithinABoundAreReadByTheirKey`），4 万页里 2 万同一时刻删掉的大组（`TestTheTrashsContentsWithinABoundReadABatchOfALargeGroupByItsKey`），有无统计、定制计划都按键读，都不整表扫描。耗时守卫对 `06a5554d` 不变：grep 的 500 个候选的调用中位数 130.6–143.4 → 130.7–135.7 ms，搜索任务建 5,000 页 9.25–9.41 → 9.13–9.34 秒 | 最初 10 种（四处各去掉用例的界线、grep 与搜索任务把过界的页当作删掉、各语句去掉 SQL 的守卫）与最终的 SQL 形式 11 种（每条语句过界也给正文、都说在界线内、去掉其后的行；"过界也给正文"只有 TOAST 的测试抓得住）都失败；修复核对第一轮 9 种（B-M2 更窄的四种：去掉 grep 的 `MaxRead` 项、单页也按 `GrepBatchBytes`、去掉 `trashBound` 的单页豁免、不减底本；长大的页当作删掉、适配器不给修订；删不截、计数不截；一批 4 MiB）与第二轮回收站的新语句 3 种都失败；两条语句的生成 SQL 退回第一版的连接，计划测试失败（核对 B） |
| 6.1 | 三个缺口随各自的发现补上：R1 的交叉情形进整个程序的正向差分（Codex 的两例与九种组合），另有撤销、权重与索引的性质里的改回；R2 穿过真实的适配器核对 `current_version` 可解析、等于 `read` 的、等于单独调用的（同路径换页与别的笔记本的令牌在内）；R3 照建议不加第二次授权，也不加它的测试 | — |

### 我的决定（负责人可以改判）

113. **R1/D1：正文操作让一页回到它批前的字节，这一页也是它们决定的页**：它的链接照它最后一个正文操作跟随，同分开调用（[总设计](../00-M9-design.md) 4.11）。batch 分开两个集合，各在定义处写明不变量：正文操作决定的页（`decided`：有正文操作、批后仍活着的每一页，不论最后的字节），推后的改写跟随它们；正文操作写出版本的页（`wrote`）。W3（不改动的 `write` 不算写）、日志与答复的计数、锁的预检（收尾审查 C-M13，总设计第 11 节第 112 条）、搜索的入队与答复的版本都不变。可以改判的一处：改写的界线里，放回的一页算 1 页，同它在分开调用里是索引的一页，写了的页照旧算 2；算 2 会拒绝分开调用收下的 batch。
114. **R2：batch 对另一页的版本不符答这一页现在的版本**：树的一遍只有节点，计划选定这个错误之后，用例读这一页现在的版本号；这时页已没了是一处差别，batch 重新计划一次。
115. **D2：读的预算是读到的字节的硬界**。规则：读正文的那条语句累加这些行此刻的大小，至多那一个越过界线的页可以过界，照设计"一页单独更大"的规则。P4 的理由（设计审查 A-n3）作废：大的请求体的名额限不住两条语句之间已完成的写，`edit` 带 `replace_all` 让一页长得远大于它的请求体。四条语句：`GrepContents`；`PageContentWithin`（batch 的读：长大的页是版本的差别，重新计划）；`TrashContentsWithin`（后台撤销）；`SearchContents`（M8 的搜索任务，M9 改过）。写法：每条在一个 `MATERIALIZED` 的 CTE 里按键读，过界的不取 TOAST。可以改判：另一种做法是近似的界加正确的论证；照实算，那个界是一批的页数乘 5 MiB。
116. **计划测试在语句真实的批量下跑**（修复核对 B-M1、B-N5）：grep 在 `MaxSearchWalk`（500）个，搜索在 `SearchBatchNodes`（100）个，回收站在 `MaxUnitPages` 个、同一时刻删掉的一组大于一批。100 个 id 的计划看不出规划器在 500 个时的转换。这条成为长期的规则，写进[总体设计](../../v0.1-design.md) 13.4 第 9 条，计划测试的规则旁。

### 修复的核对（Opus）

两位核对者：A 核对 R1/D1 与 R2，B 核对 D2 与 R3。每轮各在自己的分离的工作树里跑测试、探针与反向对照（B 第一轮另在 `06a5554d` 上开一个作对照），数据库与 Docker 的运行都经 `heavy.sh`，结束时删掉工作树、什么都不提交，不运行任何 MCP 客户端（例外见下面的"过程"）。

**第一轮**（`2b0b162a`）
- 修复的门禁：本机 `make lint knip gen-check test-web build`（vitest 166 个文件 3,344 项）45 秒；`heavy.sh` 里 `make test-go` 866 秒、`make test-go-short` 59 秒、`make e2e` 282 项 41 秒；R1、R2、R3 的提交各自检出，构建、vet 与 `go test -short` 通过，它们的数据库的测试在提交时跑过；三个新的整个程序的测试以 `go test -race -list` 核对落在组合根第三份，第一份没有加。耗时守卫对 `d3a722f2` 交替量，没有变慢的：grep 的 500 个候选的调用中位数 138–148 对 141–161 ms，最坏的一行 0.73–0.76 对 0.71–0.76 秒，一次的工作 1.89–1.92 对 1.86–1.91 秒，glob 490–621 对 417–428 ms，batch 的改写与字节的界线 3.37–3.51 对 3.30–3.36 秒、3.4–4.0 对 3.4–3.8 秒，单元的界线处的撤销 3.95–4.05 对 4.00–4.17 秒，导入的后台撤销 5,373–5,390 对 5,185–5,411 ms，搜索任务建 5,000 页都是 8.98–9.00 秒（时限 26 秒）；D2 的第一版按键读 `page_contents` 两次，让 grep 的 500 个候选的调用慢约两成（142–147 → 160–184 ms），推送之前改为读一次。持续集成 run 39989476490 为绿（竞态 9:57，组合根三份 7:35、6:14、6:17，short 3:17，成本 11:33，守卫 13:19，网页 3:05，e2e 4:31，镜像 1:29）。
- A：R1/D1 与 R2 改对、改全，没有 Important 与 Minor。11 例之外的 13 种形状（改回的页之后被移动或改名、别的页链着它、改回之后一次 W3 的写、冻住的移动的子树之后再移动、相对的链接被改写、经父页删除、之后的移动的改写写它等）与分开调用的正文、索引的指向、`changed` 与回收站都相同，两边的索引都等于重建；R1 的变异在其中 12 种上失败。权重 1 成立：单独的改写约是索引的一页的 0.7–0.8，不重新解析旧的正文，是分开调用给的数，链接与字节的界线照旧数它；`Written` 没有别的读者，`linkRewrite` 是唯一的参与者。R2：`withCurrent` 在 `planned()` 里、事务之外、不取笔记本与正文行的锁读版本，同计划的别的读；至多重新计划一次（第二次页又没了答 `BatchChanged`）；别的答复都不带修订为 0 的令牌（batch 的各条路与单独的工具都从正文行取修订）；测试的换页确在同一路径上；另在一个 batch 里换页（之前编辑与不编辑换进来的页）：答换进来的页 batch 之前的版本（`B:1`），可解析，拿它重发同一个 batch 成功。实施者的四个变异重跑都照说的失败；batch 的组合根、`page/app` 与 linking 的测试与两个性质测试（另加种子）经 `heavy.sh` 都通过。
- B：界线照写的都对，Important 0、Minor 2、细节 4。语句：累加按请求的次序，`ord` 唯一、每个键一行，窗口没有并列；至多恰好一行过界（其后是 0 字节的行时也是）；缺的与删掉的页同时离开连接与累加；重复的 id 会累加两次，没有调用者这样发；大小读存下的 `byte_size`（CHECK 守着它等于 `octet_length(content)`、至多 5 MiB），不解开 TOAST。TOAST：过界的页在四种计划下（主键的嵌套循环、哈希连接加排序、归并连接加排序、整表扫描）都取 0 块，1 MiB 不可压缩的与 4 MiB 压缩后放在行外的都是，界线内读时各取 135 与 9 块；压缩后留在行内的值不到约 2 KB，本来就在行里；TOAST 的测试可靠（计的是缓冲区的请求，不随缓存；事务里的统计不交出去；界线内的一侧至少 100 块，证明计数在动；页不可压缩）。grep：重新分批对；不打转（一批的第一页过界时它已知的大小长过界线，至多两轮之后它单独一批，读或由预先判断停下）；不悄悄跳过；游标与 `MaxRead` 照 4.13，软期限在每次读一批之前（含重读）再看，工作的界线不变。后台撤销的界线对：底本按它的版本的 `byte_size`，不会变，回收站加底本至多 `MaxUnitBytes`，同 `withinSizes` 的和；`errPastPages` 让任务减半，只在多步的一段停下。搜索任务的截断至少留下第一个节点，下一批从长大的页起、它在第一个、读得到。审计（读代码）：推后的改写"在笔记本的锁下是精确的"成立（单元持笔记本行 `FOR NO KEY UPDATE`，每个正文的写经 `Writer.Run` 先取笔记本行 `FOR SHARE` 或 `FOR NO KEY UPDATE`，与它冲突；改写自己的写在每页唯一的一次读之后）；反链一页一页读、按读到的计；请求的撤销与恢复、后台恢复按（id、`deleted_at = t`）读，`WriteContent` 要 `deleted_at IS NULL`，这些行不会变；实施者的表里漏了而不用改的：导出（大小与正文在同一个 REPEATABLE READ 的快照里）、linking 的 reindex、`toggle_task`、`restore_version` 与 `AGENTS`（各读一页）。守卫对 `06a5554d`（干净的树上）：grep 的 500 个候选的调用中位数 130.1 对 131.7 ms，搜索任务建 5,000 页 9.224 对 9.245 秒。R3 的文字准确：只有 `Read` 经 `reads.Get` 判定访问，`Content` 经 `Blobs.Open` 只核对行活着、仍是同一个文件、文件在。实施者的 8 个变异重跑（用例的四个，SQL 的四个，其中返回正文不看界线的两个只有 TOAST 的测试抓得住）都照说的失败。

| 编号 | 级别 | 发现 | 处置 |
|---|---|---|---|
| A-N1 | Nit | D1 之后 `Moved.Written` 是正文操作决定的页，许多注释仍叫它"batch 写的页"（linking 的 `rewrite.go`、`batch_links.go`，`MaxBatchRewrites` 的说明）；`followed` 一处照字面读正是 R1 的缺陷；`MaxBatchRewrites` 的说明没有计 1 的改回的页 | 只改注释：凡把它说成"batch 写的页"的都改为"正文操作决定的页"（`followed`、`reached`、`ownReach`、`writtenMet`、`followWritten`、`ownLinks`、`subtrees`、`spread`、`writtenTree`、`writtenTrees`、`rewrite`、`bytesOf`、`pagesCounted`、读回的注释，`batch_links.go` 两处，`ports.go` 的 `Moved`、`TooManyRewrites.Kept`，数同一个集合的 `MaxBatchRewriteLinks`、`MaxBatchRewriteBytes`），`MaxBatchRewrites` 与 `BatchRewrite` 的说明写明改回的页计 1（`e6db420c`） |
| A-N2 | Nit | `Unwritten` 与 `wrote` 的注释说单元或 batch "没写出它的版本"，同一个单元的推后的改写却可以写：改回 P、再 `move C→D`，P 被改写为第 2 版，在 batch 的变更集里（行为对：日志 `pages_written 0`、`rewritten 1`，答复的版本等于 `read` 的） | 改为"正文操作没写出它的版本，推后的改写可能写"（page 的 `extension.go`、linking 的 `ports.go`、`apply_batch.go`；`e6db420c`）；组合根的一处第二轮补上（A2-N1） |
| A-N3 | Nit | 测试：没有测试撤销改回了一页的 batch；两个性质测试的随机 batch 从不做出真的改回（A 加种子另写改回，25 次里 0 次是真的），只有定向的形状做得出（改名一条链接的目标，再以页的令牌写回它旧的字节：10 个种子 55 次，每次之后索引都等于重建；A 的撤销探针三种形状、含撤销的撤销，都回到确切的状态）；R1 的缺陷放回去当变异，两个性质测试照样通过，看得见 R1 的只有与分开调用的比较；"连批后删掉的也交"失败只因交了零值的 `WrittenPage`，它的 nil 的事实过不了组合根的 `PageFacts`，忠实的写法（去掉 `linksBefore` 里跳过删掉的页）通过整个程序的测试，只有用例的成员测试抓得住：排除是防御性的，R1 一段要说明 | 新的 `TestTheRevertOfABatchThatPutsAPageBackIsTheNotebookBefore`（照 A 的探针：Codex 的形状；再移动改写改回的页；再移动它的文件夹。撤销回到批前、撤销的撤销回到批后，索引每次等于重建，日志 `pages_written 0`、`rewritten 1`，每项的版本等于 `read` 的）；`TestTheIndexIsItsRebuild` 约每 5 步一次定向的改回（A 的形状，有时再改名它链着的另一个节点），另加 A 的探针没有的两项核对：那些链接之后都不指向改名的节点，没有之后的改名时正文是旧的字节（没有它们 R1 的变异通过：索引与错的正文照样自洽）；经别名解析的链接与别名含自己的标题键的目标不算，改名从不到同一个键；推后的核对要至少 10 次真的改回，六个种子 32 次，10.4 秒；R1 的变异在种子 0、1、2 上失败（`7c68072a`）。R1 一段照改（本节的处置） |
| B-M1 | Minor | grep 的一批真实是 500 个 id（`MaxSearchWalk`），`GrepContents` 把次序直接连到表上：`TestGrepPlans` 自己的种子（5.1 万行、1 万已删、500 个活着的 id）上定制计划无统计与 `ANALYZE` 之后都是整表扫描加哈希连接（100 个时是主键的嵌套循环）；守卫的形状（10 万行 2 字节，无统计）6.9–8.0 ms，旧的 `node_id = ANY` 走主键的位图 0.31–0.35 ms；违反 P4 设计 6.1（审查 C-M6）的"None scans a table whole"；计划测试只用 100 个，时间守卫在 `ANALYZE` 之后跑，都看不出 | 照 B 的写法：`GrepContents`、`SearchContents` 在 `MATERIALIZED` 的 CTE 里只按键读行，再把请求的次序连上，界线与 TOAST 的保证不变；在真实的批量、有无统计下找到转换处：grep 500 个在 2,000–50,000 页，搜索 100 个在 2,000–20,000 页；`TrashContentsWithin` 在 `MaxUnitPages`、2,000–200,000 页上一直走 `page_contents_deleted_at_idx`，同 `TrashContents`，这一轮不改（第二轮 B-N5）。`TestGrepPlans` 扩到 `MaxSearchWalk` 个，在原 SQL 上失败；新的 `TestTheContentsWithinABoundAreReadByTheirKey`（1 万页，有无统计、定制与通用计划）查 `SearchContents` 在 `SearchBatchNodes` 个上（原 SQL 整表扫描、失败）、`TrashContentsWithin` 在 `MaxUnitPages` 个上（另有 5,000 个别的时刻删的）与 `PageContentWithin`；TOAST 的测试照样通过；守卫对 `06a5554d` 三轮交替：grep 的 500 个候选的调用中位数 130.6、136.1、143.4 → 130.7、135.5、135.7 ms，搜索任务建 5,000 页（三次里最快的）9.25、9.31、9.41 → 9.13、9.21、9.34 秒（`dce90fcf`） |
| B-M2 | Minor | 四处更窄的界线没有测试，四个变异都通过 `page/app` 的 `go test -short`：grep 去掉 `MaxRead` 项（读到 6.5 MiB，过了 6 MiB 的 `MaxRead`）；`trashBound` 去掉单页的豁免（5 MiB 的恢复加 5 MiB 的写回是分不开的一段，`errPastPages` 之后任务以 `revert_too_large` 停下）；`trashBound` 不减底本；grep 的单页也按 `GrepBatchBytes`（过 4 MiB 的单页每次都过界，重读到期限，只有 `-short` 跳过的时间守卫抓得住） | 四个测试：`TestGrepsReadsHoldWhatMaxReadLeaves`（`MaxRead` 6 MiB，页长到 3、0.5、1、2 MiB：读 4.5 MiB，按字节停在 P3，游标 `p3.md:1`）；`TestGrepReadsAPageAloneWhateverItsSize`（`-short`；单独一页长到 4.5 MiB 整个读，假实现读过它的上限就报错，变异失败而不挂住）；`TestARevertJobsLonePageIsReadWhateverItsBytes`（5 MiB 的恢复加 5 MiB 的写回做完，读回收站 5 MiB）；`TestARevertJobsSegmentPastAUnitsPagesSaysSo` 加一例（3 MiB 的底本、三页长到 2 MiB：回收站的读在 `MaxUnitBytes` 减底本之内）；四个变异都失败（`0b9c3228`） |
| B-N1 | Nit | batch 第二次计划时页又长大：答 `ops[0]` `page.revision_mismatch`，不带它的版本（`gone` 不传版本，落到"那里没有页"的一支；日志也说 `differs: page`）；页还在，`changedTwice` 对有页的正文操作本该答带当前版本的 `VersionMismatch` | `PageContentWithin` 过界时给这一行的修订与哈希（正文仍不读），计划的读把长大的页当作它版本的差别：再次长大时答 `page.revision_mismatch`，带这一页现在的版本、`KeptChanging`，日志 `differs=version`；`TestABatchsReadsHoldTheirBoundOnWhatTheyRead` 两读之前各长一次（`P:4`，读 `[0 0]`），`TestTheContentsAreReadWithinTheirBound` 过界时的修订 2 与哈希；当作删掉、适配器不给修订两种变异都失败（`817aa52f`） |
| B-N2 | Nit | 搜索任务截断之后的节点数两次：截断之前已计 `Kept`、`Skipped`，下一批又计；`deleted` 不截（无害：下一批出队）；只是运行的日志的计数 | 截断时连同要删的行、保留与跳过的节点一起截，只计这一批留下的；`TestTheSearchJobsCutBatchCountsItsOwnNodes`（十页、三页长到 3 MiB，截断之后一行是现在的、一行在 `minRebuild` 之内建过、一行没了：`{Written 7, Deleted 1, Kept 1, Skipped 1}`，写 `5 0` 再 `2 1`）；删不截、计数不截都失败（`cbf0ff82`） |
| B-N3 | Nit | 搜索任务的进展靠两个常量没写出的关系：一批的第一页从不过界只因 `MaxContentBytes`（5 MiB，表的 CHECK 也守着）小于 `SearchBatchBytes`（8 MiB）；常量一改，`cut == 0` 的页会一直留在队列里 | `page/domain/search_bound_test.go` 的 `TestTheSearchBatchHoldsAPagesLargestContent` 钉住 `MaxContentBytes ≤ SearchBatchBytes`，注释写明为什么（代码里没有编译期断言的写法，那个文件已这样钉常量），截断的注释指向它；不加"第一页不限"的规则；一批 4 MiB 的变异失败（`eade85cd`） |
| B-N4 | Nit | R3 新加的一句没有重新折行（`asset/attachment_reads.go` 的一行 106 列） | 重新折行，字不变（`f770bb99`） |

**第二轮**（`7c68072a`）
- 修复的门禁：八个新的提交各自构建、vet 与 `go test -short` 通过，数据库的测试在提交时经 `heavy.sh` 跑过（B-M1 的计划测试、B-N1 的适配器测试、A-N3 的组合根测试）；本机 `make lint knip gen-check test-web build` 36 秒；`heavy.sh` 里 `make test-go` 876 秒（113 个包）、`make test-go-short` 54 秒、`make e2e` 282 项 40 秒。持续集成 run 40001296141 为绿（竞态 10:07，组合根三份 6:43、6:22、6:18，short 2:21，成本 12:03，守卫 12:31，网页 3:12，e2e 4:27，镜像 1:21）。
- A：A-N1、A-N2 修好；A-N3 修好，但它在索引的性质里加的核对可能让对的 batch 失败（A2-M1）；B-N1 合 A 对 R2 的分析：令牌的修订取自行、从不为 0，至多重新计划一次（`withCurrent` 与读的两种都落到同一个 `replanned`，第二次还不同时 `changedTwice` 答），没有新的零值令牌；改过的 R1 一段除一个行号外都准确（A2-N2）。在 `7c68072a` 上跑过的都通过：撤销改回的测试三种形状、`TestTheIndexIsItsRebuild`（32 次改回）、11 例的测试、R2 的组合根测试，`page/app` 的 `TestABatchsReadsHoldTheirBoundOnWhatTheyRead` 与 `TestABatchsMismatchOfAnotherPageTellsItsVersion`；R1 的变异让 `TestTheIndexIsItsRebuild`（种子 0、1、2）与撤销测试的第一种形状（Codex 的）失败；忠实的"连删掉的也交"通过名字合 `batch|revert|links|rebuild` 的全部组合根测试，合 R1 一段的说法。
- B：第一轮的六项都修好、有测试，Important 0、Minor 0、细节 2。四个更窄的变异各被新的测试抓住；`GrepContents`、`SearchContents` 的生成 SQL 放回第一版的连接，`TestGrepPlans`、`TestTheContentsWithinABoundAreReadByTheirKey` 各失败；CTE 的形式里不看界线给正文的，两条语句都被 TOAST 的测试抓住；适配器不给修订的被 `TestTheContentsAreReadWithinTheirBound` 抓住。真实批量的计划：grep 500 个、搜索 100 个，五张表（2,000、10,000、51,000 行各 6 字节，100,000 行 2 字节，50,000 行 1,500 字节），无统计与 `ANALYZE` 之后、定制与通用计划，都在 CTE 里读主键（位图或索引扫描）再哈希连接次序；唯一的例外是 2,000 行无统计时 grep 500 个在 CTE 里整表扫描，0.3 ms，那正是旧的 `node_id = ANY` 语句、D2 之前 grep 的计划；500 个 0.3–0.9 ms，100 个 0.1–0.23 ms。CTE 里过界的页同样不取 TOAST（默认与强制的不用哈希连接、归并连接、整表扫描、`work_mem = 64kB`，两种页都是 0 块，界线内 135 与 9 块）。守卫对 `06a5554d` 三对交替：grep 的 500 个候选的调用中位数 139.8、137.0、141.1 → 156.2、161.6、139.1 ms，两对高 15–20 ms，语句本身在 500 个上只多约 0.4 ms（0.68 对 0.27–0.32 ms），B 第一轮在固定的提交上已见 130–190 ms，实施者三轮持平，时限 1 秒，读作噪声；搜索任务建 5,000 页 9.504 对 9.663 秒。

| 编号 | 级别 | 发现 | 处置 |
|---|---|---|---|
| A2-M1 | Minor | `backed` 的新核对收集曾指向改名的节点 `to` 的每条正文的维基链接，批后还指向它就失败，却只要求被选中的那一行按标题指向它。生成器自己的词做得出两种对的 batch 让它失败（与孪生笔记本上的分开调用相同）：标题链接旁的别名链接（`note` 带别名 `nick`，P 为 `[[note]] [[nick]] x`，改名为 `dup`、写回：`[[note]]` 悬空，对；`[[nick]]` 仍按别名指向它，也对）；改名之后仍指向它的 `.md` 链接（`[[Note.md]] x`，改名为 `Note.md`：链接经另一个键 `note.md` 解析到同一个节点，计划不改写 P，写回是 W3）。六个种子碰巧没有抽到，抽法随 v7 UUID 按文字排的次序，新的种子或词会 | `backed` 的核心改为 `putBack(t, r, p, to, rename)`，只核对改名之后改指别处的链接：曾指向 `to`、自己的 `target_key` 是 `to` 旧的标题键、`target_alt_key`（以 `.md` 写的名字的另一个键）为空或不是新名字的键；`AltLast` 只为 `.md` 的后缀设，所以是确切的；`to` 的别名含它自己的标题键时照旧不做。随机的流不变，六个种子仍 32 次改回。`TestThePutBackCheckOfTheIndexPropertyReadsTheLinksTheRenameLeaves`（组合根第三份）三例，各经 `putBack`、`run` 与 `checkRebuilt`：标题旁的别名（核对 `[0]`、改回 1 页）、改名保留的 `.md` 链接（不核对、0 页）、只按标题的对照（`[0]`、1 页）；第一轮的规则在前两例失败，在核对之处与 `run` 自己的核对里都是；R1 的变异仍让属性测试失败（`5db7b789`） |
| A2-N1 | Nit | 组合根 `bootstrap/linking.go` 的 `batchMoved` 的注释仍说"batch 没写出它的版本"（不在第一轮的清单里） | 改为"正文操作没写出它的版本，推后的改写可能写"，只改注释（`2dbbaab4`） |
| A2-N2 | Nit | R1 一段引 `rewrite.go:398` 说 linking 跳过删掉的页；`e6db420c` 之后它在 `:402` 的 `ownMet` 里 | 段落改称 `ownMet`（本节 R1 一行） |
| A2-N3 | Nit（B-N1 的范围，R2 靠它） | `PageContentWithin` 的 SQL 注释仍说过界时"只给大小"；R2 的保证（没有 `:0` 的令牌）现在靠过界时也读出的修订与哈希 | 注释写明过界时给大小、修订与哈希，不读正文；重新生成时没有套用任何变异（`6a9386ab`） |
| B-N5 | Nit | `TrashContentsWithin` 照旧把次序连到表上：4 万页里 2 万在同一时刻删掉（一组大的后台撤销正是这样）、一批 `MaxUnitPages` 个，无统计时哈希连接 `page_contents_deleted_at_idx` 的位图、读这一时刻的 2 万行（1.07 ms），旧的 `TrashContents` 是两个索引的 BitmapAnd、只读这一批的行（0.20 ms）；`ANALYZE` 之后都走主键（0.125 对 0.065 ms）。一组至多 `MaxJobItems`（6 万）行同一时刻删掉，每批都读全组，约每 2 万行 1 ms | 同样的 `MATERIALIZED` CTE，内层就是 `TrashContents` 的查询，计划也就是它的；`TestTheTrashsContentsWithinABoundReadABatchOfALargeGroupByItsKey`：4 万页、2 万同一时刻删掉、一批 `MaxUnitPages` 个，定制计划无统计与 `ANALYZE` 之后都一次按 `page_contents_pkey` 读，通用计划按键或按 `deleted_at` 的索引（`TrashContents` 自己的注释允许的），都不整表扫描；原写法在无统计的定制计划上失败、新写法通过；TOAST、界线与计划的测试照样通过；新语句的三种 SQL 变异（不看界线给正文、都说在界线内、去掉 `WHERE`）都失败（`b2fc6dd3`） |
| B-N6 | Nit | D2 一段说多了四处："规划器有无统计都会选整表扫描"太强（10 万行窄的有统计时、5 万行 1,500 字节的两种都走键，实施者量的范围是 2,000–50,000 页）；"过界的正文在 TOAST 里"只在约 2 KB 之上，更小的在行里（"不取它的块"仍成立）；"再次长大时 why 是'一直在变'"只对不带 `base_version` 的项，带了的 `changedTwice` 答 `Changed`；不改的地方漏了导出（大小与正文在一个 REPEATABLE READ 的快照里读，是精确的） | D2 一段照改（本节的处置：在几万页以内的表上……有统计时也会；过界的正文若在 TOAST 里；why 照单元第二次的差别答；不改的加上导出） |

**第三轮**（`b2fc6dd3`）
- 修复的门禁：四个新的提交各自 `make lint-go`（0 个问题）、构建、vet 与 `go test -short` 通过；本机 `make lint knip gen-check test-web build` 36 秒；`heavy.sh` 里 `make test-go` 第三次通过（805 秒，113 个包；前两次见"过程"）、`make test-go-short` 57 秒、`make e2e` 282 项 44 秒。持续集成 run 40009214485 为绿（竞态 6:28，组合根三份 6:12、6:29、4:38，short 2:14，成本 9:58，守卫 12:58，网页 4:11，e2e 4:43，镜像 1:21）。
- A（它的第三份报告）：A2-M1–A2-N3 都修好，没有新的。收窄的核对是确切的：只在三条都成立时核对一条链接（曾指向改名的页、自己的目标键是它旧的标题键、`.md` 的键为空或不是新名字的键），工具的解析规则保证这样的链接之后不指向它（旧键与新键不同；`AltLast` 只为 `.md` 的后缀设；标题键也是自己别名的页跳过），仍对地指向它的（经别名或新名字的 `.md` 键）不在其中。确定的测试三例通过；另五种形状经 `putBack` 与性质自己的核对都通过、索引等于重建：别名等于新名字（`[0]`）、大小写的变体 `[[NOTE]] [[note]]`（`[0 1]`）、`[[note.MD]]` 改名为 `Note.md`（`[1]`，保留的那条不算）、别名、`.md` 与标题的链接一起（`[2]`）、路径链接 `[[F/note]]`（核对、对地悬空）；`TestTheIndexIsItsRebuild` 六个种子仍 32 次改回。变异：R1 的规则让它六个种子都失败，确定的测试的别名与只按标题两例、另五种也都失败（`.md` 一例通过，照该有的：那次写什么都不改）；第一轮的核对在别名与 `.md` 两例上、在核对之处与性质自己的核对里都失败，对照通过。改过的 R1 一段准确；A2-N1、A2-N3 的文字照改。
- B-N5：B 不另起一轮，主持者读了 `b2fc6dd3` 的改动：CTE 的内层恰是 `TrashContents` 的查询，对。B-N6 在本节的 D2 一行。

第三轮没有行为上的发现，修复的核对到此结束。

过程：试 SQL 的写法时，实施者的 `make gen-go` 失败了两次（第一轮之前与第二轮之前各一次），失败之前删掉了全部 `gen/` 目录；两次都没有套用任何变异，下一次成功的生成把它们全部恢复。第三轮的门禁里 `make test-go` 失败两次，都不在断言上：第一次 PostgreSQL 崩溃（一个后端以 "unexpected EOF" 退出，之后 "database system is in recovery mode"），那个包之后的测试都连不上；第二次组合根第二、三份里有 500 与请求的超时；两次都与另一个会话的整个 `go test`（约十几个不经我们的锁的 `postgres:18.6` 容器）重叠，第三次通过（第二轮同样的门禁通过过）。B-N5 的第一个提交（`1df7ba34`）有一行 gofmt，提交之后 lint 才抓到；它没有推送过（`git push --dry-run` 看到远端仍在 `7c68072a`），修正并进去，成了 `b2fc6dd3`。核对 B 第一轮的耗时守卫第一遍跑在一个还留着它自己的一个变异的树上（两次变异的运行碰在同一个文件上，留下 `readBound` 的单页的改动；那条路走不到这两个守卫），恢复文件之后在干净的树上重跑，上面取的是重跑的数。核对 A 第三轮另写的五种形状的探针文件随它的工作树删掉了，没有留副本，结果只在它的日志 `r3-tests.log` 里。实施者与两位核对者的报告的文件多被工具拒绝写入，由主持者照它们交回的保存。

合并为 `main` 的 `374730bf`（`Merge branch 'm9-codex': the fixes of M9's Codex review (M9 Codex review)`）。分支的持续集成每轮都为绿（`2b0b162a` run 39989476490、`7c68072a` run 40001296141、`b2fc6dd3` run 40009214485，用时见各轮）；合并之后 `main` 的持续集成：run 40011635327 在 `374730bf` 上全绿（竞态 7:53，组合根三份 5:28、6:33、6:21，short 3:25，成本 12:02，守卫 13:14，网页 4:00，e2e 4:37，镜像 1:24）。M9 仍是实现完成，待人工验收。
