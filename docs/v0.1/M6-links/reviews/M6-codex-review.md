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

## 9. 处置
