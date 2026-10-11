# M7：Codex 代码质量评审

日期：2026-10-11。评审基线：`5d8f282abc04d452e707133f04a05e7b85af48fb`（`5d8f282`）。范围：`629f7417ae685b7a052af9eaca2041d66af190a2..5d8f282`，核对为 **199 个提交、713 个文件、69,277 行新增、1,744 行删除**，包含文档与生成物。下文源码、测试、设计和既有审查记录的位置均指此基线。

## 1. 结论

**正式发现 3 项：Critical 0、Important 2、Minor 1、Nit 0。建议在 M8 之前修复 R1、R2。** R1 是成功上传后的自动插入写坏 YAML，使既有属性、标签和属性链接失效；R2 是允许的超长导入与附件孤儿清扫之间存在文件保留期冲突，任务能报告成功而附件文件已经被删；R3 是合法长标题导出时的命名规划呈平方增长。

R1 已经真实编辑器、浏览器保存、API 和 PostgreSQL 验证，原始正文仍在，不称为正文永久丢失。R2 经真实磁盘、zip、附件存取与清扫实现的控时钟探针验证；**触发前提是 `job_timeout > 24h` 的非默认配置，以及同批文件在挂接前停留超过一天**，没有声称默认六小时配置会出现此路径，也没有实际运行二十五小时。R3 有合法输入、增长测量和等长度对照，没有取得服务崩溃或内存耗尽证据。

规定的门禁均已执行。全量 Go 竞态命令首轮因一个包的 Testcontainers 回收器启动失败而 exit 1；该包按原参数复跑，13 个测试全部通过，其余包首轮通过。六组非竞态代价测试、其余 lint／生成物／前端／构建门禁、**229/229 e2e** 和 **五分钟 zip 差分 fuzz** 均通过。因此没有未解释的业务断言失败，但也不将首轮全量命令改记为 exit 0。

## 2. 评审范围与方法

### 2.1 范围、分工与历史去重

主审负责 P1、P5、跨模块生命周期、门禁、证据与分级；三个专项分别覆盖附件服务端及链接、前端及故事断言、导入及 zip 预读。主审独立复跑 R1 的服务端解析，重新构造无需 503 的 R2 同批文件探针，执行 R3 及等尺寸对照；附件专项交叉核实三项的代码链、已知项区别和分级。

团队已读／核对：`docs/README.md`；总体设计第 12–14 节，包括 12.4、13.1–13.5；M7 的 `00` 与 `01–06` 全文；reviews 下原有 **13 份记录**及 closeout 四轮修复核对、负责人取舍、未验证风险；M7 收到的七份移交；M8、M9、M10、M12 的指定 M7 移交及 `M4-performance.md`、`M5-polish.md` 指定项；`api/modules/asset.yaml`、`transfer.yaml`；M6 Codex 报告及其处置。完整阅读账目在外部证据的 `asset/asset-evidence/reading-and-crosscheck.md`。

范围包含 M7 改到的页面写入单元／观察者、编辑器扩展、阅读增强、链接解析及索引、笔记本删除和活动接线；没有扩展为 M2–M6 全面重审。专门核对 `38c3337..5d8f282` 的收尾差异。方言以仓库保存的 Obsidian **1.12.7** 样例和记录为准，未重新启动 Obsidian。

既有已修、已接受和移交事项不重复计数，包括：多段 Range 整份 200（closeout A-N1）、事件放大列表读（B-M1）、临近提交的上传取消（B-M7／FB-M1）、文件夹拖入（FB-M2 的已修状态）、深路径和目录读预算（A-I1／FA-I1、FA2-M1／F3-M1／F4-M1–N2）、导入单元交错不能假定释放行锁后仍排队、过期媒体重签的体验取舍、配额与磁盘监控、巨幅图像解码、Firefox／Safari、真实 IME 人工验收。R1–R3 各自的区别见 §4。

### 2.2 隔离、门禁与计数

原仓库开始时干净，HEAD 位于 `main`，等于指定基线。所有执行和探针修改都在仓库外的 `git archive 5d8f282` 副本中：

```text
/var/folders/9y/fk6s7m2j2rd9yzr_zb1388l40000gn/T/nerve-m7-review-o8wf7nzm/
  baseline.tar
  baseline/                 # 门禁；git init 后仅在此提交一次快照
  asset/
  frontend/
  import/
  root-probes/
  import-mutants-server/    # 同一归档的 server 子树，供独立变异
  evidence/
```

没有在原仓库 checkout、创建分支或 worktree、commit、push，也没有使用 gh。原仓库只新增本报告。各 Go 运行使用 `GOFLAGS=-p=1`／`go test -p 1`，分配运行时段，包并发合计不超过 3；Playwright 全量用 4 workers，新增故事在全量结束后用 1 worker。数据库只经测试的 Testcontainers 起停；未启停或删除其他容器，未重启 Docker，未操作用户浏览器、桌面应用、Obsidian 库或兄弟仓库。

环境：macOS arm64，模块实际工具链 **Go 1.27.1**（外层默认 Go 1.26.2），Node 24.15.0、pnpm 12.8.1，测试镜像 `postgres:18.6-trixie`，Playwright 启动的 Chromium。下表耗时主要为命令墙钟时间，e2e／fuzz 同时列运行器结果；各组覆盖重叠，不能相加为独立业务用例总数。

