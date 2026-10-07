# M6：Codex 代码质量评审

日期：2026-10-07。评审基线：`555e6ab3b1a1cbc555ddb8a01a8a0ce0bb9b4325`（`555e6ab`）。范围：`de5343333c4d81a78a0bd3547efd89f930e751cb..555e6ab`，核对为 **203 个提交、535 个文件、43,633 行新增、1,360 行删除**，包含文档与生成物。下文源码、测试、设计及旧审查记录的行号均指此基线。

## 1. 结论

**正式发现 3 项：Critical 0、Important 1、Minor 2、Nit 0。建议先修复带引号 YAML 属性中的链接补全，再处理改写日志的内容约定和属性路径碰撞的剩余成本问题。** 没有取得原正文丢失、未授权读取、持久化数据损坏或死锁的证据。

R1 经真实编辑器、保存、API 与数据库验证：选择合法页名 `Bob's` 会把合法属性写成 `ref: '[[Bob's]]'`，使该页全部 frontmatter 属性及其中的标签不再被提取，该属性链接对应的反链也随之消失。原来的其他正文仍在，属于补全和派生语义错误，定为 Important。R2 是错误日志包含原始链接目标／页面标题，违反既定日志约定；R3 是收尾 A-M3 修复没有覆盖同一路径桶内的二次扫描，合法但特殊的 YAML 结构仍触发，均定为 Minor。

**本轮不能宣称全门禁通过。** Go 主模块全量竞态测试、Vitest、203 个既有浏览器故事、生成物检查及构建通过；`make check` 在非竞态 Markdown 成本检查处失败。当时及后续存在其他会话的重负载，未获得可确认的空闲复测窗口，因此这一次门禁计时异常不作为第四项发现。R3 有独立的算法证据与三轮同输入探针，证明范围和计时限制见下文。

## 2. 评审范围与方法

### 2.1 范围、分工与历史去重

主审负责范围、历史去重、P5 读取／权限、收尾数据库改动、门禁与变异；三个分工分别核对 P3/P4 索引与改写、P1/P2/P6A 方言与服务端渲染、P6B/P7 阅读界面与编辑器。主审独立复跑 R1 的服务端解析结果及 R2 日志探针，逐行复核 R3 算法及输入构造，并由后端分工交叉检查三项分级与历史区别。

团队阅读／核对了 `docs/README.md`、总体设计的链接／Markdown／事件章节、§12.4 及 §13.1–13.5、M6 总设计与 P1–P7 文档、M6 reviews 目录全部既有记录、M6 收到的移交，以及发往 M7/M8/M9/M10 和 M12 `M4-performance.md`、`M5-polish.md` 的指定移交。M4–M5 Codex 报告以第 9 节处置后的状态去重；没有把它的七项发现重新计数。M6 改到的 M4/M5 接线在本次范围内，其余旧实现不扩成另一次 M4/M5 全面审查。

方言以仓库保存的 Obsidian 1.12.7 fixture 为准，没有启动 Obsidian 重新采样。正式发现分别与 P7 r1-4／fc4-c2-3、P4 c5-1 及日志相关接受项、收尾 A-M3 区分，见 §4。以下不重新计数：已决定的软换行显示调整、真实 IME 人工清单、超过索引／预算上限时的既定退化、最终守卫之后迟到心跳的接受项、三条语句按参数规划、每连接关闭 JIT、lezer 单行补丁、FA4-N1 百万行倾斜无测试，以及已移交后续里程碑的事项。

### 2.2 隔离、门禁与计数

原仓库开始时干净，`main` 的 HEAD 为指定基线。全部运行工作在仓库外的归档副本中：

```text
/var/folders/9y/fk6s7m2j2rd9yzr_zb1388l40000gn/T/nervewiki-m6-review-zgo7oyxi/
  baseline.tar
  snapshot/
  backend-probes/
  frontend-probes/
  markdown-probes/
  root-probes/
  evidence/
```

五份副本都源于 `git archive 555e6ab`。`snapshot` 建立了本地快照 Git 元数据供 `gen-check` 使用；这不涉及原仓库提交。Go 包并发最多 3；分工同时运行时各用 `GOFLAGS=-p=1`，合计不超过 3；Playwright 总计最多 4 workers。数据库只用本次测试启动的 PostgreSQL `18.6-trixie` Testcontainers；没有操作既有开发库、其他会话容器或进程。没有运行 `make image-smoke`，没有启动桌面应用或读取用户 Obsidian 库，没有修改兄弟仓库。

工具环境为 Go 1.26.2、Node 24.15.0、pnpm 12.8.1、macOS arm64、headless Chromium。下表各组有重叠，不能相加成独立业务用例总数。

