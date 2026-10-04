# M4–M5：Codex 代码质量评审

日期：2026-10-04。评审基线：`1000506f1151bce6020dd368f6dc122a6174b943`（`1000506`）。M4 范围 `b2e7c91..4db58bb`，M5 范围 `4db58bb..1000506`；合计 **139 个提交、606 个文件、60,454 行新增、750 行删除**，包含文档与生成物。下文源码、测试与设计文档的行号一律指这个基线。

## 1. 结论

**常规门禁全部通过，但仍有三条可以让用户已经输入的正文消失的生命周期路径；建议先修复草稿保护、并发读取的错误转换与 Markdown 成本问题，再把 M4、M5 当作满足现有约定的实现交给 M6。** M6 的设计可以继续，本轮没有发现需要推翻写入单元、编辑锁或事件扩展接口的证据。

正式发现 **7 项：Critical 3、Important 2、Minor 2、Nit 0**。其中实现问题 5 项、测试缺口 2 项。Critical 按本次评审规定的“用户写的内容丢失”分级；这里的丢失是尚未持久化的草稿随界面卸载消失，并非数据库已经保存的正文被删除。没有证实角色判定错误、跨资源信息泄露、持久化数据损坏或死锁。

最值得关注的三件事：

1. **“退出前保存”和“失去访问后正文可复制”没有覆盖整个前端生命周期。** 当前标签页注销时的一次保存没有冻结编辑器；另一标签页注销绕过本页保存；被移出工作区时外层布局直接销毁编辑器。三条路径都取得了浏览器输入、实际请求和正文／版本表的证据（R1–R3）。
2. **静态不变量不能代替多条读取语句之间的时序。** 页面及正文在一次删除事务中同步软删，仍不能保证先读节点、后读正文元数据的请求不撞上删除。`getPage` 在这个窗口答 500，正文和阅读视图端点却正确答 404（R4）。
3. **现有测试的边界仍有空洞。** 长十进制 frontmatter 数值的语义处理正确，成本却越过现有相对判据（R5）；把浏览器生命周期监听接到错误目标、把 20 页退化边界改错，所执行的现有测试仍通过（R6、R7）。这些分别影响 M6 的渲染、推送与批量改写基础。

## 2. 评审范围与方法

### 2.1 范围、分工与历史去重

按后端写入／锁／清理、Markdown／任务项、前端／浏览器、事件服务端四条线核对。主审执行全量门禁、权限矩阵、事件变异及通知探针，并独立复跑三类草稿丢失、读取／删除交错和 Markdown 小变体／计量结果。

阅读范围包括 `docs/README.md`、总体设计指定章节及第 13 节、M4/M5 总设计、十二个 Phase、对应 plans、设计／Phase／收尾审查、收到的十份 handoff、M4 的输入法人工清单，以及要求的 M6–M12 下游移交；M4/M5 的 specs 目录没有实质规格文件。报告结构参照 M3 的 Codex 评审。文档按 M5 最终设计解释，不把 M4 的旧 60 秒租约、旧会话并行模型、尚待后续里程碑注册的扩展当作当前缺陷。

权限核对沿用规则表、`bootstrap/permission_matrix*_test.go` 与 E2E 接口版本，检查状态码和 problem 码。Markdown 后续补测按用户澄清仅用已有 fixture 的小变体；中断前已取得的计量结果保留，同尺寸复测没有扩大输入类别或尺寸。所有操作只涉及本机归档快照和自行启动的测试容器，没有测试线上服务或其他项目。

R1 与 M4/P6 C1、MJ-1／M5 收尾 B-M4、MN-1 区分；R2 与 M5/P4 其他标签锁滞留的接受项、收尾 Q-1 区分；R3 与“工作区被删不保留”的接受项区分；R4 重新检验 M4/P1 `CONTENT-META-DELETED` 的等价判断；R5 与 P3 的别名预算、C5 数值语义修复及 M12 性能移交区分。具体理由见各发现。负责人确认的接管、删除拒绝、120 秒租约、30 分钟闲置退出，以及没有内容损失实证的体验事项不重新计数。

### 2.2 隔离、门禁与计数

原仓库始终在 `main`，开始时工作区干净。五份 `git archive 1000506` 快照位于 `/tmp/nwiki-m4m5-review.W31ekd/{main,backend,markdown,frontend,events}/repo`，均安装冻结依赖并初始化 snapshot Git 提交。构建、探针和变异都在快照中完成；未运行 `make image-smoke`。

| 命令／验证 | 实测结果 |
| --- | --- |
| `pnpm install --frozen-lockfile` | 五份快照均 exit 0 |
| `GOFLAGS=-p=3 make check` | exit 0；Go 主模块 **58 个有测试包、16 个无测试包**，含 `-race -count=1`；加上单独成本检查与 tools，共 **62 条成功包执行记录**；Vitest **107 文件、1605/1605**；两处 Go lint 均 0 issues；oxlint、格式、类型、knip、前端构建通过；fixture 检查 **67 例＋4 个 rename 例** |
| `GOFLAGS=-p=3 make gen-check` | exit 0，生成物没有差异 |
| `GOFLAGS=-p=3 make build` | exit 0，随后用该产物运行浏览器测试 |
| `NWIKI_E2E_VERSION=0.1.0-dev pnpm exec playwright test --workers=4` | **182/182**，38.4 秒 |
| 同上，`stories/page stories/collab --repeat-each=3 --workers=4` | **65 个原有用例 × 3＝195/195**，1.0 分钟；本轮未重现原有 E2E 偶发失败 |
| 权限矩阵及树／笔记本／流可见集合对照，`-race -count=1 -p 3 -json` | **5 个顶层、592 个子测试 PASS**；其中矩阵 **588 格**，页面／节点／编辑会话／编辑锁／任务操作 **221 格**；另有矩阵 prepare 1 项与集合对照 3 项 |
| page 模块独立基线，`go test -race -count=1 -p 1 -json ./internal/modules/page/...` | **7 个有测试包、2 个无测试包；146 顶层＋182 子测试 PASS** |
| 既有关键锁交错重复，`-race -count=3 -p 3 -json` | **8 组 × 3＝24 顶层、42 子测试 PASS**：接管／保存、解锁／保存、心跳／结束、删除／开启、正文写／开启、两个开启及两种迟到心跳 |
| 新读取／删除探针，`-race -count=3 -tags review` | 三个端点各三轮：`getPage` **3 次实际 500**，正确行为断言失败；正文与阅读视图 **6 次 404**。主审独立再跑三轮，结果相同 |
| Markdown 定向竞态基线 | **6 包通过**，包括现有随机任务项差分、fixture、清洗与预算测试；相对成本测试在 race 下按设计跳过，非 race 成本门禁由 `make check` 另跑 |
| Markdown fixture 小变体 | **3 个 fixture × 32＝96 个变体、640 次 Flip、736 次 HTML 规格检查通过**；主审独立重跑相同结果 |
| Markdown 计量 | 中断前 **6 类 × 8 个尺寸＝48 次**；随后已有 128/512 KiB 十进制及约 512 KiB 普通文档各三次，共 **9 次同尺寸重复**；主审再独立重复这 9 次。计量探针通过不表示相对成本合格，见 R5 |
| 新浏览器探针 | **9/9**，13.7 秒；`--repeat-each=3 --workers=3` **27/27**，15.2 秒；主审独立 **9/9**，13.7 秒。包含 4 个内容丢失场景、4 个合成生命周期场景及 1 个中文任务／快捷键场景 |
| 新 pages 退化边界探针 | **0、1、19、20、21 页**，基线三轮 **3 顶层＋15 子测试 PASS**；改错边界后只有 20 页失败 |
| 新 PostgreSQL 通知探针，`-race -count=3 -p 3` | **3/3**：回滚无通知，同一事务相同载荷合并、不同载荷保留，下一事务相同载荷再次送达 |