| 命令／验证 | 实测结果 |
| --- | --- |
| `pnpm install --frozen-lockfile` | exit 0，550 个依赖；前端探针副本另行冻结安装 |
| `make lint-go` | exit 0，28.009 秒；主模块、tools 均 0 issues，mod 检查通过 |
| `make gen-check` | exit 0，6.368 秒；生成物无差异 |
| `go test -race -count=1 -timeout 15m -json ./...`，baseline/server | **exit 1，927.811 秒**；83 个有测试包通过、1 个包启动失败、22 个无测试包；失败包的 13 个测试均在数据库夹具初始化处退出，未执行业务断言；无 DATA RACE |
| 同参数复跑 `./internal/modules/transfer/adapter/postgres` | **exit 0，4.032 秒；13/13 顶层测试通过**。结合首轮，84 个有测试包均取得通过结果；没有重新执行第二遍全量命令 |
| `make test-go` 的 bodyshape 非竞态成本步骤 | exit 0，0.673 秒；`TestCheckCostsAboutTheBody` |
| 同目标的 Markdown 非竞态成本步骤 | exit 0，45.554 秒；`TestTheCostsAreAboutTheSize` |
| 同目标的 bootstrap 三项非竞态成本／渲染步骤 | exit 0，143.776 秒；`TestTheAppsMarkdownCostsAboutItsSize`、`TestTheAppsMarkdownRendersCheckedHTML`、`TestTheAppsLinksAreWithinTheirBound` |
| 同目标的 linking/app 两项成本步骤 | exit 0，0.990 秒；`TestALinkWrittenManyTimesResolvesOnce`、`TestDistinctPathsToPagesOfOneTitleResolveInTimeAsLongAsThey` |
| 同目标的 linking/domain 成本步骤 | exit 0，1.276 秒；`TestTheLinktextsOfManyPagesOfOneTitleAreCheap` |
| 同目标的 page/postgres 两项成本步骤 | exit 0，4.451 秒；`TestSubtreeReadsWithoutStatisticsInTimeAsLongAsIt`、`TestSubtreeOfAFolderOfMostNodesReadsInTimeAsLongAsIt` |
| `go test -race -count=1 -json ./...`，baseline/server/tools | exit 0，2.333 秒 |
| `make lint-web`、`make knip` | 均 exit 0，5.450／2.840 秒 |
| `make test-web` | exit 0；**143 文件、2490/2490**，Vitest 25.15 秒 |
| `make build` | exit 0，3.251 秒；先构建，再执行下面的 e2e |
| `NWIKI_E2E_VERSION=0.1.0-dev pnpm exec playwright test --workers=4`，baseline/e2e | **229/229**，运行器 58.6 秒；M2–M6 原有故事也在其中 |
| `FuzzReadImportAgreesWithArchiveZip`，`-fuzztime=5m -parallel=1` | PASS，300.761 秒，**11,510,941 次执行**；306 个初始语料，新增 1 个 interesting，未发现差分失败 |
| 附件专项 | asset/app、asset/http、linking/domain、linking/app 四个包通过；另 11 个 bootstrap 顶层行为测试通过 |
| 前端专项 | 14 文件 318 测试通过；另 7 文件 51 项 M2–M6 接线测试通过；与全量 Vitest 重叠 |
| R1 新增探针 | 4 个真实 CodeMirror 输入上下文测试通过（断言实际输出）；真实服务端解析的正确行为断言 **FAIL**；新增真实浏览器保存故事 **1/1 复现**，4.5 秒 |
| R2 新增同批清扫探针 | 正确行为断言 **FAIL**：报告 succeeded、附件数 2、行数 2，而一个 blob 文件缺失；源 zip 已删 |
| R3 新增命名规划与对照 | 两个测量测试各运行一次，均调用原实现；结果见 §4；不是既有成本门禁的失败 |
| 定向反向变异 | **25 个实际修改，23 个被所选测试检出，2 个存活**；存活的是报告上限常量的两种改法，见 §6 |

`make test-go` 按基线 Makefile 的步骤拆开执行，主模块竞态、六组非竞态步骤、tools 竞态均执行，不用竞态模式中跳过的成本断言替代非竞态步骤。精确命令和退出码保存在 `evidence/run-gates.py`、各 `*.result.json`。

首轮失败的直接证据：Testcontainers 启动 Ryuk 后，等待 `8080/tcp` 映射反复得到 `port not found`，最终 Docker 回答该容器不存在；`pgtest.go:56` 缓存这个启动错误，使本包其余 12 项也退出。同参数、未改代码的独立复跑全过。不据此新增产品缺陷，也没有通过禁用回收器或操作别人的 Docker 环境绕过它。

### 2.3 证据口径与复原

R1 使用真实 CodeMirror 上传扩展、服务端 Markdown／PageFacts，以及正常 UI 保存和真实数据库；R2 使用真实 Import、zip 适配器、Local、Blobs、Sweep，Tree／Rows 用既有事务回调替身，并明确时间压缩；R3 使用原 `domain.NewPlan`，没有替换命名算法。变异实验与这些缺陷探针分开，不能把人为改坏的实现当作基线发现。

主要证据在外部临时根目录的 `evidence/`：门禁为同名 `*.log`／`*.result.json`；R1 为 `frontend-paste-contexts.json`、`root-frontmatter-recheck.log`、`frontend-browser-frontmatter.{json,log}`；R2 为 `root-batch-sweep.log`；R3 为 `root-export-growth.log`、`root-export-control.log`；fuzz 和导入变异在 `import/` 子目录；前端变异为 `frontend-mutations.json`、脚本、diff 及逐项基线／变异／恢复日志；附件变异在 `asset/asset-evidence/`。探针源码保留在各自副本。这里只引用测试环境的临时产物，没有把它们写入项目。

