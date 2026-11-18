# M12 旁支 `m12-ingest-request-deadline`（M10 的单个 REST 改名与 4 秒的请求期限）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m12-ingest-request-deadline`，从 `main` 的 `5154e0de`（提交期限的旁支记合并号的提交）开始（主持者的决定 580）。只改测试与 `Makefile` 的注释，不改产品代码。<br>实施 1 个提交 `8fd158fd`：401 个来源的单个 REST 改名改在生产的 15 秒请求期限下跑，由 11 秒判整个计时的请求；期限由界推出、与起应用的期限对证；计时的改名用 30 秒的客户端。<br>修复第 1 轮 1 个提交，头 `a1b7b7e5`：删除组那一格也先推期限、再对证，界只写在一个变量里（IDA-M1）；推期限挪到起应用之前（IDA-N2）；注释照实写（IDB-M1、IDA-N1、IDC-N1、N2）。<br>到 `a1b7b7e5` 3 个文件（`server/internal/bootstrap/ingest_extra_cost_test.go`、`ingest_cost_test.go`、`Makefile`），加 77 行、减 15 行 |
| 审查方式 | 实施者在工作树 `nerve-wiki-m12-ingest-deadline` 里查因（`gh` 只读取 guards-sources 的全部作业与产物）、实施、跑反向对照。三位审查者并行，各在自己的分离的工作树里（A `nerve-wiki-m12-side-rev`，B `nerve-wiki-m12-p4-rev-a`，C `nerve-wiki-m12-p4-rev-c`），重的都经 `heavy.sh`，不碰实施者的工作树：<br>A 改后的测试：基线、一个直接调推期限帮手的探针、六个变异；15 秒下各项超时；计时的客户端与 `ask` 是否等价。<br>B 定因与同类：重取 2026-11-05 起 guards-sources 的作业与产物、guards-core 近 44 次；按机型核历史、S2 前后与失败那一作业；沿代码核"11 秒是死参数"；逐格列同类；读预算。<br>C 门禁、提交与文档：本机跑门禁四项与 `make test-go-guards-sources` 一次；逐句核提交说明；列文档的去处与 05-lessons 的写法。<br>修复核对第 1 轮三位挪到 `a1b7b7e5`，之后不开第二轮（决定 583） |
| 日期 | 2026-11-18（查因、实施、审查、修复一轮、修复核对一轮） |
| 结论 | Important 0。Minor 4：删除组那一格没有对证起应用的期限（IDA-M1）；11 秒原本不是这一格的界（IDB-M1）；9V45 的变慢不止一次（IDB-M2）；"S2 前后哪种 CPU 都没有台阶"说过了头（IDC-M1）。Nit 10（IDA-N1、N2，IDB-N1–N3，IDC-N1–N5），Q 4（IDA-Q1，IDB-Q1，IDC-Q1、Q2）；分开是 A 0I 1M 2N 1Q，B 0I 2M 3N 1Q，C 0I 1M 5N 2Q。三位都同意：定因站得住，不是产品变慢，这一格也从没贴近 4 秒；同类只有这一格；改法照决定 359。代码一侧由修复第 1 轮 1 个提交改，文档一侧在本提交。修复核对第 1 轮：A 0I 0M 0N，B 0I 0M 2N（IDB-N4、N5），C 0I 0M 2N（IDC-C1-N1 = IDB-N5，IDC-C1-N2）；第一轮的都关上，新 Nit 都在本记录里写对 |

## 起因

- **`main` 的 CI**：`5154e0de`（只改两份文档），run 44232764121 第一次 attempt，`server (guards-sources)`，job 134561542870，AMD EPYC 9V45，`CPUs: 4`。M10 的 `TestIngestOfMoreThanBatchSourcesTakesItsTime` 的 `single_rename_401_sources/0` 答 500：
  - INGESTCOST 的 `Metric.WallNS` 4.0056 秒，`CPUNS` 1.570 秒；
  - 只有 `InsertAliases` 的 `Errors=1`，结束在约 4.008 秒，即撞上测试配置的 4 秒请求期限（`ingestCostOrdinaryBudget`）；
  - 产物 `go-test-guards-sources` 的 `test-go.log` 第 41445–41446 行。
  - sample1、2 没有跑；make 停在这一行，后面的 `TestIngestOfDeletedSourceGroupsTakesItsTime` 也没有跑。