Go 包并发上限为 3；后端独立线采用更低的 `-p 1`，避免同时启动过多数据库。浏览器最多 4 workers。上表测试集重叠，重复次数、父子 PASS 事件和已有用例不能相加成独立业务用例数。新增探针共 **15 个顶层测试单元**：浏览器 9、读取／删除 1、Markdown 3、事件边界 1、通知 1；表格循环的 96 个变体等另列，不冒充顶层测试。

变异共 **23 类**：事件 10、后端 6、前端 5、Markdown 2；**20 类被所执行的既有测试检出，3 类存活**。其中一类事件变异因缺帧导致测试失败并在清理时触发 90 秒总超时，不能描述成全部都正常快速结束。三项存活的具体测试范围见第 6 节，未把定向测试通过夸大为全仓库门禁通过。

### 2.3 证据口径、工具与复原

工具为 Go testing／race detector、PostgreSQL `18.6-trixie` Testcontainers、pnpm／Vitest、Playwright Chromium、HTTP／SQL 断言、确定性交错握手、Python 变异脚本和 Git。没有修改基线生产逻辑来制造正式实现发现。

R1–R3 的浏览器探针**断言基线确实丢失草稿**，因此绿色表示重现成功；R4 按正确的 404 断言，退出 1 是反例证据。R1 的 `holdContentWrites` 暂停真实 PUT 的发送，再以 `route.continue()` 放行，未伪造成功答复。R1/R2 暂停页面时钟固定合法先后，不能据此推断现实发生概率或窗口长度。R3 另有不暂停时钟、不使用输入法模拟的普通连续输入对照。

原始日志、探针、diff、重放脚本保留在上述外部目录。主审门禁在 `main/{check,gen-check,build,e2e,page-collab-repeat}.log`，独立复核在 `main/{matrix,lock-repeat,read-delete-recheck}.jsonl`、`main/{browser-recheck,markdown-recheck}.log`；三条分工各有 `evidence.md`。浏览器探针为 `frontend/probes/frontend-probes.spec.ts`，读取探针为 `backend/probes/review_probe_test.go`，Markdown 为 `markdown/review_{probe,spec}_test.go`，事件探针为 `events/review_boundary_test.go` 与 `events/probes/review_notify_test.go`。

探索中有两类不能算产品问题的失败：CDP 的实际冻结请求在本机 headless Chromium 中没有触发 `freeze`，后来明确改用合成 document 事件检查接线；新增工作区移除探针最初把预期 404 控制台条数写死，受请求先后影响，后来仅声明实际收到的精确 404 文本，其他普通控制台错误仍由项目监视器检查。正文、版本及编辑器消失的断言独立保留。`make check` 还打印了 9 条 Node `TimeoutNaNWarning`，测试仍通过；它们不是浏览器页面控制台，也没有在本轮完成归因，不称整个执行过程“零警告”。

收尾时所有变异恢复，临时测试移到快照外，五份快照 `git status --porcelain` 均为空。按本次日志识别的 40 个自有容器 ID 均已不存在，未发现仍在运行的本次快照进程；没有按名称批量结束进程或处理其他会话容器。原仓库只新增本报告，未触碰 `.claude/`、其他项目或用户笔记库。

## 3. 发现清单

| 编号 | 级别 | 一句话描述 | 类别 | 归属 |
| --- | --- | --- | --- | --- |
| R1 | Critical | 注销保存进行中仍接受输入，随后结束编辑，后输入的正文没有保存也没有保留 | 实现 | M5 |
| R2 | Critical | 另一标签页注销让正在编辑的本页直接换代，未保存正文消失 | 实现 | M5 |
| R3 | Critical | 被移出工作区时外层布局销毁编辑器，未按设计保留可复制草稿 | 实现 | M5 |
| R4 | Important | 读取节点之后提交删除，`getPage` 的正文元数据 NotFound 泄漏为 500 | 实现 | M4 |
| R5 | Important | 长十进制 frontmatter 数值在已测尺寸超过累计分配与增长耗时的相对预算 | 实现 | M4 |
| R6 | Minor | freeze/resume 监听目标改错，全部 Vitest 和既有 C9 仍通过 | 测试缺口 | M5 |
| R7 | Minor | 20 页退化边界从 `>` 改成 `>=`，事件模块与扩大的相关组合测试仍通过 | 测试缺口 | M5 |

## 4. 发现详情

### R1 — 注销保存期间输入的正文被丢弃

**位置。** `web/apps/web/src/pages/page/page-edit.tsx:261` 把注销接到 `save(true)`；`web/apps/web/src/stores/page-editing.ts:268–275` 只等一次保存或期限，然后无条件 `end()`；`web/apps/web/src/stores/root.store.ts:153–157` 通过该路径结束本页编辑。正常 Done／闲置退出的 `page-edit.tsx:171–192` 会先 `hold(true)`，保存后检查 `editing.unsaved`；注销路径缺少这两步。