| 命令／验证 | 实测结果 |
| --- | --- |
| `pnpm install --frozen-lockfile`，snapshot | exit 0，550 个依赖，1.6 秒；前端探针副本也冻结安装成功 |
| `GOFLAGS=-p=3 make check`，snapshot | **exit 2**。两处 Go lint 均 0 issues；mod tidy、oxlint、格式、类型、knip 通过；fixture 核对 **78 例、32 个 rename 例、15 个 resolution 例**。Go 主模块 `-race -count=1 ./...` **64 个有测试包、19 个无测试包**通过；随后 bodyshape 非竞态成本通过，核心 Markdown 非竞态成本失败，后续步骤没有执行 |
| 上条失败的核心成本：`go test -count=1 -run '^TestTheCostsAreAboutTheSize$' ./internal/platform/markdown` | 包耗时 63.905 秒；`images referred to often` 的约 512 KiB 样本为 583.952334 ms，普通对照 44.851875 ms，超过该项 10 倍判据。存在外部负载；未完成空闲复跑，**不据此认定产品成本违约** |
| `GOFLAGS=-p=1 make gen-check`，snapshot | exit 0，生成物无差异 |
| `GOFLAGS=-p=1 make build`，snapshot | exit 0，包含前端构建；既有产物大小警告保留 |
| `NWIKI_E2E_VERSION=0.1.0-dev pnpm exec playwright test --workers=4`，snapshot/e2e | **203/203**，48.4 秒；使用上述基线构建 |
| `GOFLAGS=-p=1 make test-web`，snapshot，单独补跑 | exit 0；**123 文件、1926/1926**，22.16 秒 |
| `GOFLAGS=-p=1 go test -race -count=1 ./...`，snapshot/server/tools，单独补跑 | exit 0；bodyshapegen 包通过，1.811 秒 |
| M6 六个读取端点权限矩阵，`go test -race -count=1 -json -run '^TestPermissionMatrix$/(prepare\|listBacklinks\|getPageProperties\|listTags\|getTag\|listLinkTargets\|getLinkLanding)' ./internal/bootstrap` | 最终命令 exit 0，包 4.388 秒；**6 个端点 × 13 个调用者＝78 格**通过，另有 prepare 1 项、顶层 1 项。这里表格中的 `\|` 仅为 Markdown 转义，实际正则使用普通管道 |
| R1 新增真实 CodeMirror 探针＋Go 解析对照 | **6 个 Vitest 测试通过**，2.25 秒；**2 个 Go 顶层测试通过**，0.396 秒。四种补全输出均复现缺陷，另有两个符合既定行为的 quoted tag 对照；绿色不表示产品行为正确 |
| R1 新增浏览器故事，`--workers=1 --repeat-each=3` | **1 个故事重复 3 次，3/3 复现**，12.9 秒；正文、API、数据库、反链／标签及 UI 闭环 |
| R2 日志探针 | **1 个新增测试 FAIL**，实际输出原始目标标题；主审独立复跑同样 FAIL，这是正确行为断言失败的证据 |
| R3 路径碰撞与对照 | **2 个新增测试**：碰撞增长断言三轮均 FAIL，对照测试三轮均 PASS；不是新增六个测试，也不是原生 `CheckCosts` 门禁结果，见 §4 |
| 方言字节变体 | **1 个新增顶层测试 PASS**；75 个原 fixture 各派生 CRLF、BOM、两者组合，共 **225 个变体**；核对链接／标签字段与字节位置、HTML 白名单和输出大小 |
| 扩大 3 个既有随机性质测试 | 改写 20,000 种子、后缀解析 20,000 棵树每棵 20 次尝试、索引 3,000 种子每种 60 步且每单元最多 5 次操作，均 PASS；分别 1.52／0.62／14.66 秒。输入扩大，不计为新测试，也不充当成本证据 |
| 定向变异 | **4 类全部被所执行的既有测试检出，0 类存活**；每类先跑同一测试的基线，均通过，见 §6 |

新增探针合计 **13 个顶层测试单元：Vitest 6、Go 6、Playwright 1**。Go 的 6 项为 R1 两项、R2 一项、R3 两项、方言字节变体一项。命名子例、循环样本、生成种子、重复执行与主审复跑均不再加进这个数。

成本限制：初次核心成本异常后，进程／工作目录核查确认另一个外部会话在运行测试；后续仍可见高 CPU Node 等进程。本审查没有终止它们。R3 三轮测量期间已停止本审查其他 Go／浏览器工作，但机器总体并非已确认空闲（当时 1 分钟负载约 14.8–22.9）。因此不把首轮核心成本失败归因于产品，也不声称 R3 的绝对耗时来自空闲机器。`make check` 中止后，bootstrap Markdown、linking app、linking domain、page subtree 的专门非竞态成本步骤未补齐；竞态测试按设计跳过部分计时检查，不能替代它们。

### 2.3 证据口径、工具与复原

正式实现发现使用基线生产代码。R1 调用真实 CodeMirror completion source 与 `acceptCompletion`，将实际输出送入真实服务端解析器，随后用正常 UI 保存故事验证；没有在探针里重写补全算法。R2 使用既有存储／Appender 测试替身，Markdown、预算、索引及 Rewrite 参与者为真实实现。R3 使用真实解析结果和 `PageFacts`，仅在函数外计时，不改生产逻辑。四项变异是另行检验测试敏感度，不用于制造 R1–R3。

证据根目录为上述 `evidence/`：`01-pnpm-install.log`、`02-make-check.log`、`03-gen-check.log`、`04-build.log`、`05-e2e.log`、`06-test-web.log`、`07-m6-permission-matrix-final.jsonl`、`08-tools-tests.log` 与对应退出码；发现证据为 `frontend-completion-cases.json`、`frontend-*-probe*.log`／`frontend-go-probes.log`、`backend-log-probe.txt`、`markdown-path-cost-1.log`、`markdown-path-cost-2-3.log`。主审复核为 `root-frontmatter-recheck.log`、`root-log-recheck.log`；变异脚本、diff 和逐项日志为 `run-root-mutants.py`、`root-mutations.json`、`root-*.diff`、`root-*-{baseline,mutant}.log`。探针源码保留在各自副本；前端另归档在 `evidence/frontend-probes/`，日志探针另存 `backend-review-logs-test.go`，扩大随机输入的可复原补丁为 `backend-expanded-random.patch`。

探索中两次权限矩阵命令的工具问题没有算作产品失败：第一次 `-run` 漏选必须执行的 `prepare`，夹具明确拒绝；修正后 Go 全部通过，但收集退出码的 shell 使用了 zsh 只读变量 `status`。最终改用专用变量重跑，exit 0 和 78 格通过均有完整 JSON 记录。Vitest 日志有 12 条 Node `TimeoutNaNWarning`，构建有 chunk 大小警告，Playwright 运行器有颜色环境警告；不是浏览器页面错误，也不描述成整个过程零警告。