一次前端恢复验证曾在初次 lazy load 后查询 Edit 时超过 Testing Library 的 1 秒期限（26/27 通过），原命令不改代码重跑 27/27 通过；与 closeout B-N3 的已知高负载查询相同，不另计问题。日志仍有 Node `TimeoutNaNWarning` 和运行器提示，不表述为整个过程零警告。初次自写成本探针的 `uuid.Must` 编译错误已纠正，正式测量来自编译成功的命令，不计入产品失败或变异检出。

收尾将五份完整副本的 **2,503 个归档普通文件**逐一 SHA-256 对照，全部与归档一致；server 变异副本的 **1,095 个文件**同样一致。新增探针是额外文件，所有生产变异均已恢复，baseline Git 工作区干净。只读核查日志可识别的 **40 个本次自有容器 ID**，均已不存在；未发现本轮遗留的测试服务／浏览器命令进程。原仓库最终只新增本报告，HEAD／分支保持不变。

## 3. 发现清单

| 编号 | 级别 | 标题 | 基线位置 |
| --- | --- | --- | --- |
| R1 | Important | 附件自动插入未按 YAML 引号上下文转义，使整页属性、标签和属性链接失效 | `web/apps/web/src/editor/loaded/asset-upload.ts:259` |
| R2 | Important | 超过一天仍未挂接的导入附件会被孤儿清扫删除，随后导入仍能成功 | `server/internal/modules/transfer/app/import_write.go:208`；`server/internal/modules/asset/app/sweep.go:17`、`:49`、`:72` |
| R3 | Minor | 长标题截短后共用名称前缀，导出从头试编号造成平方成本 | `server/internal/modules/transfer/domain/plan.go:192`、`:229` |

## 4. 发现详情

### R1 — 附件自动插入写坏单引号 YAML 属性

**输入与操作。** 创建普通页面 Guide，再编辑另一个页面，正文为：

```markdown
---
tags: [retained]
image: ''
ref: '[[Guide]]'
---
Body
```

把光标放在 `image: ''` 的两个单引号之间，粘贴一个名为 **`Bob's.png`** 的正常 PNG 文件（带文件、无 `text/plain` 的剪贴板；拖入使用同一插入路径），等待上传完成并按 Ctrl+S 保存。文件名符合当前名称规则，真实上传成功，返回的 `asset.link` 就是 `Bob's.png`。

**实际与应有结果。** 插入后得到 `image: '![[Bob's.png]]'`，内部单引号提前结束 YAML 字符串。保存返回 200，数据库 `revision=indexed_revision=2`，但属性 API 从 valid=true、3 个属性、一条 Guide 属性链接，变为 valid=false、properties=[]、links=[]；数据库 `page_tags`、`page_links` 数量都从 1 变成 0。原来的 `tags`、`ref` 文本和 Body 仍在，错误由自动插入产生。

应在该上下文写出可解析、值正确的字符串，例如 `image: '![[Bob''s.png]]'`，保住 retained 标签和 Guide 属性链接；如果不支持该语法位置，则明确保留已上传附件而不自动插入，不能静默写坏合法 frontmatter。是否在属性表中将这个嵌入显示为图片是另一项语义，不影响必须保持 YAML 合法的要求。

**位置与原因。** `asset-upload.ts:252–259` 在上传完成后检查落点和只读状态，然后直接拼接 `embedOf(asset.link)`；没有读取落点的 YAML 引号上下文。M6 的链接补全已有引号处理，但这里是 M7 新增的独立写入入口。问题不在后端存储丢掉了正文：后端正确保存了错误的编辑器输出，并按无效 frontmatter 重建派生事实。

**证据。** 四个真实编辑器探针中，单引号 YAML 复现；双引号 YAML、正文、围栏代码为对照。主审把真实输出交给真实 Markdown／PageFacts，单引号子例的“原有事实应保留”断言失败，双引号子例通过。随后新增浏览器故事经正常上传和保存复现上述 API／数据库结果，测试刻意断言缺陷现象而通过；不以绿色探针冒称修复。

可在保留的副本复跑：

```sh
# root-probes/server；正确行为断言会失败
GOFLAGS=-p=1 GOMAXPROCS=2 go test -count=1 -v -run '^TestM7ReviewPastedFileKeepsFrontmatter$' ./internal/modules/linking/adapter/markdown
# frontend 根目录先 make build；再在 frontend/e2e
NWIKI_E2E_VERSION=0.1.0-dev pnpm exec playwright test stories/asset/m7-review-frontmatter.spec.ts --workers=1
```

**影响与分级。** Important。正常写者用合法文件名即可触发，保存后影响全页属性、标签和属性反链；修正引号并重存能恢复派生信息。没有证明原正文永久丢失或越权。

**与已知项的区别。** M6 Codex R1 已修的是链接补全，基线保有其转义和回归；本项是 M7 的附件上传完成回调，未复用该保护。P4C 的 A L3／B L6 是同步拒绝上传时误删选区，A L5 是撤销分组；本次上传成功、落点正确，错误来自所插入内容。closeout FB-M2 的文件夹过滤和 IME 人工清单也不覆盖此问题。

**建议修法。** 在实际插入时按映射后的最新落点识别语法上下文，对 YAML 单引号内容做单引号加倍；其他上下文不能盲用同一种转义。复用已有 YAML 上下文读取能力，避免再造不一致的扫描器。回归至少经过“编辑器实际输出→后端解析→保存后索引”，断言其他属性、标签和属性链接保留，并覆盖上传等待期间落点移动及多文件插入。

### R2 — 活导入的待挂接文件早于任务期限被当成孤儿

**触发条件与操作序列。** 将合法配置 `transfer.job_timeout` 设为七天，导入一个同批含至少两个附件的 zip。首个附件已经 Put／Commit 到 `blobs/`，后续附件的解压或磁盘 IO 很慢，导致首文件在本批挂接前已经超过 24 小时；任务仍存活、心跳可继续。此时正常孤儿清扫运行，随后后续文件完成，本批创建节点和 blob 行，导入结束。