**场景与复现。** 正文初始为 `Saved.\n`，进入编辑并输入 `FIRST`；从用户菜单点 Sign out，暂扣随之发出的正文 PUT。点击仍可写的编辑器，继续输入 `-DURING-LOGOUT`，可见正文包含完整的新文本。放行 PUT 后保存成功并注销，正文及版本只有初始内容和 `Saved.\nFIRST`，后输入的部分没有可复制的界面，也没有历史版本。

将归档的浏览器探针放入已安装、已构建的同基线快照 `e2e/stories/review/frontend-probes.spec.ts`，在该快照 e2e 目录运行：

```sh
NWIKI_E2E_VERSION=0.1.0-dev pnpm exec playwright test stories/review/frontend-probes.spec.ts --grep 'typing while logout save' --workers=1
```

```text
EVIDENCE logout in-flight: editor accepted FIRST-DURING-LOGOUT, database only FIRST
```

探针通过 API 回读当前正文，直接以 SQL 检查按 revision 排序的全部 `page_revisions`。探针重复三轮、主审独立复跑都重现。时钟暂停固定“第一次保存已取文本→继续输入→保存成功→注销”的顺序；它证明可达逻辑，不测现实延迟概率。本例没有保存失败或超时，因此不能仅用既定的 1.5 秒尽力保存解释。

**历史与影响。** M4/P6 C1、MJ-1 已处理 Done 保存期间继续输入的问题；M5 收尾 B-M4 为注销补上的保存绕过了那条受保护的离开路径，MN-1 处理的期限也不能捕获这份新草稿。用户看到输入已进入编辑器，却在普通注销动作后失去它，按本次内容丢失标准列 Critical。

**建议。** 注销开始保存时也冻结编辑输入，并在结束前核对当前草稿已处理；或明确排空保存队列。补“保存进行中继续输入”的注销回归，不能只断言曾经发过一次 PUT。失败／超时的产品选择另见 D1。

### R2 — 另一标签页注销直接销毁本页未保存正文

**位置。** `web/apps/web/src/stores/auth.store.ts:43–46` 只调用发起注销标签页的 `beforeSignOut`；`stores/root.store.ts:156` 只遍历本页 edits；`app/session-root.tsx:18–23` 随 loginId 重挂 provider。`pages/page/unsaved-guard.tsx:31–45` 拦截导航和 `beforeunload`，不能拦截 provider 换代。

**场景与复现。** 同一浏览器、同一登录的两个标签页：A 编辑 `Saved.\n`，输入 `UNSAVED-OTHER-TAB`，状态为 Unsaved changes；B 在工作区首页点 Sign out。A 随认证换代显示 Sign in，编辑器消失，没有观察到 A 的正文 PUT 答复；API 回读的正文和 SQL 检查的全部版本仍只有 `Saved.\n`。

```sh
NWIKI_E2E_VERSION=0.1.0-dev pnpm exec playwright test stories/review/frontend-probes.spec.ts --grep 'second-tab logout' --workers=1
```

```text
EVIDENCE cross-tab logout: no PUT, no draft in editor or revisions, persisted content Saved.\n
```

日志中的 `no PUT` 应严格解释为“没有观察到 PUT 答复”：夹具 `contentWrites()` 监听 response，不能单凭它证明从未发送请求。暂停 A 的页面时钟固定正常的“最后输入到两秒自动保存之前”窗口，没有修改登录分代、网络或数据库实现。分工重复三轮与主审独立运行结果相同；不声称每次注销必然丢字，也不把暂停后的等待时长当现实窗口。

**历史与影响。** M5/P4 接受的是另标签锁最多滞留 120 秒；收尾 Q-1 明说另标签换代不经 `endEdits`，讨论的是组合等待，并没有实现草稿保留。B-M4 的注销前保存仅保护发起页。这里证明普通、已确认的输入确实消失，超出了不重报体验事项的范围。与 R1 分列是因为这条路径根本没有调用本页保存，修复位置也不同。

**建议。** 全浏览器主动注销前协调各标签的未保存编辑；需要立即注销时，至少为原账户保留有明确退出／复制出口的本地草稿。认证换代照常生效，不能沿用已撤销凭证写服务端，也不能把草稿暴露给随后登录的另一账户。补“B 注销、A 正在编辑”的浏览器与版本断言。

### R3 — 被移出工作区时，未保存编辑被外层布局卸载

**位置与规格。** `web/apps/web/src/pages/workspace/workspace-layout.tsx:85–86` 在工作区不再可见时直接返回 404；`app/event-stream.tsx:63–76` 重连后由外向内重新取数，触发这个卸载。页面与笔记本外壳的草稿保留无法越过它。M5 总设计 `00-M5-design.md:210–218` 要求失锁时正文只读且可复制，并明确把“被移出笔记本或工作区”列为失去访问；不保留的例外写的是“工作区被删”。

**复现一：普通输入。** 编辑者靠工作区开放角色进入编辑，以每键 100ms 连续输入 `ORDINARY-CONFIRMED-CONTINUOUS-DRAFT`，超过三秒且没有两秒停顿。管理员通过真实接口移除此工作区成员。编辑者立即看到 Page not found，编辑器、草稿和离开提示均不存在，没有观察到正文 PUT 答复，API 回读的正文及 SQL 检查的全部版本只含初始 `Saved.\n`。这个场景不暂停时钟，不模拟输入法，工作区和页面本身也没有被删除。

**复现二：组合等待。** 先输入已确认的 `CONFIRMED-DRAFT`，再用 CDP 开始 `nihao` 组合并推进页面时钟十秒；自动保存按设计等待组合结束。移出成员后，组合前已经确认的文字也随编辑器消失。因此结论不依赖“半截拼音应该被保存”的假设。

```sh
NWIKI_E2E_VERSION=0.1.0-dev pnpm exec playwright test stories/review/frontend-probes.spec.ts --grep 'workspace removal' --workers=1
```

```text
EVIDENCE workspace removal: confirmed draft + 10 seconds active composition lost; no retained editor and only original revision
EVIDENCE workspace removal: over 3 seconds continuous normal typing lost, zero PUT, zero leave confirmation, no draft in revisions
```