- **重跑**：主持者只重跑失败的作业。第二次 attempt 的 guards-sources（job 134569171570）落在 Xeon 6975P-C 上，改名三个样本 1.552–1.565 秒，五个导入守卫都过。所以只有一种红。
- **代码没动**：`5154e0de` 与上一次绿的 `main`（`b26a8dbd`）在 `server/`、`Makefile`、`.github` 上逐字相同；再往前到 `38916978`（11-17，9V45 上正常），产品代码也相同（审查 B）。
- **测试的来历**：这一格由 M10/P4/S2 的 `5d4607eb` 加进来，所以提交写 `(M10 story, found in M12)`。
- 主持者开旁支，照决定 359 的做法查因、修（决定 580）。

## 定因

### 核实的

- **历史**（实施者取 2026-11-05 起的全部 guards-sources 作业，审查 B 重取一遍；按作业与产物去重，有这一格样本的作业实施者数 250 个、B 数 247 个，差在去重与起止日期，不影响结论）。每格是每个作业三个样本里最慢的一个，不含失败那一作业：

  | 机型 | 作业数 | 范围 | 中位 | S2 之前的中位（个数） | S2 之后的中位（个数） |
  |---|---|---|---|---|---|
  | EPYC 7763 | 64 | 2.661–2.988 秒 | 2.823 | 2.823（43） | 2.823（21） |
  | EPYC 9V74 | 70 | 1.914–2.662 | 1.965 | 1.974（45） | 1.959（25） |
  | EPYC 9V45 | 31 | 1.352–1.620 | 1.487 | 1.492（20） | 1.475（11） |
  | Xeon 6975P-C | 49 | 1.537–1.641 | 1.583 | 1.570（1） | 1.583（48） |
  | Xeon 8573C | 18 | 2.007–2.406 | 2.299 | 2.291（13） | 2.308（5） |
  | Xeon 6973P-C | 7 | 1.806–2.091 | 1.920 | 1.846（5） | 1.948（2） |
  | Xeon 8370C | 6 | 2.358–2.568 | 2.405 | 2.428（4） | 2.405（2） |

  "S2 之前、之后"按样本里的 Go 版本分（go1.27.1 / go1.27.4）；按 `git merge-base --is-ancestor 4190bac0` 分，结论相同（审查 B）。
  - **不贴着 4 秒**：失败之外最慢的样本 2.988 秒（7763，run 43948816167），7763 上 64 次都在 4 秒的 67–75%。决定 359 的撤销一直只比 4 秒少 3–18%，这一格的余量大得多。
  - **S2 前后**：样本多的四种机型（7763、9V74、9V45、8573C）前后差不到 0.02 秒；6975P-C 之前只有 1 次，6973P-C 5 次与 2 次，8370C 4 次与 2 次，太少。看不出与代码有关的台阶。
  - 三个样本之间几乎不变：最慢的除以 sample0，中位 1.00，至多 1.05；没有决定 359 那种 sample2 随历史增长的格。
- **失败那一作业整份都慢**（对 9V45 上全部样本的中位）：

  | 格 | 失败那次 | 9V45 中位 | 倍数 |
  |---|---|---|---|
  | `maximum_source_page` | 0.847、0.827、0.687 秒 | 0.410 | 1.68–2.07 |
  | `mixed_maximum_bytes` | 19.146、17.739、17.373 | 9.128 | 1.90–2.10 |
  | `unreferenced_old_f` | 4.101、4.446、4.496 | 1.406 | 2.92–3.20 |
  | `single_rename_401_sources` | 4.006（被截） | 1.452 | ≥2.76 |
  | 作业前段的来源撤销 sample2 | 2.99–3.18 | 约 2.2–2.5（决定 359 记的 9V45） | 约 1.3 |

  - 跑了的四个导入格是 1.7–3.2 倍；第五格（删除组）没跑。作业里越往后越慢。
  - 其余 guards-sources 作业里，这四格的样本对本机型中位的最大倍数是 1.35–1.37：中位取各作业最慢的样本是 1.355，取全部样本是 1.371，都在 run 42688339794（9V74）。算上第五格是 1.38（`delete_q_s`，run 43919318998，9V45）。失败那一作业是 3.15–3.20（审查 B；核对第 1 轮 B 的 IDB-N5、C 的 IDC-C1-N1）。
  - Go 进程自己的 CPU 时间也多：`unreferenced_old_f` 1.89–2.38 秒对 9V45 中位 0.871，`mixed_maximum_bytes` 17.7–18.1 对 10.6，改名到截断时 1.57 对 0.88（审查 B）。
  - 改名的时间花在哪里（各语句的起止，对 9V45 上正常的一次 run 44077699226，审查 B）：改写 401 个来源 3.17 秒对 0.87–0.90 秒；读旧链接 0.50 秒对 0.15–0.17 秒；写索引做到第 42 页（0.26 秒，约 6.3 毫秒一页）就被截，正常 401 页 0.36–0.39 秒（约 0.96 毫秒一页）。前两段慢 3–3.6 倍，写索引每页慢约 6.6 倍；不是卡在某一条语句上。