收尾将五份副本中 **全部 1,978 个归档普通文件逐一做 SHA-256 对照，均与基线一致**；即所有生产变异和既有测试的输入扩大都已恢复。新增探针作为额外文件保留，snapshot Git 工作区干净。日志中可识别的 **18 个本次自有容器 ID 均已不存在**；进程核查未发现仍运行的本次快照服务。此处只对可识别 ID 作结论，不把其他会话容器当作残留。最终原仓库只新增本报告，HEAD／分支保持不变；未提交、未推送。

## 3. 发现清单

| 编号 | 级别 | 标题 | 基线位置 |
| --- | --- | --- | --- |
| R1 | Important | YAML 引号内的链接补全未转义所选页名／别名，使属性失效或值改变 | `web/apps/web/src/editor/loaded/link-completion.ts:371–384` |
| R2 | Minor | 改写失败日志记录原始链接目标，包含页面标题／正文 | `server/internal/modules/linking/app/rewrite.go:305–308` |
| R3 | Minor | 同一路径桶内仍逐链接从头查标量，A-M3 修复遗留二次成本 | `server/internal/modules/linking/adapter/markdown/facts.go:94–108` |

## 4. 发现详情

### R1 — YAML 引号内补全没有转义写入值

**现象。** 正常创建合法目标页 `Seed`、`Bob's`，来源页 Source 为：

```markdown
---
ref: '[[Seed]]'
tags: [kept]
status: ready
---
Body remains.
```

编辑 Source，在该链接里用真实补全选中 `Bob's`，保存并退出。实际正文仅将 `Seed` 换成 `Bob's`，得到 `ref: '[[Bob's]]'`；其他正文原字节保留。API 从 `valid=true`、3 个属性变为 `{"links":[],"properties":[],"valid":false}`；Source 的 `page_links`、`page_tags` 行清空，Seed 和 Bob's 的反链都没有 Source，kept 的标签页也没有 Source。UI 显示 frontmatter 不合法的提示。这是用户可由普通补全动作触发的错误。

**证据与命令。** 以下路径中的 `REVIEW_ROOT` 指 §2.2 的完整临时根目录；命令在对应副本执行。

```sh
# frontend-probes 根目录：6 个真实 CodeMirror 探针
NWIKI_REVIEW_COMPLETION_CASES="$REVIEW_ROOT/evidence/frontend-completion-cases.json" pnpm --filter @nervewiki/web exec vitest run src/editor/loaded/review-frontmatter-completion.test.ts --maxWorkers=1
# frontend-probes/server：真实解析＋正确转义对照
GOFLAGS=-p=1 NWIKI_REVIEW_COMPLETION_CASES="$REVIEW_ROOT/evidence/frontend-completion-cases.json" go test ./internal/modules/linking/adapter/markdown -run '^TestReview(FrontmatterCompletion|QuotedFrontmatterTagsAreValid)$' -count=1 -v
# frontend-probes/e2e：真实保存与索引闭环
NWIKI_E2E_VERSION=0.1.0-dev pnpm exec playwright test stories/links/review-frontmatter-completion.spec.ts --workers=1 --repeat-each=3
```

浏览器 3 次均复现上面结果；Go 解析输出由主审独立复跑确认。除页名外，别名也受影响；四例补全前均合法且有一条属性链接与一个标签：

| 引号上下文及所选值 | 实际结果 | 正确转义对照 |
| --- | --- | --- |
| 单引号属性，合法页名 `Bob's` | 单引号提前闭合，整个 frontmatter 无效，links=0、tags=0 | 把新写入值的 `'` 写成 `''` 后合法，目标仍为 Bob's |
| 单引号属性，Plans 的别名 `Bob's plan` | 同上 | 单引号加倍后合法，显示文字保持原值 |
| 双引号属性，Plans 的别名 `He said "Hi"` | 内部双引号未转义，整个 frontmatter 无效 | 内部双引号转义后合法，显示文字保持原值 |
| 双引号属性，Plans 的别名含字面 `a\nb` | YAML 把反斜杠 n 解为换行；frontmatter 仍合法、tags=1，但 links=0 | 反斜杠加倍后合法且 links=1，显示文字仍是字面反斜杠 n |

含双引号／反斜杠的两例使用的是允许这些字符的**别名**，没有用非法页名伪造入口。Go 探针核对了页名通过 `shared.CheckTitle`、别名确实能由真实 frontmatter 提取；精确输入／输出字节保存在 JSON 中。quoted `tags: '#project'`／双引号等价写法不弹出标签补全，符合 P7 的明确选择，仅作对照，不另计发现。

**位置与原因。** `link-completion.ts:244–249` 在 frontmatter 内只检查 `[[` 前是引号；`:267`、`:270–272` 把页名／别名交给不知 YAML 引号上下文的 `writing`。`:374` 和 `:378` 直接拼接目标与别名，`:382–384` 直接写入。服务端 `facts.go:44–80` 按解析结果生成属性、链接和标签事实，因而错误补全进一步影响索引。

**影响与分级。** Important。页面其他已写正文没有被删，存储忠实保存了编辑器输出；已证明的是补全写法错误及可重建的派生信息消失，没有数据库损坏证据。第四例尤其可能没有 YAML 错误提示。修复转义后正常保存即可重新提取。

**建议修法。** 将 frontmatter 的单／双引号上下文传入插入函数，按对应 YAML 字符串规则转义新增目标与别名，并按实际写入长度更新光标／替换范围。已有锚点和显示文字不能被重复转义。补四种值以及新链接／已有闭合链接的回归，并把真实 CodeMirror 输出交给服务端解析验证目标与显示值。

**与已有记录的区别。** P7 r1-4 已解决无引号 `[[…]]` 被当成 YAML 列表；本例从合法的带引号单行属性开始。fc4-c2-3 已解决 frontmatter 被误判表格而写入反斜杠管道；本例与表格无关，是值自身的引号和反斜杠。M12 frontmatter 高亮／块标量／CR/BOM 边界移交没有覆盖这条已支持的普通 quoted property 插入路径。