这不是“网络上传 zip 用了二十五小时”：网络接收已结束，问题发生在后台导入的文件阶段。该阶段不受 HTTP 上传的最低速率／一小时附件传输限制约束。

**实际与应有结果。** 清扫只看到首文件的真实修改时间已过一天、数据库还没有 blob 行，于是删除文件。随后 `Blobs.Attach` 只插入 blob 行，导入报告 succeeded，附件计数照常增加；源 zip 又按结束逻辑删除，成功附件实际没有可读文件。应保证活任务仍持有的文件不被当作孤儿，或在越过安全期限之前终止／重建该批，不能报告成功且留下缺文件的活行。

**位置与原因。**

- `transfer/app/import_write.go:208` 先完成整批 `r.put`，`:217–231` 才进入创建单元；`:260–270` 顺序 Put 每个附件，只在内存里保留其 File。
- `asset/app/blobs.go:78–85` 已提交真实文件，`:114–115` 的 Attach 只调 `CreateBlob`，不具备在途所有权协议。
- `asset/app/sweep.go:14–17` 固定 **24h**，注释的依据是它远长于文件提交到行提交的时间；`:49–61` 只按 KnownBlobs 判断，`:72` 用文件年龄选取。
- `platform/config/config.go:270` 允许任务最长 **7d**，`validate.go:275–285` 没有与孤儿年龄交叉约束。这个组合打破了上述清扫依据。
- `transfer/app/import.go:85–92` 在正常结束之后删除源导入 zip。

**动态证据及边界。** 新增 `TestM7ReviewLongBatchCanLoseItsPendingBlob` 使用两个小附件、真实 zip 上传存储、真实 Import／Local／Blobs／Sweep。第一个文件实际 Commit 后，在第二个 Put 的钩子执行清扫，将清扫时钟置为首文件真实 `ModTime()+25h`；**不修改文件 mtime，不制造 503，不重试 Tree.Import**。任务上下文为合法的七天期限。

输出为：

```text
before second Put: simulated age=25h; deleted=1; rows-before=0
end state=succeeded attachments=2 rows=2 attempts=1
source zip lookup=storage: no file at the key
7-day context.Err=<nil>
successful attachment ... has no file
```

按“成功附件必须有文件”断言，测试 exit 1。Tree／Rows 使用既有事务回调替身，未用真实 PostgreSQL 执行这一个新探针；真实生产 Attach 的只插行行为为静态补证。只推进 Sweep 时钟是在压缩本批等待时间，不是宣称实际跑过 25 小时、改变了系统时钟或完成了持久存储故障压测。

```sh
# root-probes/server
GOFLAGS=-p=1 GOMAXPROCS=2 go test -count=1 -v -run '^TestM7ReviewLongBatchCanLoseItsPendingBlob$' ./internal/modules/transfer/app
```

早期另做过 Tree 答 503 的重试探针，但没有确认当前生产 create 路径会在 Put 后返回那种 503，故不以其为正式触发依据；正式结论只用无需重试的同批文件链。

**影响与分级。** Important，支持配置下存在成功记录与文件不一致及服务端附件数据丢失。需要非默认长任务及极慢 IO／长时间停留，不将其描述为默认配置常见问题；客户端若还保留原 zip 可以重新导入，服务端自身已不保留可恢复的源包。没有证明其他用户可因此读取附件。

**与已知项的区别。** P1 A-M1 已钉住“修改时刻是真实时间”，本探针 mtime 完全真实，失效的是后续跨模块时间假设。P2 A2 修的是清扫运行期限 50 分钟，与候选文件年龄 24 小时不同。P6A A-6／F2-1 的超时测试核对任务结束和批边界，没有让清扫与活任务待挂接文件相遇。提交结果不明时保留文件、等待以后清扫的既有取舍仍应保留；本项删掉的是尚会成功提交的活任务文件，不是确定无主文件。M12 配额／磁盘监控移交也没有接受这种成功后缺文件的结果。

**建议修法。** 明确 Put 到 Attach／Drop 之间谁持有文件：可以先把可配置任务期限与清扫宽限建立严格约束并留余量；或给导入中的 blob 建可续期的任务持有记录，清扫跳过活持有者，任务结束后释放。单纯在 Attach 前 Open 一次仍与清扫的“查行后删文件”竞争，不能作为完整修复。增加首文件已提交、后续文件延迟、清扫、挂接的确定性交错回归，守住成功行对应文件必在。

### R3 — 导出长标题的编号分配重新从 2 扫描

**输入与操作。** 在同一父页下放 N 个合法页面，每页正文一个字节，标题为：

```go
strings.Repeat("a", 250) + fmt.Sprintf("%05d", i)
```

`i=0..N-1`，N 分别取 1000、2000、4000。标题均为 255 字节，各自标题键不同，探针逐项经过 `shared.CheckTitle` 验证；没有伪造数据库中不合法的兄弟节点。导出时加 `.md` 超过段长，必须截短并编号；编号之前原本用于区分各页的末尾数字被截去，所有页落入同一个候选名称族。

**实际与应有结果。** 文件最终命名正确，但 `plan.go:229` 对每一个节点重新从 2 尝试，`:235–237` 不断构造相同的已占名称并折叠标题键。N 个兄弟约需 `1+2+…+N` 次查找。应在保持保留原名、确定顺序和避让文件／文件夹规则的同时，复用已走过的编号范围，使成本随节点数近似线性。

**实测。** 原 `domain.NewPlan` 的一次测量如下。对照只把五位数字挪到标题开头，长度、数量、正文和截短要求相同，截短后前缀各不相同。