两种情形各三轮重现，主审另独立验证。日志中的 `zero PUT` 同 R2，仅指未观察到 PUT 答复；初版日志写作“3.5 seconds”是固定说明，并非计时结果，本报告只采用输入步骤可支持的“超过三秒”。移除后正常出现的精确 404 控制台文本单独声明，未吞掉任意普通控制台异常；数据丢失由 DOM、API 当前正文与版本表共同断言。

**历史与影响。** M4/P6 Q2 的后续保护已覆盖页面／笔记本外壳，M5 收尾 C-I6 的 C8 覆盖阅读状态的成员移除；两者都没有守住编辑中的工作区外壳。M5/P4 接受的工作区删除不是本次成员移除。其“自动保存后至多约两秒”的理由也不能概括 debounce：持续输入与组合等待可使已确认草稿长期未保存。本例直接违反失权后的本地正文保留设计。

**建议。** 工作区外壳也识别其未保存编辑，或把草稿出口提升到会被移除的外壳之外。失权后立即停止服务端读写，但保留已经在本地的只读文字与复制出口。只添加路由离开确认不足以处理原地址上的数据刷新卸载。

### R4 — 页面读取与删除交错时答 500

**位置。** `server/internal/modules/page/app/get_page.go:44–62` 先找节点和判定可读，`:27–32` 再调用 `viewOf`；`app/view.go:37–39` 把 `ContentMeta` 的仓储错误原样返回。`adapter/postgres/queries/contents.sql:6–9` 排除已删正文，`adapter/postgres/store.go:149–154` 将找不到行映射为仓储 NotFound；这里没有转换为领域的 `page.not_found`。相邻 `app/get_page_content.go:35–42`、`get_page_view.go:44–52` 已正确使用 `found(err, domain.ErrNotFound)`。

**确定性交错。** GET 已读到未删节点，并完成读取判定；在判定返回前用 channel 暂停。另一普通 DELETE 经 HTTP 处理器、写入单元和真实 PostgreSQL 事务删除这页，返回 204，并确认节点与正文同时软删。继续 GET，随后 `ContentMeta` 查询返回 NotFound，最终得到 500。

探针只包装模块测试现有 Authorizer，在成功判定后暂停，不改权限结果、仓储、隔离级别或 HTTP 响应。多条独立查询之间可以自然发生该调度。它验证的是读／删并发，不替代真实 access 权限矩阵。相同交错同时测正文和阅读视图，作为正确的 404 对照。

在外部快照复制 `backend/probes/review_probe_test.go` 到 `server/internal/modules/page/`，从快照的 server 目录运行：

```sh
go test -race -count=3 -p 3 -tags review -run '^TestReviewReadDeletionInterleaving$' -v ./internal/modules/page
```

```text
DELETE node = 204; concurrent GET page = 500 {"status":500,"code":"internal_error","title":"Internal Server Error"}
want 404 page.not_found after concurrent deletion
DELETE node = 204; concurrent GET page/content = 404 ... "code":"page.not_found"
DELETE node = 204; concurrent GET page/view = 404 ... "code":"page.not_found"
```

分工三轮、主审三轮结果一致，无数据竞态报告。退出 1 是正确行为断言捕获了缺陷。

**历史与影响。** M4/P2 T5 修的是写事务等锁之后的 NotFound，P3 的 `S5-VIEW-CONTENT-ERR-RAW` 守的是 `/view`，均不是当前路径。P1 把 `CONTENT-META-DELETED` 视为等价，理由是未删页面只有未删正文；这只描述同一时刻的不变量，不能排除两个查询之间提交删除。本次直接证明基线会把正常生命周期竞态答成服务故障。没有证实正文损坏，也没有把未实际运行的其他删除入口扩张进结论。

**建议。** 在正文元数据错误处同样转换领域 NotFound，并补 GET 与删除的确定性交错。是否将整个读模型放入一致快照另行决定，修复本例不要求改变整个读架构。

### R5 — 长十进制 frontmatter 数值超过现有相对成本判据

**位置与预算。** `server/internal/platform/markdown/yaml.go:271–280`，尤其 `:275`，先以 `new(big.Int).SetString(digits, base)` 解析任意长整数，再转 float64，发现无穷才保留原文。`markdowntest/inputs.go:159–178` 的 frontmatter 成本样本没有长十进制数。`markdowntest/costs.go:109` 要求累计分配不超过同尺寸普通文档 14 倍，`:116` 要求四倍输入的耗时不超过四分之一尺寸的 8 倍加 1ms；总体设计 4.3、13.3 与 M4/P3 3.10 将这些作为相对门禁。

**样本与复测。** 合法 frontmatter `---\na: ` 加 N 个 `9`，再接 `\n---\nbody`，与既有大整数语义测试同属纯十进制类别，但不是复制其原字符串。中断前已经测得 4–512 KiB 的结果；本轮保留证据，只对已有的 `N=128<<10`、`N=512<<10` 样本（各另加 16 bytes）和 `markdowntest.Normal(512<<10)` 各重复三次，没有扩大到最大正文或并发 HTTP。

复制 `markdown/review_spec_test.go` 到外部快照 `server/internal/platform/markdown/`，在 server 目录运行：

```sh
go test -p 3 -tags review -run '^TestReviewRepeatExistingSamples$' -count=1 -v ./internal/platform/markdown
```

主审独立复核的原始数字：

| 样本 | 实际输入 bytes | 三次 Parse＋Render 耗时 ms | 三次 TotalAlloc 差值 bytes |
| --- | ---: | --- | --- |
| 十进制 128 KiB | 131,088 | 12.940 / 12.746 / 12.923 | 41,939,296 / 41,936,224 / 41,936,256 |
| 十进制 512 KiB | 524,304 | 150.598 / 151.435 / 151.830 | 619,749,816 / 619,744,688 / 619,750,008 |
| 普通约 512 KiB | 524,619 | 33.693 / 30.779 / 28.352 | 39,775,512 / 39,782,416 / 39,657,408 |

每次先 GC，计时覆盖 Parse＋Render，取三次最快。四倍输入的最快耗时比为 **11.81**；`8 × 12.746417 + 1 = 102.971336ms`，小于实测 `150.597791ms`。用数值样本最少分配除以普通样本最多分配，仍为 **15.58 倍**，超过 14。分工先前独立重复得到 11.49 倍耗时增长、至少 15.57 倍分配，方向一致。本次 CheckHTML 均通过，既有大整数保留原文的语义回归也通过；计量探针未另断言完整属性值逐字相等。没有超过“普通文档十倍耗时”那条门槛。