- **9V45 的变慢不止一次**（审查 B 的 IDB-M2）：再前一次 `main`（run 44224495963，`f08dca69`，02:36:24Z 起）的 guards-core 也落在 9V45 上，也慢约 2 倍：`healthy_hundred_k` 1.32–1.60 秒，对 11-16 23:50 起另外 5 次 9V45 的 0.64–0.81 秒；`virtual-export-whole` 两个状态的中位 3.58、3.21 秒，对 1.71–2.23 秒。两次之间的那一次 `main`（run 44228054450，`b26a8dbd`，03:06:35Z）的 guards-core 在 6975P-C 上，`healthy_hundred_k` 0.75–0.77 秒，正常（核对第 1 轮 B 的 IDB-N4）。11-18 02:36–04:01 UTC 之间，9V45 上两个作业慢了 2–3 倍；同一份产品代码前一天在 9V45 上两份都正常（guards-sources run 44077699226 改名最慢 1.532 秒；guards-core run 44069983898 `healthy_hundred_k` 0.64–0.69 秒）。
- **结构上的毛病**（实施者与审查 B 沿代码核实）：
  - M10/P4 设计这一格时，由测试的 4 秒请求期限判：[04-P4](../../M10-llm-wiki/04-P4-virtual-ingest.md) 第 215 行"单move/rename超400来源页不冒称batch非法，以现有request和parser界验证/记录实际可达规模，不提高任何限制"；`5d4607eb` 的 `single` 一支写的是 `budget = ingestCostOrdinaryBudget`。
  - `6d1549fc` 把批一支的字面 `ms >= 11000` 改成参数 `unitBudget`，四个调用都传 11 秒；`single` 一支不读它，这一格的 11 秒从此是死参数。`02567298`（M10 决定 150）加 `requestBudget` 时仍给这一格 `ingestCostOrdinaryBudget`。
  - REST 改名不记单元：产品里 `unit_ms` 只在 `page/app/apply_batch.go:168`（"batch applied"）写。
  - 所以判这一格的一直是 4 秒的期限（界等于期限）；越过期限只答 500，记不下用时。

### 推断

- 那段时间的 9V45 慢了（同机的邻居、降频或宿主的 I/O），不是产品。依据：同一份产品代码前一天在 9V45 上正常，重跑在 6975P-C 上正常，Go 一侧同样的活也要 1.6–2.7 倍的 CPU 时间。是哪一种，日志里分不出。
- 照那一作业的速度，整次改名约 5–6.5 秒：照各段约 3 倍的速度，止于约 5.1 秒；照截断前实测的写索引速度（42 页 0.264 秒，约 6.3 毫秒一页，正常约 0.96 毫秒一页）外推 401 页，止于约 6.5 秒（审查 B 的 IDB-N2）。
- 照这次的倍数落在 7763 上，改名约 8.1–9.6 秒。

## 同类的清单

查的是"在测试的 4 秒请求期限下跑，守卫自己的界或判定高于 4 秒"，顺带看"客户端的超时先于界"（审查 B 第三节逐格核过，与实施者的结论相同）：