| 节点数 | 尾部数字被截去：耗时 | 同输入累计分配 | 开头数字对照：耗时 | 对照累计分配 |
| --- | ---: | ---: | ---: | ---: |
| 1,000 | 632.57 ms | 263.18 MB | 10.67 ms | 6.13 MB |
| 2,000 | 2.536 s | 1.042 GB | 21.10 ms | 12.58 MB |
| 4,000 | 10.332 s | 4.148 GB | 45.60 ms | 25.24 MB |

上表 MB／GB 为十进制，分配是 `runtime.MemStats.TotalAlloc` 的差值，**不是峰值堆**。4000 项的测量结束堆增量约 8.6 MB，不能写成占用 4 GB 堆或 OOM。机器不是专用空闲基准机；静态循环、两次规模翻倍约四倍时间／分配，以及等尺寸对照共同构成证据，不把绝对秒数当容量承诺。

```sh
# root-probes/server
GOFLAGS=-p=1 GOMAXPROCS=2 go test -count=1 -v -run '^TestM7ReviewExportLongName(Growth|Control)$' ./internal/modules/transfer/domain
```

**位置、影响与分级。** `transfer/domain/plan.go:179–201` 的逐兄弟重命名调用 `freeName`，`:224–239` 每次从 2 开始。`transfer/app/export.go:137–159` 在快照中调用规划；`NewPlan` 没有上下文检查，规划结束前也不能响应任务取消。影响导出 worker 的 CPU、累计分配和持快照时长；本轮只实测到 4000 项，没有全服务卡死或错误内容证据，定 Minor。

**与已知项的区别。** P6A A-3 及第一轮“A-3 的余项”已修的是**导入**单元里同键兄弟的编号缓存；此处是**导出**时不同合法标题截短后碰撞，另一份实现。P5A A P2-2 的按正文大小分批解决的是正文读取内存，不约束命名规划循环；M12 的 GB 级附件复制／负载测试也不等于接受该确定的平方算法。

**建议修法。** 用能复用候选编号进度的命名分配器，按截短后的碰撞族处理，并保留原始兄弟／文件／文件夹预留名的避让；注意编号位数增长会改变截短长度。补这种不同原名共用截短前缀的功能与相对增长测试，另在大规划阶段设可取消的检查点。无需改变导出的命名语义。

## 5. 设计层面的疑问

### D1 — 文件提交到模块挂接之间的所有权，应由哪一层保证

R2 暴露的不是应否保留孤儿的取舍，而是 `Attachments.Put / Attach / Drop` 的跨阶段契约只描述操作，没有表达活任务的持有期。普通 HTTP 上传依靠短期限暂时满足清扫假设，P6 的最长七天任务改变了调用条件。

建议在修 R2 时作一个明确选择：继续用时间上界保证，就让配置校验和清扫年龄使用同一个可验证约束；采用任务持有记录，就让 transfer 通过端口表达持有／释放，由 asset 或存储侧决定文件何时可清。无需让 page、asset、transfer 相互导入。M8 恢复、M10 来源区仍应经同一附件生命周期，不能各自另写一次“先文件、后行”的隐含约定。本项是 R2 的设计决策，不再计一条发现。

### D2 — M10 的贡献者是否只承诺有界的小文本

`transfer/app/contributors.go:33–37` 的 `Sink.Add(path, content []byte)` 要求调用者先构造整份内容；`export.go:256–288` 顺序调用贡献者，Add 内部没有独立字节／文件数预算。贡献者运行于快照内（`:137–159`），因此大虚拟 log 的构造和压缩会延长快照。

当前组合根交空，不能据此认定 M7 已有不受限的用户入口。M10 移交第 1 项已经要求贡献者最后一跳行为测试，本轮不重报这项已移交工作。需要在注册首个贡献者时确定的是：虚拟 index／log 是否保证大小、条目数和生成时间有界；若保证，写明上限并让贡献者守住；若需要很大输出，再提供带上下文的流式写入口和统一额度。不要仅为未来可能性现在增加通用插件框架，也不要使所有贡献者自行重复实现取消与预算。

## 6. 测试缺口与变异结果

### 6.1 与本轮发现对应的缺口

R1 缺少“实际附件插入结果仍是合法 YAML、已有属性事实保留”的跨层断言；R2 缺少“活导入已提交但未挂接的 blob 遇到清扫”的时间边界；R3 只有少量长名／碰撞功能样例，没有大量不同原名截短为同前缀的增长形状。这三项分别并入 R1–R3，不重复计数。

另有一个未独立计为发现的契约测试缺口：`transfer/domain/report_test.go:16` 用 `domain.MaxProblems` 决定输入数，`:23` 又用同一常量决定预期。把实现上限 **1000 改为 2000** 后，`TestAReportKeepsItsFirstProblems` 仍 PASS，尽管 `api/modules/transfer.yaml:556`、`:561` 明确写前 1000 条。直接去掉截断守卫则 FAIL，说明测试守住了截断机制，但没有独立钉住协议中的固定数字。建议输入固定 1001 条，断言恰保留 1000、顺序正确且 truncated=true。基线实现本身符合上限，没有以此宣称当前报告失控。

### 6.2 真实反向对照

每次只在副本改一项实现、执行表中对应的既有测试并恢复。前端逐项有基线／变异／恢复；后端对应包的原实现由本轮定向或全量门禁验证。下列 23 个检出均为相关行为断言失败，不靠编译失败认定有效，也没有声称每个变异都跑了全仓库或全部 e2e。