### R2 — 改写错误日志包含用户的链接目标文本

**现象与证据。** 目标页名为 `Private acquisition plan`，来源页写为：

```markdown
---
x: &x '[[Private acquisition plan]]'
aliases: *x
---
[[Private acquisition plan]]
```

目标改名为 `Renamed`。全量改写会改变 aliases，真实参与者按既定策略退回只改正文；探针先验证正文已成为 `[[Renamed]]`、frontmatter 保持原样，再检查日志。实际输出：

```text
level=ERROR msg="a link is not rewritten: no writing of the page with it was kept" page_id=... start=13 target="Private acquisition plan"
```

在 `backend-probes/server` 执行：

```sh
GOFLAGS=-p=1 go test ./internal/modules/linking/app -run '^TestReviewRewriteLogDoesNotContainPrivateTitle$' -count=1 -v
```

新增测试按“标题不应出现在日志”断言，exit 1；主审独立复跑同样失败。没有人为强制 Rewrite 的失败分支，存储／Appender 用项目既有替身，解析与改写流程为真实实现。

**位置与影响。** `rewrite.go:307` 的 `slog.String("target", l.Target)` 直接写出原始目标。这段字符串来自正文，且本例恰为合法页名。总体设计 `v0.1-design.md:1033`（§13.1 第 10 条）明确要求“标题与正文不进日志”。同类字段还在 `rewrite.go:252–254`，本轮没有独立触发那个分支，不重复计数。已有 `rewrite_test.go:375–386` 反而期待日志含 `target=A/x`，没有保护此约定。

**分级与建议。** Minor，属于日志内容约定不一致。本次没有证明无权用户能读取日志，不按未授权泄露定 Critical。删除原始 `target`，保留 page id、字节位置、必要大小与固定原因；需要标识目标时使用节点 id。调整已有精确日志断言，增加不含正文／标题的测试。

**与已有记录的区别。** P4 c3 F1／c5-1 的 aliases 保护及正文-only fallback 本次实测有效，不重新报告“属性没有改写”。c7–c10 的日志级别、原因和大小接受项没有豁免原始正文／标题字段，也没有修改 §13.1(10)。本项只针对新确认的日志内容。

### R3 — 同 Path 桶中的属性标量仍被重复从头扫描

**现象与根因。** `Scalar.Path` 用点连接层级，键本身也允许点。以下三个合法标量具有同一个 Path `a.a.a`：

```yaml
---
a.a.a: '[[x]]'
a.a:
  a: '[[x]]'
a:
  a.a: '[[x]]'
---
```

`facts.go:94–99` 把同 Path 的标量收进一个切片；`:104–108` 的 `scalarOf` 为每条属性链接重新从切片头按源范围查找。不同标量源范围不相交、每个值含一条链接时，L 个值会进行 `1+2+…+L` 次范围检查。找到的值仍正确，问题是成本。调用位于同文件 `:47–54`。

**输入与实证。** 新探针递归枚举 n 段 `a` 的所有键分割，产生 `2^(n−1)` 个同 Path 的叶子，每叶为 `'[[x]]'`。可复原的核心生成规则为：`Y(0)='[[x]]'`，`Y(n)={ 'a':Y(n−1), 'a.a':Y(n−2), …, 'a.…a':Y(0) }`。最大输入 n=12、2,048 条链接、51,176 字节，实际解析合法，未越过已有 YAML 值数／深度限制；没有扩大到并发或资源耗尽试验。

在 `markdown-probes/server` 运行：

```sh
GOFLAGS=-p=1 go test ./internal/modules/linking/adapter/markdown -run '^TestCodexReviewPropertyPath(Collisions|Controls)$' -count=1 -v
GOFLAGS=-p=1 go test ./internal/modules/linking/adapter/markdown -run '^TestCodexReviewPropertyPath(Collisions|Controls)$' -count=2 -v
```

探针先确认 frontmatter 合法、标量／链接数量正确、路径确实相同、每条链接的引号识别正确；只对已解析 Facts 的 `PageFacts` 计时，每次测量前 GC，每组测 7 次取最快。三个执行轮次如下，不能相加成三个独立用例：

| 轮次 | 512 链接／12,780 B | 2,048 链接／51,176 B | 4 倍链接耗时比 |
| --- | ---: | ---: | ---: |
| 1 | 0.338583 ms | 4.657792 ms | 13.757 |
| 2 | 0.466250 ms | 6.385166 ms | 13.695 |
| 3 | 0.413917 ms | 4.749541 ms | 11.475 |

新增增长断言沿用既有 `facts_test.go:292–323` 的 4 倍项数不超过 8 倍耗时判据，三轮均 FAIL。原测试 `:303–305` 仅生成 `related.N` 唯一路径，因此未覆盖本输入。此处是**新增探针失败，不是基线已有成本门禁失败**。

对照只将键内 `.` 改成 `:`，保持字节数、节点数和链接数相同而路径各异。2,048 项的独立测量中，碰撞 PageFacts 为 4.781–5.276 ms，唯一路径为 1.030–1.242 ms；累计分配约 2.22 MB 对 2.10–2.14 MB。两种结构 Parse+Render 约 9.1–11.1 ms，普通同尺寸文档约 4.55–5.05 ms。**没有证明原生 `markdowntest.CheckCosts` 的 10 倍／14 倍上限失败。**

**影响、限制与分级。** Minor。算法仍可随同路径标量数平方增长，A-M3 的成本保护不完整；静态循环与合法输入身份碰撞是主证据，三轮计时和等尺寸对照是佐证。机器存在外部负载，绝对时延与倍率不能视为空闲基准。最大输入三轮的最快耗时为 4.66–6.39 ms，且规模受 YAML 上限约束；没有保存超时、服务不可用或链接配错的证据，不升级为 Important/Critical。