| 守卫 | 请求期限 | 自己的界 | 判 |
|---|---|---|---|
| `TestIngestOfMoreThanBatchSourcesTakesItsTime` | 改前 4 秒，改后 15 秒 | 改前只有 4 秒；改后 11 秒 | **就是这一格** |
| `TestIngestOfAHundredThousandPagesTakesItsTime` | 4 秒 | 4 秒（同一个常量） | 界等于期限，真等于。guards-core 上 7763 最慢 1.44 秒（实施者，2026-11-05 起）；近 44 次里 7763 的 5 次 1.21–1.27 秒，最慢一次 1.60 秒，是上面那台慢的 9V45（审查 B） |
| `ingest_mcp_preview_test.go:52` 一类 | 4 秒 | 4 秒 | 界等于期限 |
| `TestLintSourcesIndexCostsItsNormalWrites`、`TestLintRangeIndexCostsItsNormalWrites` | 4 秒 | 只有 4 秒 | 决定 359 有意留下 |
| `TestVirtualIndexOfAHundredThousandPagesTakesItsTime` | 4 秒 | 中位 1.87、0.07、2.59 秒 | 界在期限之下 |
| `TestVirtualExportOfAHundredThousandPagesTakesItsTime` 的 `virtual-export-whole` | 4 秒 | 中位 5.74 秒 | 不在此类：量的是"启动后台任务、轮询、下载"三段，启动答 202、活在后台任务里，下载走 `api.Stream`，处理器本身没有请求期限；没有一个请求承担 5.74 秒 |
| 另四个导入额外形状、`TestIngestAtTheBatchBoundaryTakesItsTime` | 15 或 30 秒 | 11 或 29 秒 | 界在期限之下 |
| 来源五项、lint 摘要、升级三格、护栏、grep、批的改写与撤销与恢复、后台 | 4/15、12、6/15、15、15、15 秒，或进程内 | 都在各自的期限之下，或不受请求期限 | 已修（决定 359、475）或本来就对 |

经 `client()`（5 秒）计时的只有在 4 秒下跑的几格，客户端比期限晚；在 15、30 秒下跑的都用 30 秒的客户端或进程内。没有第二格（审查 B）。

## 改法

只改测试与 `Makefile` 的注释，产品没动。

1. **实施** `8fd158fd`：
   - 这一格的请求期限从 `ingestCostOrdinaryBudget`（4 秒）改为 `ingestCostBatchBudget`（15 秒）：`ingestCostServerBudget` 设请求 15 秒、写 60 秒；读仍是测试配置的 5 秒（Go 的 `net/http` 在处理请求时清掉读期限，11.4 秒的请求不被截，审查 A 实测），提交 2 秒不变。
   - 11 秒判整个计时的请求（`single && took >= unitBudget`，信息带用时）：REST 改名没有单元的日志行，拿整个请求比，比单元的时间严，多含 HTTP 与契约核对。
   - `ingestAdditionalDeadline`：期限只由界推出，取 4、15、30 秒里比界大的最小一个（30 秒是 M10 决定 150 的两个稠密形状），再与形状给的期限对证；`ingestAdditionalTeam` 另答应用起时的期限，再对证一次。
   - 计时的改名改用 30 秒的客户端（`sendRequestBy`，契约照样核，计时的范围不变）：`client()` 5 秒就放弃，期限提到 15 秒之后，它会先于界来判。
2. **修复第 1 轮** `a1b7b7e5`（主持者的修复说明 `ingest-deadline/fix-1.md`）：
   - 删除组那一格写成 `request, bound := ingestCostBatchBudget, 11*time.Second`，先推期限、起来之后对证；wall 的两处判定用 `request`，`unit_ms` 的判定用 `bound`，不再写字面的 `11000`（IDA-M1）。
   - 两处都先推期限、再起应用；`started` 的对证留在起来之后（IDA-N2）。
   - 改名那一格的注释与 `Makefile` 的两行写清：原由测试的 4 秒请求判（M10/P4 设计），传进来的 11 秒自 `6d1549fc` 起没断言过，M12 起在 15 秒下由 11 秒判（IDB-M1）。`ingest_cost_test.go` 的注释写明两个稠密形状在 30 秒下（IDA-N1 = IDC-N2）；`Makefile` 上一行的"原请求/单元预算"改准（IDC-N1）。
3. 改后五个额外形状都由界推出期限：`maximum_source_page`、`unreferenced_old_f`、单个改名、删除组 15 秒对 11 秒，`mixed_maximum_bytes` 30 秒对 29 秒。样本、夹具与别的判定不变（负责人 2026-11-06 的话："调整测试用例量级或者时限即可"）。

## 审查的发现与处置