| 实际变异 | 所选既有测试／入口 | 结果 |
| --- | --- | --- |
| 签名忽略 blob；签名忽略下载标志（2 项） | `asset/adapter/mac`：`TestSignIsTheKnownAnswer`、`TestValidChecksEveryValueAndTheExpiry` | 2/2 检出 |
| 解析答复丢掉 Asset 标志 | `linking/domain`：`TestTheResolutionCasesResolveAsWritten` | 检出 |
| 附件改名允许去掉扩展名 | `page/domain`：`TestCheckAssetRename` | 检出 |
| CSP 的 img-src 允许任意 https 外站 | `asset/adapter/http` 内容响应头测试 | 检出 |
| 会话换代不终止旧上传；不保留已占文件名（2 项） | `stores/asset.store.test.ts` | 2/2 检出 |
| 阅读附件不设新标签页；不接回播放中的媒体（2 项） | `reading/assets.test.ts` | 2/2 检出 |
| 上传结束忽略只读状态；使用未映射的原落点（2 项） | `editor/loaded/asset-upload.test.ts` | 2/2 检出 |
| 从编辑器注册表移除附件扩展 | `pages/page/page-asset-upload.test.tsx` | 检出；恢复后的首次加载超时及原样复跑见 §2.3 |
| 导入上传离开时绕过阻挡；文件夹拖入不先阻止默认行为（2 项） | `pages/notebook/transfer-import.test.tsx` | 2/2 检出 |
| XHR 上传不交令牌 | `services/asset.service.test.ts` | 检出 |
| 不检查压缩比 | `TestAnImportSkipsTheEntriesItCannotRead` | 检出 |
| 不预留后续兄弟原名 | `TestAnImportNumbersNoNameALaterSiblingHolds` | 检出 |
| 批正文字节由 8 MiB 放大到 16 MiB | `TestAnImportWritesInBatches` | 检出 |
| 太深而未挂接的附件不删除文件 | `TestAnImportSkipsWhatIsTooDeep` | 检出 |
| 不拒绝 zip 数据区重叠 | `TestAnImportsArchiveRefusesItsDirectory` | 检出 |
| 把未获准上传也计入在途配额 | `TestTheUploadsCountAsTheirRowsAreWritten`、`TestTheUploadsUnderWayCountForEveryStart` | 检出 |
| 不按结尾宣称条数提前拒绝 | `TestAnEndOfMoreRecordsThanTheMostIsRefusedUnread` | 检出 |
| 报告截断守卫整体去掉 | `TestAReportKeepsItsFirstProblems` | 检出 |
| `MaxProblems` 1000→999；1000→2000（2 项） | 同上 | **2 项存活**；前者仍满足“至多1000”，不当作资源上限失守；后者证明固定1000未独立钉住 |

合计 **25 个变异，23 个检出、2 个存活**；两个存活同属一个常量边界，不算两项缺陷。完整补丁／命令／退出码见 `asset/asset-evidence/mutation-results.json`、`evidence/frontend-mutations.json`、`evidence/import/mutants-results.json`、`evidence/root-report-cap-results.json`。导入脚本把 1000→999 命名为 `zero-problems-limit`，名称不准确，统计按实际 diff，不按标签推断变异内容。

### 6.3 AS1–AS5、TR1–TR4 的逐句核对

核对了九份故事及共用数据库／文件断言的具体内容，完整逐句映射在 `evidence/frontend-story-assertions.md`，附件专项独立核对 AS 表及 TR1–TR4。以下压缩列出每句承诺及证据，不能把故事标题或 229 条通过当作覆盖证明。

| 故事 | 分句与实际断言 | 限制／本轮缺口 |
| --- | --- | --- |
| AS1 | 页下／根下上传、按父节点列出、元数据、打开和下载原字节；页面真实 filechooser／拖入、改名保留扩展名、删除后的焦点；`expectUploaded` 查 node／changeset／blob／SHA 和真实文件；16 MiB 限速上传断言非零进度并中途取消，查行和目录无残留；`.md` 不发送请求 | 晚取消仍可能已提交是 B-M7／FB-M1 已接受项；原生桌面拖动未验证 |
| AS2 | ClipboardEvent／DragEvent 进入真实 CodeMirror；上传后继续键入、坐标拖入后精确全文；`expectEmbedded` 查正文、父节点、附件、索引，阅读图片 naturalWidth>0 | 原故事仅普通正文、安全图片名；缺少 R1 的 YAML 引号上下文 |
| AS3 | 图片、Ogg、WebM 的签名 src／类型及真实加载；PDF 新 tab、ZIP 下载字节、未解析标记；属性附件链接；改名后的精确正文与索引；换 HTML 后音频为原元素且继续播放 | Markdown 附件写法由 051–053／061 fixture 代替故事是 B-N1 已接受项；跨小时到期主要由定时器单测守住 |
| AS4 | 删除子树／笔记本同刻软删 blob，旧地址404、范围外附件仍200；页面确认框数出子页／附件；后台到期清理行与文件、较新数据保留；无主活动的字节与最近时间含附件 | 后台清理仅 API 版本是总设计明确例外，closeout B-I1／C-I4 的页面补测在基线上存在 |
| AS5 | 类型矩阵的 content／download 响应头、Disposition、CSP／CORP／缓存、原字节；Range206和条件304；SVG 脚本被 sandbox 拒绝、外站 CSS／图片失败原因是 CSP；HTML 真实 download | 外站 route 中止与 CSP 失败有区分，未真的外访；原生 PDF／跨浏览器媒体仍属既有移交 |
| TR1 | 整本／子树 zip 路径完整数组，正文／图片字节、空页规则、冲突文件夹；meta 最后写、原 id／kind／root／真实 sort_order；页面下载实际 zip；新导出使旧文件到期、旧URL404 | 页面数据较小，长标题大规模增长未覆盖，见 R3；贡献者最后一跳在 M10 已移交 |
| TR2 | 去外壳、隐藏项过滤、建页／附件、名称编号、报告逐条；导入结束 done/total 与真库相同；导到既有页下；库内链接和原有未解析链接解析正确；上传中离开只停止一次，忙时早拒绝 | B-M8 明确只在 e2e 核结束进度；中间 200/250 由取消两批的 app 测试断言。活批遇清扫是 R2 新缺口 |
| TR3 | 把实际下载的导出包导入另一笔记本，`treeOf` 比层级／顺序／正文及真实附件 SHA；Guide 链接指向新库对应页 | B-N2 的 `Plan.md` 文件夹改名例外被精确限定，未放宽成任意树差异 |
| TR4 | 不安全路径、特殊类型、压缩比、重复、无效正文／CRC、深路径逐项拒绝／跳过和报告；精确剩余树及 blob 集合；非 zip／目录／宣称条数整包失败不写；真实实例的上传／解压大小上限 | API 二进制矩阵比页面版本更宽；没有把它等同于所有可能 zip 形状。收尾深路径／读预算修复另由既有回归和本轮 fuzz 覆盖 |