**严格口径。** TotalAlloc 是一次操作期间的累计分配，不是峰值活堆或 RSS，不能写成同时占用 620MB。与原生 `CheckCosts` 相比，Parse＋Render 的计时范围和输入尺寸一致，字符串转字节都在计时之外；但原生分配轮在三次计时之后另跑且不显式 GC，本探针每轮先 GC 再同时计时／计分配，热身次序也不同。探针没有把样本加入 `Pathological()` 后执行修改过的原生 `CheckCosts`，因此不声称原生门禁失败；严格结论是已测条件下超过相同数值预算，独立重复仍观察到超线性增长。没有 CPU／heap profile 或局部替换的因果隔离，源码显示昂贵的大整数转换路径；没有测 5 MiB、并发负载、OOM 或 HTTP 拒绝，不能外推这些结果。

**历史与影响。** P3 的别名展开预算针对重复展开，本例只有一个标量；C5 修复的是超浮点整数输出 Inf，既有测试使用 1 后接 400 个 0 的十进制值／键，以及 300 个 F 的十六进制值，没有守住更长数值转换的成本。M12 移交接受普通大文档内存和部署成本，不等于放弃现有相对预算。本例证明该预算样本集合有遗漏，会延续到 M6 共用的 Parse。

**建议。** 在昂贵转换前做线性范围判定，明显超出目标数值范围时按既有语义保留原文；处理符号和前导零，不能仅按原字符串总长度拒绝。将该已有反例纳入成本样本，并补范围附近、前导零及键／值的语义回归。

### R6 — 真实浏览器 document 生命周期接线缺少回归

**位置。** `web/apps/web/src/events/deps.ts:48–51` 当前正确地区分 window 的 pagehide/pageshow 与 document 的其他事件；`events/hub.test.ts:302–318` 通过 `FakePage.fire` 直接送事件；`e2e/stories/collab/c9-one-stream.spec.ts:80` 的接手触发是关闭持有者，无法证明 document freeze/resume 接线。

**变异与实证。** 仅把 freeze/resume 的监听目标从 document 改为 window，其他选举逻辑保持不变。完整前端测试 **107 文件、1605/1605** 通过，变异产物的既有 C9 两种选举也 **2/2** 通过。新增探针在真实浏览器执行 `document.dispatchEvent(new Event('freeze'))`，要求另一标签接手，再以 resume 返回并核对正文刷新；Web Locks 与 localStorage 租约两项均失败：

```text
Test Files 107 passed (107)
Tests 1605 passed (1605)
2 failed ... synthetic document freeze ... Web Locks / lease
2 passed
Expected: > 1
Received:   1
```

基线同两个探针通过；恢复后全部 9 个浏览器场景各重复三次，27/27 通过，其中这两个 freeze 场景各三次，主审独立运行也通过。可重放变异 diff 为 `frontend/mutations/freeze-target-window.diff`，日志为 `freeze-target-window.log`、`freeze-target-window-e2e.log`；先在外部快照应用该 diff、放入归档探针，再执行：

```sh
pnpm --filter @nervewiki/web test
make build
cd e2e
NWIKI_E2E_VERSION=0.1.0-dev pnpm exec playwright test stories/collab/c9-one-stream.spec.ts stories/review/frontend-probes.spec.ts --grep 'C9|synthetic document freeze' --workers=2
```

**影响与建议。** 这是测试缺口，基线接线正确。Fake 生命周期证明状态机，关闭标签证明关闭路径，两者不能发现监听目标改错。补经过 `browserPageLifecycle` 的 document 事件用例即可守住这层；合成事件不等于验证操作系统实际冻结，真实后台冻结仍需另验。本次没有把 CDP 未发事件算作实现问题，也不声称覆盖 Safari／Firefox。

### R7 — 恰好 20 页的事件退化边界没有被现有测试守住

**位置。** `server/internal/modules/events/app/publisher.go:60` 当前为 `len(data.Pages) > domain.MaxPages`；`app/publisher_test.go:57–92` 测了小集合及 `MaxPages+1`，没有恰好 20 页。13.1 第 30 条和 M5 的约定是“多于 20 页为 null”。

**变异与扩大验证。** 将 `>` 改成 `>=`，全部 events 模块测试通过；随后扩大到 events 与 bootstrap 的流、可见性、删除、页面写入、会话事件等相关测试，**43 顶层＋110 子测试 PASS**，仍没有捕获。扩大的完整选择表达式保留在 `events/recheck_boundary.py`；不是只凭一个窄单测的通过判缺口。

新增 `TestReviewPagesBoundary` 分别生成 0、1、19、20、21 个正文改动，并检查解码后的 `pages` 列表／null。基线重复三轮全部通过；同一变异下只有 20 页失败：

```text
count=20 emitted=0 null=true tree=true
count 20 must remain listed, got []
```

```sh
# 在外部快照中加入归档的 review_boundary_test.go 后
go test -race -count=3 -p 3 -run '^TestReviewPagesBoundary$' -v ./internal/modules/events/app
```

`events/pages-20-off-by-one.diff`、`boundary-expanded.jsonl`、`boundary-mutant.jsonl` 分别保留改动和两次对照。基线并没有提前退化；错误变异会让恰好 20 页的事件失去精确列表、改走整体刷新，不能将它描述成基线丢事件或正文损坏。

**建议。** 在现有 Publisher 测试补 19／20／21 的精确内容断言，保留独立的 7999 字节载荷上限测试。M6 的一个单元多页改写会使这个边界更常用，交接前补齐成本低。

## 5. 设计层面的疑问

### D1 — 有界注销的失败结果如何与草稿保留共存？

`stores/page-editing.ts:268–275` 明确选择保存失败或达到期限也结束编辑，`root.store.ts:153–157` 限制整个注销等待。避免注销无限等待有合理目的，但“已经尝试保存”不能表达正文已安全处理。R1/R2 已证明两条成功／跨标签路径的问题；本轮没有独立执行网络失败、冲突或超时的注销场景，不把这些额外风险计入发现。

建议作者明确：失败时是否提供仍与原账户绑定的本地复制出口、是否询问放弃、期限后正在执行的写如何反馈。任何方案都应保留认证换代与账户隔离。这里是对已接受尽力保存边界的设计问题，不是要求取消期限或以旧凭证继续写。