编号照审查者的报告；代码位置是 `8fd158fd` 的。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| IDA-M1 | Minor | 五个额外形状只有四个经推期限；删除组丢掉起应用的期限，界与期限在不同的字面里，改成 4 秒不察觉 | `a1b7b7e5`：删除组也推期限、对证，一个界变量；改成 4/30 秒在起应用之前红，应用不在 15 秒下起红在对证（决定 582）。核对 A：关上 |
| IDA-N1 = IDC-N2 | Nit | `ingest_cost_test.go:27-30` 把 batch 一概说成在 15 秒下，两个稠密形状在 30 秒下 | `a1b7b7e5` 改注释。核对 A、C：关上 |
| IDA-N2 | Nit | 推期限在起应用之后；说明的"fails before any request"不确切 | `a1b7b7e5` 挪到起应用之前；说明的一句在"更正"。核对 A：关上 |
| IDA-Q1 | Q | 两个正式的 batch 形状也没有推出、对证 | 不动，见决定 582 |
| IDB-M1 | Minor | 11 秒原本不是这一格的界：M10 设计由 4 秒的请求判，11 秒自 `6d1549fc` 起是死参数；旁支是把界从 4 秒换成借来的 11 秒，不是"界不变" | `a1b7b7e5` 改注释与 `Makefile`；决定 580、581 写"4 秒 → 11 秒"；说明在"更正"。核对 B：关上 |
| IDB-M2 | Minor | 再前一次 `main` 的 guards-core 也在 9V45 上、也慢约 2 倍；"250 次只此一次"说过了头 | 决定 580 与本记录照实写；"更正"。核对 B：关上（"上一次"写成"再前一次"，见 IDB-N4） |
| IDB-N1 | Nit | 倍数的下沿是 1.7，不是 1.8 | "更正"。核对 B：关上 |
| IDB-N2 | Nit | "约 5 秒"只是一种算法 | "更正"：约 5–6.5 秒，两种算法。核对 B：关上 |
| IDB-N3 = IDC-M1 | Nit / Minor | "S2 前后哪种 CPU 都没有台阶"说过了头 | "更正"：样本多的四种差不到 0.02 秒，少的写出个数，"看不出与代码有关的台阶"。核对 B、C：关上 |
| IDB-Q1 | Q | 这一格的界取 11 秒还是约 6 秒 | 取 11 秒，并列约 6 秒一格（决定 581） |
| IDC-N1 | Nit | `Makefile` 前一行的"原请求/单元预算"对这一格不再成立 | `a1b7b7e5` 改。核对 C：关上 |
| IDC-N3 | Nit | 说明的"deadlines are worked out … and checked"不含删除组 | 随 IDA-M1 关上；`8fd158fd` 那一句在"更正"。核对 C：关上 |
| IDC-N4 | Nit | "every ingest shape"多算了没跑的第五格 | "更正"。核对 C：关上 |
| IDC-N5 | Nit | "1.37 倍"没说量的是什么 | "更正"。核对 C：关上（数见 IDB-N5） |
| IDC-Q1 | Q | 13.4 第 8 条要不要说 batch 的形状一向在 15 秒下 | 采纳，加半句（决定 583） |
| IDC-Q2 | Q | 第 12 节放哪一行、记录叫什么 | 放 P4 一行；本记录名（决定 583） |
| IDB-N4 | Nit（核对第 1 轮） | `a1b7b7e5` 说明的"the run before on main"：44224495963 是再前一次，前一次是 44228054450，正常 | 本记录与决定写"再前一次 `main`"；"更正" |
| IDB-N5 = IDC-C1-N1 | Nit（核对第 1 轮） | `a1b7b7e5` 说明的"1.35–1.38"：只算那四格是 1.35–1.37，1.38 来自第五格 `delete_q_s` | 本记录写对；"更正" |
| IDC-C1-N2 | Nit（核对第 1 轮） | `Makefile:617` 的引文省了"/记录实际可达规模"半句，没标省略号 | 不改代码，接受；本记录的定因引的是全文 |

## 主持者的决定

负责人可以改判；正文在 [00-M12](../00-M12-design.md) 第 11 节"2026-11-18：导入守卫的旁支的决定"。

- **580 只重跑失败的项；开旁支，照决定 359 的做法，单个 REST 改名改在生产的 15 秒期限下跑，界从 4 秒换成 11 秒、由它判整个请求；产品不改**；照常审查、合并（同决定 356、359、541、548、564）。
- **581 这一格的界取 11 秒**（IDB-Q1），并列约 6 秒一格与它的代价。
- **582 删除组也由界推出期限、与起应用的期限对证；两个正式的 batch 形状不动**（IDA-M1、IDA-Q1）。
- **583 文档的去处与收口**：13.4 第 8 条的例外加这一格并加半句（IDC-Q1）；第 12 节放 P4 一行，记录用本名（IDC-Q2）；05-lessons 第 1 节 359 那一条下加两个子条；修复核对第 1 轮之后不开第二轮；已推的提交说明不改写，不准处写在"更正"。