## 7. 核实过、没有发现新问题的部分

除 R1–R3 外，以下范围未取得新的缺陷实证；只是本轮核对边界，不是无缺陷保证：

- P1：真实文件修改时刻、临时文件和启动检查、ENOSPC／EDQUOT 映射、流式读写截止时间、无请求体下载、低速率及停机阶段；没有把单实例和磁盘余量软约束重新列为问题。
- P2–P3：multipart 原始字节与前置限制、`.md`／标题键／重名规则、派生签名密钥与期限和两种桶、Range／条件请求／响应头／内联类型、可见性、三条笔记本删除路径及先文件后行清理；附件三种链接写法、`resolved_asset`、改名移动的原子改写与编辑锁；MaxShown／媒体数量及附件标记。
- P4 及旧模块接线：队列、取消／进度、焦点与播报、分页、文档拖放保护、只读／组合／离开、重签与到期重读、媒体保留、SWR／MobX 换代；通过既有单测、变异和全部浏览器故事核对。
- P5–P6：快照、正文分批、zip 路径和 meta、River 心跳／取消／救援／结束／TTL／队列与连接预算；zip64、结尾与起点修正、被拒记录的读预算、数据区不重叠、分类／NFC／Windows 路径、批上限／编号／位置变化、解压限制、上传在途计数及报告截断。五分钟差分 fuzz 没有新的反例。
- 并发与模块边界：检查 notebook／页面内容／索引／transfer 行的加锁先后及组合根端口接线；全量竞态运行包含相关 bootstrap 交错。对导入下一单元可能先抢到刚释放行锁的测试，按 13.4 第 4 条核对两种允许结果，没有将有限次数重复误称为强制排队。没有新增已复现死锁，未要求 page／asset／transfer／linking 相互导入。

## 8. 未能验证的部分

1. **没有一次从头到尾 exit 0 的全量竞态命令记录。** 首轮的唯一失败包是 Ryuk 初始化故障，已独立原样复跑全过；这提供了全部有测试包的通过证据，但保留首轮失败记录，没有伪称两次相加就是一次全绿。
2. R2 没有真实等待 25 小时，没有在真实 PostgreSQL 上做该新时间交错，也没有通过真实磁盘长时间阻塞制造它。动态证据是控 Sweep 时钟、真实文件与归档、事务回调替身；源码证明支持配置允许该持有时窗。未证明默认六小时配置同样触发。
3. R3 没有空闲专机上的容量基准，没有扩大为多租户／大规模并发或整机崩溃实验；累计分配不等于峰值内存，4000 项之外的绝对延时不作外推结论。
4. 未实际填满磁盘或耗尽文件系统配额、未做物理断电／不同挂载系统的持久化试验；507、启动和残留清理以现有故障注入、契约及集成测试为证据。未运行会自行管理额外容器的 image-smoke，未验证生产部署或反向代理。
5. 未启动 Obsidian 重采方言，未执行真实 IME、系统剪贴板、Safari／Firefox、VoiceOver 及真实 PDF／媒体查看器人工清单；这些按用户指定和原移交保留，不新增计数。
6. 反向对照是列出的 25 项定向实验，没有对 69,277 行全部做变异，也没有逐句对九个故事都改坏后再跑 e2e；报告条数固定数字的存活已如实列出。M8／M9／M10 尚未注册的功能只能核对端口和移交，不能代替后续行为验收。

总体结论：**有必须在 M8 之前修的问题：R1 的 YAML 自动插入错误和 R2 的活导入文件保留期冲突；R3 应补命名算法与增长回归，但本轮证据不把它升级为 M8 阻断项。**

## 9. 处置（Claude，2026-10-11）

逐项对照代码核实之后全部采纳：三项发现都成立；D1 取时间上界（见 R2），D2 写进 M10 的移交；6.1 的测试缺口补上；R3 的同一种平方另在导入的撞名序号里找到，一并修。第 8 节第 1 项：首轮全量竞态命令失败在 Testcontainers 的 Ryuk 起不来，与代码无关；持续集成每次都跑全量。修复在分支 `m7-codex`：`280c974` 修三项发现与测试缺口。每项都有反向对照。