**建议修法。** 用标量真实身份（范围或序号）索引，或利用源序让各路径桶的查询游标单调前移，避免同一桶反复从头扫描；补同 Path 合法碰撞输入的功能与增长回归。

**与已有记录的区别。** 收尾 `M6-closeout-review.md:42` 的 A-M3 修复原先“每条链接扫全部标量”为按路径建表，通常唯一路径已线性化；本项是修复后同桶形状的遗漏。P6A ordinal 及 P7 r2-5／fc2-c2-3 处理的是点路径歧义下的配对正确性，不等于接受此后端成本。M4–M5 R5 是长整数解析成本，输入、位置与处理阶段都不同。

## 5. 设计层面的疑问

### D1 — 已决定的软换行显示调整应固定哪些语义不变

单个换行显示为空格是已知且已决定修改的行为，不计发现。实施时建议明确视觉换行与文字提取／锚点身份的边界，并补一组跨层回归：

- `obsidian/callout.go:100–103` 清除第一个顶层 Text 的软／硬换行，参与 callout 标题和正文分界；P1-L4 已接受跨行强调／链接节点的标题边界。新增可见换行要同时检查普通标题、标题内强调／链接、后续正文、折叠后再展开。
- `markdown/parse.go:94–95` 的 PlainText、`:131–132` 的 ShownText 把软／硬换行折叠为空格，分别影响标题 ID、图片／属性链接文字。视觉变化是否保留这些值，应明确并钉住，避免原有锚点无意换身份。
- CRLF、BOM、链接／任务字节范围、事件刷新后的锚点和右栏仍应一致。若只改显示，不必因此改提取事实；若确实改变持久化提取语义，要按既有 Extractor 版本／重建约定处理。

### 收官后体验打磨清单（不计发现）

以下保留既有接受／移交，不追加本轮缺陷数：callout／无 id 节点的焦点恢复细节；右栏未解析属性链接的创建入口；frontmatter 高亮和长反链列表的使用体验。真实 IME 第 16–18 步继续按原人工清单验收。

## 6. 测试缺口与变异结果

R1 缺少“真实补全输出经 YAML 解析仍保持原值”的边界；R2 缺少改写失败日志不含页面内容的约束；R3 既有增长测试只有唯一路径。它们与对应实现发现合并计数，不另列三个测试缺口。

四类变异均在独立 `root-probes` 里执行，每类先跑同一基线测试，再改一处、跑相同命令、立即恢复。统一参数 `GOFLAGS=-p=1 go test -count=1 -timeout=90s -run '^测试名$' -v 包路径`；完整参数、退出码、补丁保存在 §2.3 指定证据。

| 变异 | 既有检出测试与包 | 基线／变异及实际失败 |
| --- | --- | --- |
| 反链正文预算停止条件由 `read >= MaxContentRead` 改为 `>` | `TestABacklinksContextsAreOfTheExtractorAndWithinTheRead`，`./internal/modules/linking/app` | PASS／FAIL；实际 3 次读取、contexts `[0 0 1 1]`，期望 2 次、`[0 0 1 0]` |
| Planned.Query 的 DescribeExec 改为 CacheDescribe | `TestPlannedNeedsNoCacheOfPgx`，`./internal/platform/postgres` | PASS／FAIL；关闭 description cache 的合法配置报 `cannot use QueryExecModeCacheDescribe with disabled description cache` |
| 每连接 `SET jit = off` 改为 `SELECT 1` | `TestPoolTurnsJITOffOnEachConnection`，`./internal/platform/postgres` | PASS／FAIL；4 个连接及再次取出的连接仍为 jit on |
| Subtree 层读取从 planned 改回普通 queries | `TestTheReadsOfGrowingArraysArePlannedWithThem`，`./internal/modules/page/adapter/postgres` | PASS／FAIL；`ChildrenOfAll` 出现在缓存语句集合中 |

**4/4 检出，0 存活。** 失败均为相关行为／配置断言，不是编译失败或测试超时。这只能说明列出的定向测试能守住这四个变异，不代表已穷尽所有错误改法，更不代表改坏后的全仓库门禁已运行。

另外扩大三项既有性质测试：`TestARewriteKeepsWhereEveryLinkLeads`、`TestSuffixesResolveAsTheCandidatesOfTheLastKeys`、`TestTheIndexOfUnitsOfManyWritesIsItsRebuild`。标题词元增加 `.md.md`、完整 Unicode 折叠、emoji、NFC 等价写法、单引号、实体样式及空白／标点；仍分别守住链接指向与外部字节、后缀解析等价、增量索引等于重建。纯领域函数的输入扩大不表示每个词元都走过 HTTP 标题规范化。

## 7. 核实过、没有发现新问题的部分

除 R1–R3 外，下列检查未取得新的缺陷实证；结论限定于本次静态核对、既有门禁和上述探针：

- **P1／P2／P6A：** wikilink、嵌入、callout、标签、aliases/tags 按已记录 fixture 核对；225 个字节变体保护 CRLF/BOM 下的范围映射；输出白名单、大小检查、预算取得与释放及 panic 分支有既有测试覆盖。没有将已接受方言差异重新列为问题。
- **P3／P4：** 相对→根→后缀顺序、`.md` 两种读法、别名优先级、子树改名／移动与被抢走的链接、删除和重建的索引维护；写入参与者先后、页面行锁与索引 advisory lock 顺序、`linking.pages_locked` 转换均核对。全量竞态测试包含 `interleavings_links_test.go` 和 `interleavings_rewrite_test.go` 的 8 个顶层交错测试；扩大随机测试继续满足指向保留和增量等于重建。
- **P5：** 六个读取端点的 78 格权限、problem 码及不可见资源响应定向通过；反链上下文上限、32 MiB 内容读取边界、标签／属性／目标读取范围与现有规则一致。预算边界变异被检出。不能由此替代未补齐的非竞态数据库成本门禁。
- **P6B／P7：** 应用内链接导航、锚点落点、callout 展开、任务焦点、事件重读／重连刷新层次、右栏共享 view 数据与分页、补全组合期间的防护、表格边界及缓存范围已核对；既有浏览器故事整体通过。没有把模拟组合事件当作真实输入法验收。
- **收尾改动：** 三条按参数规划语句的接入与无缓存配置、每连接关闭 JIT、子树按层读取由既有测试及本轮三项变异检验；lezer 表格正则 patch、编辑器 inlineLimit／blockDepth 与既有回归核对。未取得推翻负责人接受选择的证据。