## 6. 测试缺口与变异结果

### 6.1 三项存活变异

| 改动 | 实际运行的现有测试 | 反向对照与结论 |
| --- | --- | --- |
| freeze/resume 监听 document 改 window | 全部 Vitest 1605 项＋C9 两项 | 新浏览器 document 事件两项均失败，基线通过；列 R6 |
| `pages` 的 `> 20` 改 `>= 20` | events 全模块；再扩大至相关 bootstrap，共 43 顶层／110 子测试 | 新边界测试只有 20 页失败，基线 5 格全过；列 R7 |
| `ContentMeta` 生成 SQL 去掉 `deleted_at IS NULL` | page 全模块 7 个有测试包，146 顶层＋182 子测试，全部通过 | M4/P1 旧等价理由未覆盖多语句读的删除窗口；另以未变异基线的真实交错证明 R4。没有运行该 SQL 变异的 E2E，也没有证明该变异本身一定更坏，不将其另计实现缺陷 |

### 6.2 被检出的二十项

| 分组 | 改动 | 捕获证据 |
| --- | --- | --- |
| 页面／锁，5 项 | 去掉客户端种类核对；正文锁按账户而非会话；删除只查子树第一项锁；解锁不清过期会话；参与者追加绕过守卫 | 现有 page/app 对应错误码、锁及参与者测试失败；没有编译失败冒充检出 |
| 前端，4 项 | close 不保存；自动保存 2 秒改 3 秒；hub 丢弃后续类型；租约忽略 storage 抢占 | 分别有 2、1、2、2 个现有 Vitest 失败 |
| Markdown，2 项 | 任务偏移减一；checkbox 输出改 radio | 现有 fixture 偏移及完整 HTML 断言失败；变异前后恢复运行均通过 |
| 事件，9 项 | 丢 tree 标志；丢 session id；忽略 access.reached；移除笔记本过滤；移除工作区过滤；初始化队列误用小缓冲；7999 字节上限偏一；禁用凭证到期；取消心跳重认证 | 现有 events 模块测试失败；心跳重认证变异出现缺帧失败后清理等待，最终触发 90 秒测试超时，其余不是这种退出 |

这些变异逐项独立执行、逐项恢复。它们证明已运行断言确实依赖这些逻辑，不证明所有回归都会被抓住。R1–R5 的新增反例也应转为断言正确行为的正式回归；当前用于记录缺陷的绿色浏览器探针不能原样当作验收测试。

## 7. 核实过、没有发现新问题的部分

### 7.1 权限、契约与客户端

规则表与笔记本级矩阵相符，矩阵通过真实组合根、HTTP 和 PostgreSQL 逐格检查状态及错误码。覆盖 M4 的 12 个页面操作及 M5 的读锁、解锁、任务项，心跳／结束另有别人会话的目标；事件流按凭证与可见集合过滤，不把它伪装成一个资源角色判定。矩阵覆盖检查与“列表／树／流等于逐项允许读取”的测试通过，相关接口 E2E 也通过。

核对时保留了规格中容易混淆的差别：看不到资源答 404、看得到但不能做答 403；**请求体**里不可用的编辑会话答 409 `page.edit_session_ended`，**地址**里不可用的会话答 404 `page.edit_session_not_found`。不能为了“一律 404”误改已经明列的 409 规则。墓碑原因在所有权／客户端／页面检查之后返回，心跳先判笔记本权限；现有领域构造路径只给 `page.locked` 添加 `lock`、给 `page.edit_session_unlocked` 添加 `ended_by`，成员序列化与声明码的 adapter 契约测试通过。平台没有按错误码强制禁止其他构造附加这些成员，不能据此声称测试会自动拦下任何错误组合。

PAT 与登录会话共用规则，写入的客户端分别为 api／web；本人另一客户端也不能凭账户绕过正文的会话锁。删除按账户、正文按会话的区别由现有测试守住，反向改动被检出。已有删除、停用、过期、墓碑、跨目标的应用及契约用例通过；本轮没有穷举所有这些状态与每个角色的笛卡尔积，不能把矩阵成绩表述成这种穷举。

### 7.2 写入单元、锁、软删除与注册者

核对 `page/app/unit*.go`、SQL、模块根扩展测试及 bootstrap 交错：工作区共享锁先于笔记本锁和判定；树写从一开始取笔记本排他模式，正文／开启／解锁取共享模式，再到正文行与会话行，不在单元中升级。参与者经同一单元和守卫追加，不递归调用参与者；观察者一次收到合并结果，空操作不发事件；多注册者按顺序、首错停止。相关 page 基线与完整 Go 门禁通过。

M4 的 30–44、M5 的 45–51 既有交错随全门禁执行；选出的八组另重复三次，通过真实锁等待握手。其中六组 bootstrap 交错在结束时检查无环、深度、兄弟标题唯一、每页正文、revision／版本、软删跟随与活会话数量，另两组模块级迟到心跳交错检查相关会话 SQL 和结果。租约边界、墓碑不复活、开启／解锁先清过期行、迟到心跳找不到已删行等现有测试通过。这里不声称穷举接管、心跳、删除等所有两两排列；新增读取／删除反例见 R4。

删除子树、笔记本普通删除／无主删除／工作区删除的页面注册者，以及页面活动来源、会话结束订阅者均有真实组合与数据库行为测试，随门禁通过。清理从叶到根，跳过持锁行，保留仍被跟随行需要的父节点；page 的 `TestPurgeSkipsAHeldNodeAndKeepsItsAncestors`、`TestPurgeKeepsWhatAHeldRowNeeds`、`TestPurgeTakesADeletedSubtree` 通过。过期墓碑随过期会话清理；未新增实际清理积压负载。

### 7.3 事件流与事务通知

现有 `TestEveryPageWriteReachesTheStreams`、`TestEverySessionChangeReachesTheStreams`、`TestEveryVisibilityChangeResetsItsStreams`、`TestEveryNotebookDeletionResetsItsStreams` 通过，覆盖页面写、本人结束／接管／解锁／删除、可见性变化的接口和 CLI 路径。hub 的笔记本／工作区过滤、初始化时登记后计算可见集合及排队、慢流缓冲满时最后以 reset 结束都有现有测试；删除过滤或 access.reached 处理的变异会失败。