| # | 处置 | 反向对照 |
|---|---|---|
| R1 | 上传完成的嵌入按落点写入（`editor/loaded/asset-upload.ts` 的 `embedAt`）。落点在 frontmatter 里（闭合的，或正在写、还没闭合的，照链接补全的 `frontmatterAt`，它从 `link-completion.ts` 移到 `yaml-quotes.ts`）时，只在字符串的开引号之后插入，按那个字符串写（`quoted`：单引号里 `'` 写两次；名称里没有 `"` 与 `\`）；名称含 YAML 字符串里写不出的 U+FFFE、U+FFFF 时不插。frontmatter 里别的地方（纯量、字符串中间、闭引号之后）与它的 `---` 行上（第一行不论闭合与否，以及闭合的那一行；`fenceAt`）不插入：附件照常上传、列在附件一节，编辑器说"已上传，未插入：在 frontmatter 里，嵌入只能紧跟在属性值的开引号之后"。一批几个文件时第一个插进字符串，其余的落点已不在开引号之后，同样说明。落点按插入时映射后的位置判断：上传期间引号被删掉的，不插入。属性链接是字符串的整个值（M6/P1 第 10 条），所以这与 M6 Codex 评审 R1 的链接补全是同一条规则；总体设计 13.2 新增第 28 条，以后自动写进正文的入口照做。测试：`asset-upload.test.ts` 四例（两种引号与 `Bob's.png`、正在写的 frontmatter；五种不插的落点，正文开头照插；一批两个、上传期间引号被删、U+FFFE 与 U+FFFF；拖入文件夹时文件夹那句仍在前面）；e2e AS2 新增一例：真实浏览器在 `image: ''` 的引号里粘贴 `Bob's.png`，保存之后正文是 `image: '![[Bob''s.png]]'`，属性接口 `valid`，三个属性、`ref` 指向 Guide 的属性链接都在，标签 `retained` 计 1 | 不按落点、直接插 `![[link]]`：单测与 AS2 的新例失败（AS2 停在 `image: '![[Bob's.png]]'`）；不转义、只认闭合的、不守第一行、不守闭合的那一行、只在闭合时守第一行、不滤 U+FFFE、不滤 U+FFFF、把不插的算成"正文已被替换"、文件夹那句不并上：各自的用例失败 |
| R2 | 取时间上界（D1）：`transfer.job_timeout` 的上限从 7 天改为 23 小时（`config.MaxJobTimeout`），比附件的孤儿清扫等的一天（asset 的 `SweepAge`）少一小时，与 River 的救援比任务多等一小时同一个口径；单元的 COMMIT 在任务期限之后至多再占 `database.commit_timeout`（少于 8 秒），在这一小时之内。导入先存下一批的文件、再在单元里写行，从存下到行提交都在它的任务之内，清扫因此删不到还会挂上的文件；`Attachments.Put` 的注释写明文件在 Attach 或 Drop 之前归调用方、在它的任务之内。`TestNoJobOutlastsTheOrphanSweep`（bootstrap）钉住 `MaxJobTimeout + 1 小时 ≤ SweepAge`，`asset.SweepAge` 从模块根导出给它；配置的校验测试与 `config.yaml` 的注释随之改。另一种做法是让清扫的年龄随 `job_timeout` 变长（像救援那样由组合根算出），要把它交进 asset 的 `Deps`；v0.1 单实例，默认的 6 小时已经够几 GB 的导入（M12 的压测移交核对），所以选了改动小的上限（负责人可改判）。原先合法的、超过 23 小时的配置从此启动失败（变更记录写明） | 上限改回 7 天：配置的两个测试与 `TestNoJobOutlastsTheOrphanSweep` 失败；上限改为 24 小时、清扫改为 23 小时：后者失败 |
| R3 | 导出的改名（`transfer/domain/plan.go` 的 `freeName`）按名称族记下一次试的序号：截短之后的名称（按标题键），与是否写文件。名称被占了就一直被占，所以前面的序号不必再试，结果与原先逐个从 2 试起相同。导入的撞名序号有同一个问题，Codex 没报：`page/app/unit_import.go` 原按名称键记，截短之后相同的不同名称各从 2 试起，一个单元里同样是平方。改为按 `domain.Family`（序号的位数写成 `#`，标题里没有这个字符）的键记，同一个键的各种写法与截短之后相同的名称共用。增长按分配次数比较，不计时（不受机器负载影响）：数量 4 倍，导出的分配 4.1 倍（原 18 倍），导入的 4.2 倍（原 17 倍），测试要求至多 6 倍；名称按序号依次取、截短随位数变短，逐个核对。规划阶段没有加取消的检查点：成本已与节点数成正比 | 不用记下的序号（导出、导入各一）、导出按未截短的名称记、导入按名称的键记：增长测试失败；记下的序号多跳一个：名称的用例失败；导出的名称族不分是否写文件：新例"文件跳过的序号仍是文件夹的"失败（一个文件名过长的页与一个撞名的文件夹截成同一个前缀，`s 2.md` 是别的文件夹的名称）；`Family` 的序号只写一个 `#`：`TestNumbered` 失败 |
| D1 | 见 R2：时间上界。配置的上限与清扫的年龄由一个测试连起来；总体设计 13.1 第 6 条写明文件从存下到行提交归谁、在多长时间之内，后面的 M（M8 的恢复、M10 的来源区）经附件存文件的，同样在一天之内写行 | — |
| D2 | 写进 [M10 的移交](../../M10-llm-wiki/handoffs/M7-transfer.md)第 6 项：注册第一个贡献者时定下它的输出有界（字节、文件数、生成时间），写明上限、由贡献者守住、测试钉住；真要很大的输出时，再加带上下文的流式写入与统一的额度。现在不加框架 | — |
| 6.1 | `TestAReportKeepsItsFirstProblems` 改为写死接口的 1,000：先加 1,000 条（第一条带长路径）不截断，再加一条截断，留下的是前 1,000 条、次序不变 | `MaxProblems` 改为 999、2000：都失败 |