## 8. 未能验证的部分

1. **空闲机器上的完整非竞态成本门禁。** 核心 Markdown 首轮失败未在无外部负载条件下复测；其后的 bootstrap Markdown、linking app／domain、page subtree 成本步骤未完成。不能把这次 `make check` 标为通过，也不能仅凭首轮计时认定产品回归。建议在空闲窗口重跑 `GOFLAGS=-p=3 make check`，异常项先同条件复跑再处置。
2. R3 三轮均出现同方向增长，代码复杂度也确定，但没有空闲机器的精确时延基准。没有测试保存超时、服务不可用或更大的并发负载。
3. 没有重新启动 Obsidian 采集结果；没有运行需要桌面应用的 `tools/md-fixtures/obsidian/verify.mjs`，只采用已记录的 1.12.7 行为。未运行 image-smoke。
4. 没有执行真实 IME 人工清单、Safari／Firefox／VoiceOver 人工验收。IME 清单是用户明确排除的待验事项，不作为测试缺口；M4–M6 状态未改，正文 `ce'shi` 的来源疑点留待既定后续验收。
5. 没有补造 FA4-N1 百万行倾斜测试，没有扩展为远程数据库／多笔记本／M12 性能压测，也没有验证线上服务或他人系统。既有接受与移交仍按原记录处理。

## 9. 处置（Claude，2026-10-07）

逐项对照代码核实之后全部采纳：三项发现都成立；D1 在负责人已决定的下一个改动（阅读视图里单个换行显示为换行）里逐条照做；第 8 节第 1 项已复测：`make test-go`（含不带竞态检测的成本步骤）在本机每轮都通过，持续集成每轮也通过（`a7c0988` 那次，GitHub 报 Internal server error，image 与 e2e 两个作业没有启动，下一次提交重跑）。修复在分支 `m6-codex-review`：`7bbf06b` 修三项发现，`cc9ae1e`、`e627541`、`d761526`、`891613a`、`7557dd8`、`a7c0988`、`87e6f5a` 依次修七轮修复核对的发现；合并提交是 `6b9882e`：合并之后 `make image-smoke` 通过，main 的持续集成（运行 37647889910）四个作业都通过。每项都有反向对照。