## 实施中定的

各条主持者接受，不另编号。

1. 期限取 4、15、30 秒里比界大的最小一个；30 秒那一档盖住决定 150 的两个稠密形状。没有形状的界低于 4 秒，4 秒那一档今天走不到（审查 A 的 m4 存活，照决定 359 的规则留着，不算毛病）。
2. REST 改名没有单元的日志行，拿整个计时的请求比 11 秒。
3. 读超时留测试配置的 5 秒，与同一文件里本来就在 15、30 秒下跑的 batch 形状一致；不照抄决定 359 的"读 30 秒"。
4. 十万页那一格（界等于期限）与 lint 的两格不动。
5. 删除组 wall 的判定也改用同一个 `request` 变量（修复第 1 轮）。

## 接受与推后的

- **接受**：
  - 这一格 4–11 秒之间的变慢现在照绿。本机睡 3 秒，改前 4.0 秒答 500，改后 4.58–4.69 秒照过；本机要多出约 9.4 秒才在界上红。11 秒对 7763 最慢的 2.99 秒留 3.7 倍，从前的 4 秒留 1.34 倍；11 + `parse_max_wait` 的 2 = 13 秒，仍在生产的 15 秒之下。
  - batch 的单元过界时不打计划：打计划的条件只看 err、wall 与 `isError`，`unit_ms` 过界在后面直接 `Fatal`。M10 起就这样，不是本支引入的；单个改名的新断言反而补了这一支（核对第 1 轮 A，不编号）。
  - `Makefile:617` 的引文省了半句（IDC-C1-N2）。
- **推后**：两个正式的 batch 形状（`old_f_120000` 15/11，`maximum_old_f_hundred_k_100_ops` 30/29）没有推期限、对证；以后改这两个形状时一并接上（决定 582）。
- **未核实**：
  - 9V45 那一段变慢的原因（邻居、降频、宿主的 I/O）；
  - 截断前 42 页的写索引速度能不能代表 401 页；
  - 审查 B 的死参数探针（把这一格的界改成 1 ms，预期在 `5154e0de` 上照绿）排不上 `heavy.sh` 的锁、没跑，结论靠读代码；
  - guards-core 的"离期限远"只在 11-16 22:31 到 11-18 03:46 那一段上核过（审查 B）。

## 更正

已推送的说明不改写（决定 583）。

**`8fd158fd` 的说明**（逐条也写在 `a1b7b7e5` 说明的"Corrections to 8fd158fd's message"，下面照核对第 1 轮改准）：

- "the unchanged 11 s bound"：这一格的界是从 4 秒（它的请求期限）换成 11 秒，借自同规模的批形状，不是"不变"（IDB-M1）。
- "no other of the 250 jobs was over 1.37 times its CPU's median"：说过了头。再前一次 `main`（run 44224495963，`f08dca69`）的 guards-core 02:36 UTC 起也在 9V45 上、也慢约 2 倍；11-18 02:36–04:01 UTC 之间 9V45 上两个作业慢 2–3 倍（IDB-M2）。
- "1.37 times its CPU's median"：量的是其余 guards-sources 作业里，这四格的样本对本机型中位的最大倍数，1.35–1.37（IDC-N5、IDB-N5）。
- "every ingest shape took 1.8-3.2 times"：是跑了的四个导入格，1.7–3.2 倍；第五格没跑（IDB-N1、IDC-N4）。
- "about 5 s"：约 5–6.5 秒，两种算法见"定因"（IDB-N2）。
- "no step across S2 on any CPU"：样本多的四种机型前后差不到 0.02 秒，少的几种个数太少；看不出与代码有关的台阶（IDB-N3、IDC-M1）。
- "Giving the shape 4 or 30 s, or starting its app at 4 s, fails before any request"：在 `8fd158fd` 上三种都红在起应用、注册账号之后；`a1b7b7e5` 起，给形状 4 或 30 秒红在起应用之前，应用在 4 秒下起红在起来之后、计时的请求之前（IDA-N2）。
- "The shapes' deadlines are worked out from their bounds … and checked against the one each app started under"：在 `8fd158fd` 上只对五个里的四个成立，删除组是 `a1b7b7e5` 接上的（IDC-N3）。

**`a1b7b7e5` 的说明**：