到期以及心跳重认证后的撤销／注销／停用处理，Listener 重连、安静连接探活与断线重置，长连接解除请求／写期限但保留读期限、停机取消 context，均由现有测试随 Go 门禁验证。`API.LongLived` 接入平台中间件，未发现模块重写认证限流次序的情况。**`/readyz` 不等待 Listener；Listener 尚未生效时是 `/api/v0/events` 答 503 `not_ready` 并带 Retry-After**，这两处不能混为一谈。启动与关闭次序按 `bootstrap/app.go` 核对，现有平台停机测试通过；本轮未新做进程信号或反向代理演练。

新增真实 PostgreSQL 探针在一次事务发送 `same,same,next`，下一事务再发 `same`，另外一事务发送后回滚；三轮收到的都恰为 `same,next,same`，没有回滚通知或多余通知。它验证同事务相同载荷去重与提交边界，不测全局通知队列锁的吞吐。pages 的 0／1／19／20／21 边界基线正确；7999 字节边界与 tree/session 信息的错误变异被现有测试捕获。

### 7.4 Markdown、任务项与浏览器交互

已有 Markdown 测试涵盖 CommonMark／goldmark 对照、清洗标签和属性、地址协议、标题 id、代码、脚注、frontmatter、HTML 大小、解析预算适配器及任务项随机差分。新增小变体来自 `062-tasks`、`065-tasks-nested`、`067-tasks-footnotes-headings`，组合 BOM、CRLF、短 frontmatter、NFD 前缀、外层引用；期望位置从 fixture JSON 独立做字节映射，未拿被测实现生成期望。96 个变体的提取位置和 data-task 相符；640 次 Flip 均只改目标一个字节、保持长度及 UTF-8，并重新核对全部位置／状态；736 次 CheckHTML 通过。没有发现新输出规格错误。R5 单列成本样本遗漏。

真实 Chromium 的全部页面／多人编辑故事及三轮重复通过，包含保存／冲突的保留与放弃、自动保存、CDP 组合、快捷键、离开提醒、闲置退出、接管／解锁后只读、不悄悄重开、任务项、树写与跳转等已有验收。新增中文探针验证复选框名称“完成审查”、Space 后同项保留焦点、Mod+E 进入编辑、中文正文 Mod+S 真实保存、回阅读后 Edit 焦点。

一个浏览器一条流在 Web Locks 和 localStorage 租约两种方式下，已有十标签故事通过；新增合成 pagehide/pageshow、document freeze/resume 的四个场景均能交接、接收真实 PAT 正文更新、恢复后读到最新内容。未知后续类型经 hub、BroadcastChannel 到处理表的路径有测试，丢弃该类型的变异失败。R6 只指出具体浏览器监听目标没有被原有测试覆盖。

页面／PAT 版本的共同数据库断言按调用点核对：PG2/PG3 的改名／移动，PG5、PG7–PG9 的正文／版本／会话，C1–C4 的活会话／墓碑／删除，C10 的 `expectContentWritten` 与 `expectToggleRevision`。C5/C9 的浏览器专属行为、C6 的生命周期以及 C7 不可见事件的差异有设计例外，未发现新的对等性缺口。

### 7.5 指定长期约定逐条映射

下表“符合”限于读过的实现与本轮已运行测试；出现例外的条目明确指向发现。代码路径均相对基线，`page`、`events` 指服务端相应模块，`src` 指 `web/apps/web/src`。