| # | 处置 | 反向对照 |
|---|---|---|
| R1 | 补全从 frontmatter 的第二行读 YAML，读到 `[[` 之前（`editor/loaded/yaml-quotes.ts` 的 `openingQuote`），读法照服务端的 `go.yaml.in/yaml/v3`。读法覆盖：引号里的字符串可以跨行；`''` 与 `\` 的转义；键与值；流式集合；锚点与标签；块标量与纯量的续行，以节点的列为准；注释；别名；YAML 的另三种换行。`[ ]` 里单独的 `?` 之后紧跟 `]` 时，库把这个 `]` 当成空键、接着在 `[ ]` 里读，与 YAML 不一致；从那里起它不判任何开引号，只会漏补，不会写错（A6-M1，负责人可改判）。属性链接是一个字符串的整个值，所以 frontmatter 里只有紧跟字符串开引号的 `[[` 补全（修复核对 A-Q1）：字符串之中、纯量、块的文字、注释里都不补，那里没有属性链接，别名写进去还可能让 YAML 失效。页的链接与别名按所在的字符串写入：单引号里 `'` 写两次，双引号里 `\` 与 `"` 转义。frontmatter 里不列含 YAML 写不出的字符的页与别名：`[`、`]`、制表符之外的控制字符、U+2028、U+2029、U+FFFE、U+FFFF。测试：`yaml-quotes.test.ts` 的各种读法；`link-completion.test.ts` 在两种引号里选页与别名、在已闭合的链接里选，以及列出哪些项；e2e L5 新增一例，在引号里选页与别名，保存之后 frontmatter 合法、属性的值与索引的链接都对。差分对照：修复核对者的语料都以库为准，到最后一轮合计约 3,090 万个引号（另有前几轮的 121,356 个模板）。服务端能解析的文档里，没有一处在库不开的地方判开。漏补只在上面那条边界之后；能走到这条边界、服务端又能解析的文档，用标准的 YAML 解析器读（yaml 2.9、js-yaml 4.3）一份都不合法（A7-Q1）。每个会补全的位置，写入的结果交服务端解析，都合法，值就是选中的那一项 | 每种读法改一处（不转义、错认开引号、续行与块按行首量缩进、不认上一行留下的节点、不认空键前的锚点、把流式里的 `?` 当纯量、不分其余三种换行，等等），过滤去掉一个字符：各自的用例失败。等价的四个（块里注释不结束纯量；行首单独的 `:` 不要求没有锚点、或不要求在键的起点；`?` 另要求在键的起点）在全部语料上与最终版逐个引号相同，库在能区分它们的位置都报错 |
| R2 | 改写留下的链接，日志只记页 id 与链接的起点，不记目标（目标里有标题，v0.1 总设计 13.1 第 10 条）。两条"留下"的日志经同一个函数（修复核对 B-N1）。测试断言日志里没有目标的文字 | 写回 `target`：用例失败 |
| R3 | 属性链接在同一路径的字符串里，按起点二分查找自己所在的字符串。前提是 frontmatter 的字符串按书写的顺序给出：`Scalars` 的注释写明，`FuzzParse` 核对它们有序、不重叠（B-N3）。增长测试加"一条路径"的形状：12 个键 `a` 的 2^11 种分法，与列表的形状一起，16 倍的数量不超过 48 倍的时间（B-M1、B2-M2、B3-M5，见下） | 改回从头扫描：增长测试失败，竞态、非竞态、`GOMAXPROCS=1` 下都失败。`>` 改 `>=` 是等价变异：链接的范围至少从字符串起点之后 2 个字节开始 |
| D1 | 在软换行的改动里逐条核对：callout 的标题到第一个换行为止；`PlainText`、`ShownText` 仍把换行读成空格，标题的锚点与链接的文字不变；提取结果不变，不涉及提取器的版本；CRLF、BOM 下的字节范围不变 | — |

### 修复的核对（Opus，七轮）

每轮两个核对者，一个看前端、一个看后端，各在快照的副本里跑门禁、差分对照与反向对照。前六轮各有行为上的发现，多是语料扩大之后露出的、修复之前就有的少见 YAML 形状；第七轮没有：前端只差一条制表符的用例（代码本身是对的），后端只有措辞。补上之后不再另起一轮，负责人可改判。

| # | 发现 | 处置 |
|---|---|---|
| A-M1 | 多行纯量的续行被当成 YAML 结构读，错判一直延续到后面的属性 | 纯量缩进过其节点的续行不再开始任何结构 |
| A-M2、A-Q1 | 不在字符串里（纯量、块的文字、注释）时，别名原样写入，`: ` 或 ` #` 会让 frontmatter 失效或值被截断 | 只在紧跟字符串开引号处补全（见 R1） |
| A-M3 | 有三处开始标记的读法与库不同，都会写出失效的 frontmatter：行首单独的 `: `；流式集合里引号字符串之后的 `:`；`#` | 照库读 |
| A-M4、A-N1 | 测试守不住过滤与分隔符；过滤多排除了合法的字符（制表符） | 制表符照列；每个字符一例 |
| A-N2 | NEL、LS、PS 在 YAML 里也是换行 | 按四种换行分行 |
| B-M1 | 增长测试不开竞态时在负载下误报 | 两种数量轮流计时，取五次中最好的一次 |
| B-N1 | R2 的第一处日志没有测试守住 | 两处经同一个函数 |
| B-N2 | `PageFacts` 的错误文字带属性的键 | 按序号指明是第几个属性 |
| B-N3 | `Scalars` 的次序没有不变量测试 | `FuzzParse` 核对 |
| B-Q1 | 访问日志的路径里有标签名 | 日志按路由写路径：slug 与是 uuid 的 id 照写，别的通配符写成参数名（`loggedPath`，访问日志、recover、API 错误、LongLived 都经它） |
| B-Q2 | reindex 撞键时在标准错误上列出页面标题 | 改列页的 id |
| A2-M1 | 纯量的续行与块标量的内容，缩进按行首量，会写坏 | 以节点的列为准：没有自己的键、`-`、`?`、`:` 的行，取上一行留下"值待填"的节点；键前的锚点、标签算作键的起点；锚点名只取字母、数字、`-`、`_`；行首的文档开始 `---` 跳过 |
| A2-M2、A2-N1 | 新读法有几条测试守不住 | 逐条补例 |
| A2-M3 | 别名里 U+2028、U+2029 旁边有空格时，写出的链接服务端不认 | frontmatter 里不列含这两个字符的项 |
| A2-Q1 | 剩下的漏补：键里的字符串；`--- # c` | 前者接受（键里没有属性链接）；后者照库读 |
| B2-M1、B2-N1 | 没有路由接住、落到 `/api/` 子树的请求原样记路径；清理路径后的重定向丢了 id | 子树接住的记成 `/api/...`；id 不是 uuid、或值为空时写成参数名 |
| B2-M2 | 增长测试在负载下仍会误报 | 数量 16 倍对时间 48 倍，每次计时前回收，逐条核对移到计时之外 |
| B2-M3 | recover 与 LongLived 的遮挡没有测试 | 经带 `{tag}` 的路由测试 |
| B2-Q1 | reindex 只给 id，管理员不经 SQL 很难转给能处理的人 | 列出页在网页里的地址：工作区 slug、笔记本 id、页 id（13.1 第 10 条允许管理员看到 slug） |
| A3-M1 | 空键前的锚点或标签，节点的列取错 | 算作该行的节点 |
| A3-M2 | 流式集合里的 `?` 被当成纯量的开头，之后的引号状态反转 | 流式集合里的 `?` 是键的指示符，后面有没有空格都一样 |
| A3-M3 | 上一轮新读法里有 5 条测试守不住 | 逐条补例 |
| A3-M4 | frontmatter 里，页的链接本身不过滤 U+FFFE、U+FFFF（修复之前就有） | 与别名一样过滤 |
| A3-Q1 | 流式集合当键的文档里还有 2 处不同 | 接受：服务端本身就解析不了这样的文档 |
| B3-M1 | 清理路径后重定向到没有通配符的路由，日志写原始路径 | 写路由本身 |
| B3-M2 | 参数绑定失败的 debug 日志带出原值（修复之前就有） | 只记参数名与错误的类型 |
| B3-M3 | reindex 读工作区或 slug 一失败，整条命令就结束 | 退回按 id 列出、错误进日志、继续；只在命令被取消时中止 |
| B3-M4 | 网页路由 `/` 的排除没有测试 | 测试的路由器像 `wire.go` 一样注册 `/` |
| B3-M5 | 增长测试在 `GOMAXPROCS=1` 加重负载下偶发失败 | 超界时继续计时，最多十五次取最好；本机 `GOMAXPROCS=1`、24 个副本并发下 1,440 次 0 失败 |
| B3-N1、B3-N2 | 非规范写法的 uuid 照原样记；注释 | 按规范写法记；注释改了 |
| B3-Q1 | `/api/...` 丢了运维需要的信息：64 个操作的 404 分不出是哪一个 | 维持，负责人可改判：按 `request_id` 关联客户端的记录 |
| B3-Q2 | slug 的值不校验，用户在地址栏敲的进日志 | 维持，负责人可改判：slug 不是页面的内容，网页路径已作为已知例外原样记下 |
| A4-M1 | 流式集合里，纯量之后的注释没有结束这个纯量；第 0 列的 `#` 被当成正文（修复之前就有） | 注释结束纯量，第 0 列的 `#` 也是注释 |
| A4-M2 | 别名 `*a` 被当成纯量的开头，之后的 `:` 读成正文（修复之前就有） | 别名是独立的节点 |
| A4-M3 | 行首单独的 `:` 之后紧跟紧凑映射（`: a: v`）时，节点没有认出来，下面的行整行被当成续行跳过（修复之前就有） | 行首单独的 `:` 与 `- `、`? ` 一样，同一行它后面的键是这一行的节点 |
| A4-N1 | 注释 | 改了 |
| A4-Q1 | 双引号里本可以转义写出的字符（U+FFFE、控制字符）也不列出 | 维持，负责人可改判：两种引号一样处理，与别名一致 |
| B4-M1 | 清理后重定向到网页路由 `/` 的路径，日志写原始路径 | 写清理后的路径，与路由器的清理相同 |
| B4-N1 | 笔记本或工作区被删时 `workspaceSlug` 返回空串，没有测试守住 | 直接对数据库测；reindex 一个已删工作区的笔记本，按 id 列出、不报错 |
| B4-Q1 | CONNECT 的尾斜杠重定向把 `r.Pattern` 设成路径，`loggedPath` 会当成路由读（眼下触发不到） | 加测试：不带方法的路由都不含通配符 |
| A5-M1 | YAML 库把 `[?]` 里单独的 `?` 之后的 `]` 当成空键吞掉，序列没有结束；之后的引号状态读反了，服务端能解析的文档会写坏（修复之前就有） | 先照库读 `[ ]` 之外的 `,`；第七轮改为下面的边界 |
| A5-M2 | 块里的别名、流式集合里第 0 列的注释，测试守不住 | 补用例 |
| A5-M3 | 判为等价的"单独的 `:` 不要求键的起点"，在 `[?]` 之后并不等价 | 修了 A5-M1 之后，在全部语料上与最终版相同 |
| A5-N1 | 注释 | 改了 |
| B5-M1 | 路由器清理的是编码后的路径，CONNECT 不清理；日志清理解码后的路径，会记成路由器既没重定向到、也没服务过的路径 | 照路由器的判断：重定向时记它的目标（解码后），否则原样记 |
| B5-N1 | 根路径的处理没有测试 | `/`、`/lab/../` 都记成 `/` |
| B5-N2 | 护栏没覆盖带 CONNECT 方法的路由 | 不带方法的、方法是 CONNECT 的都查 |
| A6-M1、A6-M2、A6-M3 | 同属 `[?]` 之后：库在那里的读法与 YAML 不一致（纯量、它们的续行、收尾的 `]` 都另有读法），照库模拟的读法仍有误开，测试也守不住 | 不再模拟：`?` 之后的下一个记号是 `]` 时，从那里起不判任何开引号（只会漏补，不会写错），A5-M1 的 `,` 撤回。负责人可改判 |
| A6-N1 | 注释 | 改了 |
| A6-Q1 | 块里注释也该结束纯量 | 照改 |
| B6-M1 | 日志测试只用 GET 与 CONNECT，HEAD 等方法下 B4-M1 的泄漏回归不报 | 补 HEAD、POST、小写 `connect`、带 query、absolute-form 五例 |
| B6-N1 | `webPath` 有两处不起作用 | 化简 |
| B6-Q1、B6-Q2 | 日志写解码后的路径，`%2F` 与 `/` 分不出；`/api` 重定向到 `/api/` 记成 `/api/...` | 维持，负责人可改判：与服务的路径一致，没有内容进日志 |
| A7-M1 | `?` 与 `]` 之间隔制表符的情形没有用例 | 补例 |
| A7-N1 | `[?, ]`、`[?,, '` 照旧读，没有用例 | 补例 |
| A7-Q1 | 边界之后的漏补（`[?]` 之后、序列闭合之后都不补） | 维持：要准确恢复就得模拟库与 YAML 不一致的读法 |
| B7-N1、B7-N3 | 注释 | 改了 |
| B7-N2 | 网页路径里的 `+` 按 query 解码的变异测试不报 | 补例：按路径解码，`+` 照写 |

前端标签页整页加载时，地址里的标签名进访问日志（网页的路径照原样记）。这是已知的例外，负责人可改判：要遮挡就得在服务端认出网页的各个路由。

八次提交的反向对照，按规格去重共 121 个（前端 74、后端 47；代码改写之后重写的同一变异各算一个）。除五个等价变异（R3 的 `>=` 与 R1 的四个）之外，全部被测试检出。中途存活过的都补了用例，再改再跑，例如：注释的读法；块标量取自己的节点；上一行的节点不保留；空值照写；名字不是 id 的通配符装着 uuid；笔记本已删时读 slug。

总设计的约定随之写明：日志里路径的写法与网页路径的例外、改写日志只记页 id 与起点、参数绑定失败的 debug 日志、reindex 列出页的地址（13.1 第 10 条）；增长测试的判据（13.4 第 8 条）。P3 3.6、P7 第 4 节、M0 P4 的错误处理表、M6 总设计的变更记录同步。