- "the run before on main (44224495963, f08dca69)"：是再前一次；前一次是 run 44228054450（`b26a8dbd`），它的 guards-core 在 6975P-C 上，正常（IDB-N4）。
- "1.35-1.38 by how the median is taken"：只算"those four shapes"是 1.35–1.37；1.38 来自第五格 `delete_q_s`（IDB-N5 = IDC-C1-N1）。

## 反向对照与变异

都在本机 arm64，经 `heavy.sh`；变异从备份复原。命令是 `Makefile` 那一行的 `go test -count=1 -v -run '^Test…$' ./internal/bootstrap`。先在没改的树上跑同样的命令。

**实施时**（"改前"是 `git archive 5154e0de` 的副本，测试配置的 4 秒；"改后"是 `8fd158fd`，15 秒；拖慢是在 `RenameNode.Execute` 的开头睡）：

| 变异 | 改前 | 改后 |
|---|---|---|
| 不改（基线） | 绿，1.61–1.68 秒 | 绿，1.59–1.69 秒 |
| 睡 3 秒 | 红：4.001 秒答 500 | 绿，4.58–4.69 秒 |
| 睡 4 秒 | — | 绿，5.62–5.74 秒 |
| 睡 10.5 秒 | — | 红在界上："single REST rename took 12.238212672s, at or above its 11s bound (request deadline 15s)" |
| 去掉新的断言，再睡 10.5 秒 | — | 绿，12.20–12.23 秒：这条断言是唯一判它的 |
| 计时的客户端换回 `client()`，再睡 4 秒 | — | 红在 `Client.Timeout exceeded while awaiting headers` |
| 形状给 4 秒、给 30 秒 | — | 红在推期限："bound 11s; request deadline 4s, want 15s, the smallest above the bound" |
| 钩子不调 `ingestCostServerBudget` | — | 红："the app started under a 4s request deadline, want 15s" |

**审查 A**（`8fd158fd`）：基线 1.572/1.585/1.577 秒；探针直接调 `ingestAdditionalDeadline`，四个形状推出 15/30/15/15 秒，与原来的期限相同，边界与四个错的组合都对；m1 单元回调里睡 9.8 秒，红在界上（11.39 秒），失败时照旧打出计划；m2 再去掉新断言，绿（11.34–11.41 秒）；m3 推期限的表去掉 15 秒一档，红；m4 去掉 4 秒一档，存活（没有形状用到，见"实施中定的"第 1 条）；m5 换回 5 秒的客户端、睡 4.5 秒，红在 `Client.Timeout`；m6 `started` 不赋值，红。

**修复第 1 轮**（`a1b7b7e5`）：

| 变异 | 结果 |
|---|---|
| 不改：改名、删除组 | 绿，1.594–1.690 秒；0.236–0.249 秒 |
| 删除组给 4 秒、给 30 秒 | 红在推期限，测试 0.00 秒，在起应用之前 |
| 删除组的应用不在 15 秒下起 | 红在对证，1.27 秒，起来之后 |
| 删除组 `unit_ms` 的判定（`:329` 那一处）换成 1 ms | 红，读到 batch 的 `unit_ms`，判定确实走到了 |
| 改名给 4 秒、给 30 秒 | 红在推期限，0.00 秒（`8fd158fd` 上同样的错要 1.45 秒才红） |

**核对第 1 轮 A**（`a1b7b7e5`，删除组）：基线 0.256/0.261/0.251 秒；d1 期限改 4 秒，红在 `:276`，0.00 秒；d2 在 `apply_batch.go` 量单元时睡 11.5 秒，红在 `:330`"deleted group exceeded its 11s unit bound"，`unit_ms` 11750，wall 11.757 秒，没到 15 秒；d3 同样的拖慢，再把 `:329` 的 `bound` 换成 `request`，三个样本 11.74–11.77 秒都照过。可见在 11 与 15 秒之间，判的只有界。

## 核对

**修复核对第 1 轮**（2026-11-18，三位挪到 `a1b7b7e5`）：