| 条文 | 核对位置与结论 |
| --- | --- |
| 13.1-1 | `page/app/unit.go`、`unit_session.go`、`toggle_task.go`：单元、无操作、会话例外与事件边界符合；参与者绕守卫变异被检出 |
| 13.1-3 | `access/domain/rules.go`、`permission_matrix*_test.go`、三种可见集合对照：本轮矩阵状态码／错误码一致 |
| 13.1-4 | `page/app/authorize.go`、各会话用例及矩阵：常规码顺序和请求体／地址区别符合；读取／删除错误转换遗漏见 R4 |
| 13.1-5 | `page/app/unit*.go`、正文／会话 SQL、`interleavings_*_test.go`：所测锁序成立，8 组重复通过；不构成全部调度无死锁证明 |
| 13.1-6 | page 删除／清理 SQL、`bootstrap/registrants.go`、清理测试：同步软删、叶到根、跳锁、墓碑处理符合 |
| 13.1-8 | `page/adapter/http`、`httpserver/apierrors.go`：领域构造、lock/ended_by 成员序列化及声明码的契约测试通过；不等于平台强制限制所有码／成员组合 |
| 13.1-11 | 模块根、`bootstrap/events.go`／`events_registrants.go`／`page_names.go`、架构测试：窄端口、类型别名、组合转换形状符合 |
| 13.1-14 | `httpserver/longlived.go` 及测试、page BodyLimits：认证期限、流期限例外、逐帧写期限及请求体处理符合所测路径 |
| 13.1-15 | `config/validate.go`、session 常量、事件 hello、前端 autosave／idle 常量：配置交叉约束及两端常量有门禁；2 秒改 3 秒变异失败 |
| 13.1-21 | page 扩展与 registrants、bootstrap 事件／删除／任务测试：已注册的最后一跳存在；后续空扩展的行为仍由相应 handoff 要求补测 |
| 13.1-23 | `bootstrap/app.go`、`postgres/listener.go`、page River cleanup、平台 jobs：注册与次序符合，过期墓碑可清理；未测长时间积压 |
| 13.1-29 | `page` EditLock、writersSession、sessions SQL、锁交错：按会话写／按账户删、120 秒活性、墓碑不复活符合所测场景 |
| 13.1-30 | `events/app/publisher.go`、hub、postgres.Notify、组合根与事件测试：提交／过滤／reset／后续类型符合；精确 20 页的测试缺口见 R7 |
| 13.2-1 | `src/app/session-root.tsx`、RootStore、树写队列、PageEditing：分代和串行基础符合；注销草稿保护不完整见 R1/R2 |
| 13.2-6 | `.oxlintrc.json`、services/api、event.service：客户端依赖方向、problem／Retry-After 保留与类型／lint 门禁符合 |
| 13.2-7 | retry／NotLoaded、PageEditing、events/handlers：加载、有限重试、阅读键、失锁后停止写的既有测试通过；外层卸载例外见 R3 |
| 13.2-9 | `src/test/setup.ts`、`e2e/fixtures/browser.ts`／test：普通页和另页统一监视；普通控制台错误／警告按声明核对，事件流 4xx 单列 eventStreamErrors、由相关故事断言；Node 测试运行警告另述 |
| 13.2-15 | RootStore 的 pagesOf／editPage／events：按资源／按代缓存、编辑独立、每代一个 hub 有测试且通过 |
| 13.2-16 | 页面／笔记本／工作区 layout、event-stream 外向内刷新：页面与笔记本保留成立；成员移除时工作区外壳遗漏见 R3 |
| 13.2-17 | page-tree-item、reading-view、失锁横幅、焦点测试：已测名称与焦点符合；没有做实际读屏朗读 |
| 13.2-20 | shortcuts、whenComposed、page-edit 及 PG9/C5：对话框优先、组合等待、Mod+S/E 符合；R3 不是组合保存本身错误 |
| 13.2-21 | unsaved-guard：导航／beforeunload 有测试；R1–R3 的卸载不经过它；“至多约两秒”不适用于持续输入 |
| 13.2-23 | main 注入三张表、reading/enhancement、editor/extensions、events/handlers：顺序与清理／错误隔离、未来类型转发有测试 |
| 13.3-1 | 编辑器 line-breaks、往返测试与 PG9：保留未编辑的 BOM／换行字节；M5 任务项 Flip 是明确新增的单字节编辑，640 次变体反转符合所测位置 |
| 13.3-2 | Parse、fixture 与应用注册：服务端同一树提取／渲染，67 个现有 fixture 及 96 个小变体通过；本轮不访问 Obsidian |
| 13.3-3 | page 解析适配器、Parsed、写入单元与阅读视图：判定后、事务前解析并取预算，事务内不再解析；参与者自带结果且不占解析预算，观察者不得保留 Parsed，Fetch 不加锁，阅读不缓存 HTML。当前实现及测试核对通过，后续注册者仍须遵守；未新增 HTTP 并发预算探针 |
| 13.3-4 | 原始 HTML 逐节点清洗，渲染器／扩展标记单独登记；SafeURL、UTF-8 data-task、CheckHTML／CheckSize 与当前允许元素／属性的检查通过，后续扩展仍须登记规格 |
| 13.3-5 | 扩展的成本、节点渲染函数、Markup／SafeURL 与库依赖约束经代码及门禁核对；加固后不使用分隔符表，IsInLinkLabel 恒假、扩展链接不参与原有嵌套判断的限制已移交。新增计量比较见 R5，不把 race 通过当成本证据。未来 linking 若在 platform/markdown 之外使用 goldmark 须先改架构规则并补最后一跳 |
| 13.4-3 | 构建产物、受监视浏览器、页面／PAT 共用数据库断言：182 全量＋195 次故事重复通过，新增探针另列 |
| 13.4-4 | 真实 PostgreSQL 锁握手及 checkPages/checkNotebooks：现有交错执行并部分重复，另补读／删反例 |
| 13.4-5 | contract_test 与 apitest 的 long-lived 分支、事件 settle：契约派生测试和流头检查通过，非自由文本 5xx 的新反例见 R4 |
| 13.4-6 | 矩阵行、列、豁免与覆盖反例：所有契约操作覆盖机制存在，本轮 588 格通过；不替代生命周期组合测试 |

### 7.6 扩展点与 M6–M12 交接

M6 的链接改写应使用同一 `Writer.Run`／Appender，按节点 id 顺序写多页正文，任何守卫拒绝让整个单元回滚；观察者只处理合并后的结果。M7 导入、M8 还原／撤销、M9 batch 与 MCP 客户端沿用这些约束，当前模块根测试证明扩展形状，不能当作未来真实注册者的验收。会话结束订阅者已经持会话行锁，不得反向锁正文或节点；这条 handoff 约束与当前锁序一致。

Markdown、阅读增强和编辑器扩展已经有注入位置和测试；M6 新语法须同时登记 HTML 规格、字节位置及成本样本。事件的服务端 Publisher、未知类型转发和前端 `eventHandlers` 已贯通替身类型；增加工作区级类型时，需要按 M6/M10/M11 移交补上访客加入／恢复、新建工作区等目前不发 access 的增长路径，新 SWR 键也要进入重连刷新集合。它们是明确的后续义务，本轮未把尚未注册的功能记为 M5 缺陷。

M12 的普通大文档内存、代理部署、体验与真实输入法清单仍有保留价值；R5 针对已定相对判据的额外遗漏，R1–R3 针对实际内容丢失，不能由这些移交一并豁免。修复这些问题并把新增反例转为正式回归后，现有扩展结构可以继续作为 M6 的基础。

## 8. 未能验证的部分

- 浏览器只实跑本机 Chromium。真实中文输入法人工清单、Safari／Firefox、操作系统真实冻结／恢复和读屏器实际朗读未验证；合成 document 事件只证明监听接线与交接逻辑。
- 页面／多人编辑 65 个既有用例各重复三次未失败，不能证明 CI 不再偶发失败。代码中与心跳／控制台计数有关的时序疑点没有取得现有故事失败实证，未列发现。新增探针初版的计数问题已单独披露。
- Markdown 没有扩大到 1／5 MiB 新样本或做新并发 HTTP 负载；没有峰值内存和 profile。解析预算的 503 行为有现有适配器／应用测试随门禁通过，本轮没有另做服务级并发排队探针。没有新增懒惰续行专门变体，该项依赖已有测试。
- 接管／解锁／心跳／保存／开启／删除没有穷举全部两两排列；锁重复测试不构成无死锁证明。新增确定性交错只直接证实单页删除与三个读取端点的结果。
- PostgreSQL 通知探针没有重测全局队列锁的实现或多实例吞吐；没有重新验证 Caddy／nginx 代理、长时间断网或真实进程信号的完整停机序列。
- 没有做大工作区、长期清理积压或大批量页面的负载试验；未来 M6–M12 注册者尚不能在当前版本实测。没有对任何用户开发容器或其他项目施加负载。
- 注销失败／冲突／超时的额外草稿行为未独立运行，留 D1；不能从 R1 的成功保存交错直接推断那些分支。
- 本轮只提交报告，未在原仓库修复实现、补测试或改其他文档。临时证据保留在本机外部目录，不随报告提交；作者后续需将采纳的反例变成永久回归。