- **A**：IDA-M1、N1、N2 关上，IDA-Q1 照决定不动；跑了 c1-base、d1–d3（见上）；没有新的发现。顺带的一条（batch 的单元过界不打计划）记在"接受与推后的"。
- **B**：IDB-M1、M2、N1、N2、N3 关上；重算了 1.37 倍（`rb_check1.py`）；新 Nit 2 条（IDB-N4、N5），都是第一轮的写法被照抄进了更正，本记录写对。
- **C**：IDC-M1、N1–N5 关上，IDC-Q1、Q2 采纳；`a1b7b7e5` 上 build、vet、`make lint-go` 都是 rc 0；新 Nit 2 条（IDC-C1-N1 = IDB-N5，IDC-C1-N2）。C 只读 B 的数据重算时，经 B 的脚本重写过一次 `~/.cache/m12-ingest-review-b/rows.json`，输入与代码相同，内容应不变。

没有 Important、Minor，不开第二轮（决定 583）。

## 门禁与 CI

- **门禁**（本机 arm64，经 `heavy.sh`，连着的几项写成一个脚本经一次 `heavy.sh` 跑完，每项之前判 `df`）：

  | 头 | 谁 | 结果 |
  |---|---|---|
  | `8fd158fd` | 实施者 | `go build ./...`、`go vet ./...` rc=0；`make lint-go` 两处 `0 issues`、`go mod tidy -diff` 两处干净；`make test-go-short` 94 个包 `ok`（51 秒）；`make test-go-guards-sources` 一次 rc=0（493 秒，11 个顶层测试都过；改名 1.56–1.62 秒在 15 秒下，来源的新建、移动、遗忘仍在 4 秒下） |
  | `8fd158fd` | 审查 C | 同上四项 rc=0（`make test-go-short` 48 秒）；`make test-go-guards-sources` 457 秒，11 个都过；改名 1.569–1.594 秒 |
  | `a1b7b7e5` | 实施者 | 四项 rc=0（`make test-go-short` 49 秒）；改到的两个顶层测试各一次，都绿 |
  | `a1b7b7e5` | 核对第 1 轮 C | build、vet、`make lint-go` rc=0 |

  golangci-lint 是主克隆 `bin/` 里的 2.14.0，没有下载。顶层测试的名字没改，分份不动。
- **分支的 CI**（普通推送，没有改写已推的历史）：

  | 推送 | 头 | `ci.yml` | 结果 |
  |---|---|---|---|
  | 1 | `8fd158fd` | 44242224718 | 19 项第一次 attempt 全绿：guards-sources 在 Xeon 6975P-C 上（作业 11:50，Test 11:29），改名 1.570–1.624 秒，期限 15 秒；`race` 14:15，八份组合根的 Test 4:14–5:57，`e2e` 8:03（E2E 一步 6:43），`image` 1:15。另两份工作流（Release、Vulnerabilities）也绿 |
  | 2 | `a1b7b7e5` | 44254273872 | 第一次 attempt 18 项绿：guards-sources 在 Xeon 6975P-C 上（Test 11:09），改名 1.529–1.555 秒，删除组 0.305–0.318 秒；`race` 15:11，八份组合根的 Test 4:28–6:29。`e2e`（job 134639978308）卡在"Install the Playwright browsers"（07:10:17 起），到 20 分钟的作业时限被取消，E2E 一步跳过；本支只改组合根的测试，`8fd158fd` 上 e2e 是绿的。主持者只重跑 e2e：第二次 attempt 为绿（e2e 6:59：装浏览器 0:38，E2E 5:54） |

- **本提交自己的分支 CI**：（待填：本提交的分支 CI）

## 环境

实施在工作树 `nerve-wiki-m12-ingest-deadline`；审查与核对各在分离的工作树里（A `nerve-wiki-m12-side-rev`，B `nerve-wiki-m12-p4-rev-a`，C `nerve-wiki-m12-p4-rev-c`），不碰实施者的工作树。重的都经 `heavy.sh`，每项之前判 `df`（34–37G）；变异由脚本改文件、从备份复原；审完各工作树干净，没有 `*.mutbak`。`TMPDIR`、`GOTMPDIR` 与日志在 `~/.cache/m12-ingest-*`：实施者与审查 C 的用完已删；审查 B 取下来的历史与产物（约 12 MB）是引用的证据，留在 `~/.cache/m12-ingest-review-b/`，主持者用完可以删。容器由 Ryuk 收走。本机 arm64（Parallels 虚机）。实施者与审查 B、C 经 `gh` 只读 CI 的日志、产物与历史；e2e 的重跑由主持者做。

要留的在设计工作目录的 `m12/ingest-deadline/`（`review-*.md`、`check1-*.md`、`fix-1.md`、`host-facts*.md`、`commit1.txt`、`commit2.txt`）；实施报告与修复报告没有落成文件，在主持者的会话里。
