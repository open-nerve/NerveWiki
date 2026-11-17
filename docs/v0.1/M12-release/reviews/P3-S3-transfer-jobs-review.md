# M12/P3 S3（导入导出的任务）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m12-p3-s3`。实施的 4 个提交，从 `main` 的 `2d32d346` 开始，审查时在 `a48ace20`：<br>T4 启动时的收拾 `62e1416e`：五种任务设 `jobs.Job.DiscardRunning`，`Rescue.AtStart` 在 `InterruptJobs` 之后照 `domain.Kinds()` 逐种 `lost`；River 适配器、模块与 `TestTheRescue` 的测试；`riverHoldsSQL`、`riverTakenSQL` 与 3.14.3 的六处种子；`TestServeFailsTheJobsLeftRunning` 的三种排队行。<br>T2 按人的上限 `9aea99ec`：`MaxPersonJobs = 5`、`ErrTooManyJobs`（409 `transfer.too_many_jobs`，没有 `Retry-After`），`CountActive` 一条语句交回全部与此人的，`Uploads` 记人，`room` 先按人、再全局，回滚以零 UUID 进 `room`（算进去、从不因此被拒）；契约四处、`make gen`；网页的码、两种语言的文字、`offeredOf`、假服务端；`TestAPersonsSixthJobIsRefusedAndAnothersStarts`。<br>T3 名称的编码 `292b238f`：`adapter/archive/names.go`（`unicodePath`、`readNames`），`Entries` 以读出的名字判文件夹与 `Special`；domain 的 `NameSource`、`RawEntry.From`、`Sorted.Read`，报告的 `name_unicode_path`、`name_gbk`，`importRun` 在 `finish` 之前加进报告；契约的枚举与说明；网页的报告文字；适配器的表（13 格）、差分（13 个计数各带下限）、模糊；`TestAnImportReadsNamesNotInUTF8`。<br>决定 482 `a48ace20`：`transfer.revertLeftGroup` 去掉后半句"不要先撤销这次撤销"，两种语言，`transfer-background.test.tsx` 的两处断言。<br>修复第 1 轮 7 个只改测试的提交 `65315a27`–`cbda8435`（"审查的发现与处置"）；合并 `main` 一次 `7dfb248a` |
| 规格 | [P3 设计](../03-P3-server-correctness.md) 3.11–3.20、4.2、6.3、7.3 的 S3、7.5、第 8 节第 5、6、12、13 条与"S3 另"；2.3 的决定 361–377（P3-1 到 P3-17）；[00 号文档](../00-M12-design.md) 4.6.6、决定 336、373、374、376、377、482 |
| 审查方式 | 三位审查者（Opus）并行，各在自己的分离的工作树里（挪到 `a48ace20`），重的经 `heavy.sh`，不碰实施者的工作树；`TMPDIR`、`GOTMPDIR` 在 `~/.cache` 下各自的目录，用完删了：<br>A T4 与决定 482：启动的次序（每个 `Job` 的丢弃在它的 `Start` 之前，全部在 River 起来之前）、取到又开始了的与只取到没开始的分得开、日志与码、六处种子；探针量启动时的 `lost` 与 `newAcmeTeam` 返回的先后；把 `river/jobs.go` 与 `maintain.go` 换回 `2d32d346` 的反向对照；网页的中文一句。<br>B T3：0x7075 的解析、整包的判断、解读之后的安全（24,066 个双字节逐个解）、`Classify` 的 `Read` 与 `importRun`；差分的 13 个下限照报告的方法重量；变异 B1–B14。<br>C T2、契约与网页、每个提交单独为绿、文档草稿：码的次序、并发的探针（8 个导出同时开）、回滚、上传记人、网页的 P2/S1 写法；在自己的工作树里逐个提交跑门禁；自己的分份判法；变异 MC-1–MC-7 |
| 日期 | 2026-11-17（实施、审查、修复第 1 轮、核对第 1 轮） |
| 结论 | 没有 Important。Minor 2：`unicodePath` 的三处边界没有种子或测试钉住，去掉或放松照绿（P3S3B-M1）；后台开始在队列锁下的按人计数没有测试钉住（P3S3C-M1）。Nit 10。三位都确认：T4 不误伤这个进程自己取到的任务，取到又开始了的与只取到没开始的分得开，四个 T4 测试在没修的生产代码上都红；T3 照 3.12.1 的五条规则，B 的 24 条变异除去三处边界、`Special` 与"只在成功时加"都红，差分的 13 个下限逐个重量相同；T2 的五处调用与码的次序同设计，同一个人同时来的请求不会一起越过上限（真库，竞态，三次）。修复第 1 轮 7 个提交，核对第 1 轮三位的旧发现都解决，新的只有 2 条 Nit（只在文档），在本提交改 |

## 审查的发现与处置

编号照审查者的报告。审查时的代码位置是 `a48ace20` 的。修复第 1 轮照主持者的修复说明（`p3s3/fix-1.md`）在 `a48ace20` 之上加 7 个只改测试的提交，普通推送、不改写已推的历史，每个单独为绿；文档的部分在实施者的草稿里改，落在本提交。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P3S3A-N1 | 细节（文档稿） | M8/P4 `04-P4-background.md:912` 引着决定 482 去掉的后半句"不要先撤销这次撤销"，没有后注；决定 481 给同一种情形的三处都加了 | 本提交照决定 481 的先例在那一句之后加后注，指向 00-M12 决定 482 与 P3 设计 3.22 第 1 条；M9/P3 `03-P3-batch-revert.md:2100` 只说那条提示"留着"，仍对，不改。核对 A：解决 |
| P3S3A-N2 | 细节（测试） | 中文的 `transfer.revertLeftGroup` 没有测试整句比对：把后半句放回去，三个相关的测试文件全绿 | 照 A（`cbda8435`）：`transfer-report.test.ts` 对 `en`、`zh-CN` 各整句比对这一句。变异修之前绿、修之后红。核对 A：解决 |
| P3S3A-N3 | 细节（测试与文档） | `newAcmeTeam` 不等后台任务起来：A 的探针量得启动时的 `lost` 总比它返回早约 0.7 秒，3.14.3 的 `-race -count=20` 证明的是"不偶发"，没碰上赛跑；三处种子把任务行与 River 行分成两条语句，`lost` 落在两条之间就会把那一行记成失败，"消掉赛跑"实际缩成一个往返 | 照 A 的 (a)（`aec2ec6a`）：`riverHeldJobSQL` 把任务行的 `INSERT` 包进 `WITH j AS (… RETURNING id)`，River 行从 `j.id` 写，一条语句；`TestTheQueuesLimitIsTheConfigured`、`TestOneImportAtOnceThroughServe`、`TestAPersonsSixthJobIsRefusedAndAnothersStarts` 三处。对照：照原样绿；每个种子之后插一次收拾，River 行照写的绿、不写的三个都红。3.23 第 13 条记 A 量的余量。核对 A：解决，另提 C1-N1 |
| P3S3A-N4 | 细节（测试） | 启动时 `lost` 失败就让启动失败（`jobs.Job.Start`），没有测试钉住：改成"记一笔、继续"全绿 | 照 A（`de177838`）：`TestTheRescue` 末尾一格，River 的读失败时 `AtStart` 交回错误、没有任务被记失败（替身 `held{err}` 本来就能注入）。3.23 第 14 条。核对 A：解决 |
| P3S3B-M1 | 中（测试） | `unicodePath` 的三处边界（`names.go:40` 的 `size > len(extra)-4`、`:48` 的 `len(field) < 5`）没有种子钉住：B5 去掉 `len(field) < 5`、B6 把边界放宽 4 字节、B7 越界时把尺寸截到余下的，普通的 `go test` 都绿。设计列的"截断"种子截在 CRC 里，被 `len(field) < 5` 挡掉 | 照 B（`65315a27`）：模糊的种子加三个：整个只有 1 字节的字段 `75 70 01 00 01`、尺寸比余下的多 1 字节的 `full[:len(full)-1]`、截在 CRC 与名字的第一个字 `测` 之后的 `full[:12]`。B5 红在种子 #9（越界 panic），B6 红在 #10，B7 红在 #10、#11。核对 B：解决，另提 C1-N1 |
| P3S3B-N1 | 细节（测试） | `Special` 按读出的名字判，没有一格钉住：改成按原名结尾的 `\` 判（B11）全绿。GBK 名字以 0x5C 结尾的符号链接会既不是文件夹、也不是特殊文件，被当附件收下 | 照 B（`89fa3b13`）：表第 11 格加符号链接 `资料/俓`（`…82 5C`），断言读出的名字、不是文件夹、`special`。B11 红。核对 B：解决 |
| P3S3B-N2 | 细节（测试） | 失败、取消的导入也列出读过的名字（决定 372 不分结局），没有一格钉住：改成只在成功时加（B12）全绿 | 照 B（`36b27738`）：`TestAnImportListsTheNamesReadOtherwiseLast` 加一个失败的导入（`MaxNodes` 2，`too_many_nodes`），报告恰是 `name_gbk: 图片/截图.md`、`name_unicode_path: 说明.md`。B12 红。核对 B：解决 |
| P3S3B-N3 | 细节（文字） | 契约、domain 的注释、网页两种语言说这两种名字"不是 UTF-8"；从字段读出的 ASCII 名字、决定 376 下读成 GBK 的合法 UTF-8 名字其实是合法 UTF-8 | 不改（决定 532）：对读者的意思是"名字照原样读不出"；3.23 第 12 条记一句。核对 B：解决 |
| P3S3C-M1 | 中（测试） | 后台的恢复与撤销先在锁外按人数一次、再在队列锁下数一次（3.13.1 要同一把锁之下）：所有的格都被锁外那一次拒了，锁下那一次不传人（MC-1）全绿 | 照 C（`3e6174cb`）：`TestAPersonsSixthBackgroundStartIsRefused` 加 "four of his, his fifth after the first count"：替身的 `onCount` 在锁外的计数之后给他加第 5 个，恢复与撤销都答 `ErrTooManyJobs`、不入队。MC-1 红在新的两格；锁外那一判去掉（MC-4）时新的两格照绿，钉的正是锁下那一判。核对 C：解决 |
| P3S3C-N1 | 细节（文档稿） | 契约的说明与 3.15 给的句子不同（并进分号列着的那一串、`caller`、导入的 `the other imports`；`TransferProblem` 多半句 `path being the name so read`），3.23 没有记 | 3.23 第 11 条照实记；意思与设计一致。核对 C：解决（恢复、撤销两段是 `…, transfer.too_many_jobs: …`，没有 `is`，草稿引的是导出、导入的那一种；不改） |
| P3S3C-N2 | 细节（文档稿） | 10.3 草稿说每个改网页的提交都跑了 knip 与 `test-web`：`9aea99ec` 的 `test-web`、`292b238f` 的 knip 与 `test-web` 是在改前的 `03b29219`、`fae5c81e` 上跑的 | 10.3 照实写；C 在最终的 sha 上补跑，都是 0（4,608、4,614 个，"门禁与 CI"）。核对 C：解决 |
| P3S3C-N3 | 细节（文档稿） | 变异的总数把两条正向对照算进"全抓到"；第七份的 324 秒是推断，没有标 | 写成服务端 50 条红、2 条正向对照照预期绿、网页 3 条红；324 秒标推断，并写 CI 的实数（Xeon 6975P-C 上 4:10）。核对 C：解决 |

## 主持者的决定

负责人可以改判；写进 [00 号文档](../00-M12-design.md)第 11 节"2026-11-17：P3/S3 的决定 530–532"：

- **530 `a48ace20` 留作单独的提交**（P3 设计 3.23 第 2 条）：决定 482 的文字在 S3 的 `en.ts`、`zh-CN.ts` 里（3.11 的文件），断言它的 `transfer-background.test.tsx` 不在 3.11 的文件表里；实施者单独成一个提交，主持者留下。
- **531 README 的两句与总体设计 13.1 第 4、23 条随 Step 的文档提交，不随代码的提交**（偏离 6.3；3.23 第 1 条）：文档的提交是主持者的。代码的提交不碰 `docs/` 与 `README.md`。
- **532 "不是 UTF-8"的文字不改**（P3S3B-N3；3.23 第 12 条）：契约、domain 的注释与网页两种语言照设计 4.2、3.15 写的；对读者的意思是"名字照原样读不出"。从字段读出的 ASCII 名字、决定 376 下读成 GBK 的合法 UTF-8 名字，其实也是合法 UTF-8，3.23 记一句。

修复说明里另的（不编号）：
- 修复第 1 轮收下 A-N2、A-N3、A-N4、B-N1、B-N2，各补测试或改种子；A-N3 把三处种子写成一条语句（修复说明给的两条路里取"改成一条语句"，改得成，所以 10.3 不另写"没有测试碰上赛跑"那一种）。C-N1、C-N2、C-N3、A-N1、B-N3 进文档。
- 修复核对之后不再开第二轮：三位都没有 Important、Minor；新的 2 条 Nit 只在文档，在本提交照审查者改（"核对"）。

## 实施中定的

实施报告第 7 节与 P3 设计 3.23 第 3–10 条（三位审查者各在自己的范围里判过；实施者没有要编号的产品决定）：

- app 的替身认得来源：`archives` 加一张 `from`（名字 → 来源），`zipArchive.Entries` 照它填 `From`（`app/fakes_test.go`），app 的测试经 `Import.Run` 的整条路（第 3 条；设计写"替身照旧，测试直接构造 `RawEntry`"）。
- zip 的帮手：适配器的表直接交 `*zip.FileHeader`，整个程序的测试有自己的 `unflaggedZip`；`zipEntry`、`vaultZip` 不动（第 4 条）。
- 差分在新的 `names_internal_test.go`（`readNames` 不导出；第 5 条）。生成器用 `math/rand/v2` 的 PCG 与 `IntN`，不经浮点（B 核对过几处转换不溢出）。
- 一个条目有几个 0x7075 时第一个说了算，无效时不往后找；模糊的断言是"当且仅当"（第 6 条）。B 的 B3（只看第一个扩展字段）由种子 #5 抓到。
- `Read` 收作为库的顶层文件夹、本身不导入的那一个条目；路径是 zip 里读出的名字（第 7 条）。B 的探针核对过。
- 来源到码是 `NameSource` 的不导出方法，不是包级的表（`gochecknoglobals`；第 8 条）。
- T2 的 app 格是三个新测试，回滚一格在已有的 `TestARollbackStartsInItsUpgradeAlone` 里；`importCut` 那一格改为整句比对，没有另加（第 9 条）。
- T4 的测试帮手 `root.module`；app 的替身 `InterruptJobs` 记进调用表（第 10 条）。

## 接受与推后的

- **接受**：
  - 整包的判断把会被忽略的条目（`.git/`、`.trash/`、`__MACOSX/` 下）也算作候选，其中一个解不开就整包不按 GBK 读；设计第 3 条没有把它们除外，实现照设计（B 的未核实的怀疑）。
  - 导入的 `Check` 不加锁、先读上传再读行，那个导入提交了行之后、`rowWritten` 之前的瞬间，一个人的另一个导入可能被数两次、多答一次 `too_many_jobs`：与全局计数已写明的"数两次，不漏"同一个取舍，窗口在微秒级，没有复现（C 的未核实的怀疑）。
  - 启动时 `lost` 的错误包装仍是 "rescue the jobs River dropped"，启动时略不贴切；上一个进程活着时 River 已丢下、还没等到 5 分钟那一轮的排队行，启动时也记成 `left by a stopped server`（决定 374 不读 River 的错误文字的代价）。A 判不报。
  - 去掉 `room` 里 `person != (uuid.UUID{})` 的防护（MC-5）等价：零 UUID 对不上任何行的 `created_by_id`，也对不上任何算数的上传。
  - 3.14.3 第 4 处（`rollingBackIn`）的反向对照不确定：`TestUpgradePermissionMatrix -count=5` 里 4 次红、1 次绿，同设计说的。
  - 分份脚本"去掉 `$$`"的对照不会失败（没有别的顶层测试以两个新名字开头），同 S1。
- **推后、交出去的**：
  - Shift-JIS、Big5、EUC-KR 的名字读成乱码的中文；GB18030 的四字节让整包不按 GBK 读；只有恰好合法 UTF-8 的 GBK 名字的包认不出；GBK 的包里没有标志的真 UTF-8 名字读成 GBK（决定 376 的回归）：发布说明的已知限制（P5/S3，P3 设计第 9 节），适配器的表第 7、10、12、13 格各钉一样。
  - 导入时选编码、CP437、Shift-JIS 等的识别，按人的上限做成配置项：v0.2 的 `M12-transfer.md`（P3 设计第 9 节，收尾时写）。
  - 合并之后两次 `main`：第七份与竞态一份的 Test 一步照 06-environment 第 4 节记进 00 号文档第 13 节。

## 反向对照

工具：`docs/v0.1/claude_to_codex/tools/mut.py` 的副本（`NW_SERVER` 指工作树，从 `.mutbak` 复原），每张表先在没改的树上把每个命令跑一遍为绿，之后 `git status` 干净、没有 `*.mutbak`。实施时在 `a48ace20` 上：轻的 34 条（archive、domain、app 加 http、river 四个命令）、重的 18 条（组合根与库的测试）、网页 3 条（vitest）。

**7.3 的 S3-1 到 S3-18，都红**：

| # | 变异 | 由什么抓到 |
|---|---|---|
| S3-1 | 不看标志（去掉 `Flags&0x800` 那一支） | 表第 8 格；差分 |
| S3-2 | 读 `NonUTF8` 而不读 `Flags` | 表第 4、8 格；差分 |
| S3-3 | 没有候选的包也按 GBK 读 | 表第 7 格、第 10 格（单独）；差分 |
| S3-4 | GBK 的包里恰好合法的 UTF-8 名字不读 | 表第 6 格、第 10 格（GBK 旁）；差分；整个程序 `TestAnImportReadsNamesNotInUTF8` |
| S3-5 | 有 U+FFFD 也算读得出 | 表第 9、13 格；差分 |
| S3-6 | 一个候选解不开时其余照样读 | 表第 9、13 格；差分 |
| S3-7 | 0x7075 的 CRC 不核 | 表第 2 格；差分；模糊的种子 #1、#4 |
| S3-8 | 按解读之前的名字判文件夹 | 表第 11 格（`资料/乗`） |
| S3-9 | 报告不写解读过的名字：`Classify` 不加 `Read` / `importRun` 不加 | domain 的 "names read otherwise"、app 的那一格 / app 的那一格、整个程序 `TestAnImportReadsNamesNotInUTF8` |
| S3-10 | 解读过的名字排在别的问题之前 | app 的 1,100 条一格 |
| S3-11 | `room` 不按人数 | app 三个新测试与回滚一格；http 四格；整个程序 `TestAPersonsSixthJobIsRefusedAndAnothersStarts` |
| S3-12 | 按人的拒绝答成 `ErrQueueFull` | 同 S3-11 的 app 与 http 各格 |
| S3-13 | 回滚也按人拒绝 | `TestARollbackStartsInItsUpgradeAlone` |
| S3-14 | 正在上传的导入不按人记 | 三个新测试里"…and an import she/he uploads"各格（`Check`），导入的 "…his fourth after the Check"（`write`） |
| S3-15 | 先判全局、再判按人 | 三个新测试的"…the queue full"各格；http 四格（替身 `maxQueued` 3，五个任务让两样都满） |
| S3-16 | `AtStart` 不跑 `lost` | `TestTheRescue`；模块的 `TestAQueuedJobRiverHeldRunningFailsAtTheStart`；`TestServeFailsTheJobsLeftRunning`（`taken queued`、`waiting dropped queued`） |
| S3-17 | 某一种不设 `DiscardRunning`（五种各一次） | River 适配器的那一格；是导入时，模块的那一个与 `TestServeFailsTheJobsLeftRunning`（`taken queued`）也红 |
| S3-18 | 收拾的任务排在五种之前 | 模块的那一个（次序）；`TestServeFailsTheJobsLeftRunning` |

**3.20 的对照**：差分先在"所有没有标志的名字都按 GBK 读"上红（表第 7、9、10、13 格也红）。3.14.3 的六处种子各去掉 River 行：`waiting held` 红；`importing` 红；矩阵的读与取消两格这一次都红；`rollingBackIn` 5 次 4 红；`TestTheRescue` 在 S3-16 下红；第 6 处的两个测试去掉 River 行、种子之后插一次收拾（`tm.rescued`）都红，带着 River 行插收拾照绿（两条正向对照）。分份的脚本：去掉 `$$` 照绿；从 `SEVENTH` 去掉、也加进 `EIGHTH`、只在 `EIGHTH` 都 rc=1。

**实施者另加的，都红**：O-1 第一个 0x7075 无效时往后找（模糊的种子 #4）；O-2 空的名字也取（#6）；O-3 字段里不是 UTF-8 的也取（#7）；O-4 版本不核（表第 3 格、差分、#2）；O-5 字段与原名相同也记（表第 4 格、差分）；O-6 ASCII 的名字也经 GBK（差分）；O-7 `Read` 也收被忽略的（domain）；O-8 读出的名字记成跳过（app）；O-9 `AtStart` 只收拾第一种（`TestTheRescue`）；O-10 别人正在上传的也算我的（`TestTheUploadsCountAsTheirRowsAreWritten` 与三个新测试里别人的格）；O-11 第六个也让过（app 各格）；O-12 `CountActive` 的 `mine` 不按人（`TestThePersonsJobsCountAmongAll`）；网页 W-1 `offeredOf` 不收 `too_many_jobs`（`trash-background`）、W-2 码到文字的表去掉它（五个文件各一格；`page-menu` 那一格第一次的命令路径写错没跑到，补跑红）、W-3 `name_gbk` 说成 `unicode_path` 的文字（`transfer-report` 两种语言）。

合计：服务端 50 条红、2 条正向对照照预期绿，网页 3 条红；没有抓不到的。

**修复第 1 轮的**（`cbda8435` 上；修之前的绿是审查者量的，B12 与 MC-1 实施者写那一格之前也手跑过）：

| # | 变异 | 修之前 | 修之后，红在 |
|---|---|---|---|
| B5 | 去掉 `len(field) < 5` | 绿（B） | 模糊种子 #9（越界 panic） |
| B6 | `size > len(extra)-4` 改成 `size > len(extra)` | 绿（B） | #10（越界 panic） |
| B7 | 越界时把尺寸截到余下的 | 绿（B） | #10、#11（`full[:12]` 读出完整的 `测`） |
| B11 | `Special` 按原名结尾的 `\` 判 | 绿（B） | 表第 11 格 |
| B12 | 只在成功时加读过的名字 | 绿（B） | `TestAnImportListsTheNamesReadOtherwiseLast` |
| MC-1 | 锁下的 `room` 不传人 | 绿（C） | 新的两格（恢复、撤销） |
| A-N4 | 启动时 `lost` 失败照样往下 | 绿（A，沿代码） | `TestTheRescue` 新的一格 |
| A-N2 | 中文放回"；不要先撤销这次撤销。" | 绿（A） | `transfer-report.test.ts` 的 zh-CN 格 |

**审查者另加的**：A 把 `river/jobs.go` 与 `maintain.go` 换回 `2d32d346` 的，四个 T4 测试都红（核对第 1 轮在 `cbda8435` 上重跑，`TestTheRescue` 另红在新的一格）；B 的 B1–B14（B1 合法 UTF-8、解出 U+FFFD 也读，B2 解不开的合法 UTF-8 也挡整包，B3 只看第一个扩展字段（模糊的种子 #5），B4 有标志的也读字段，B8 `Entries` 不交 `From`，B9 ASCII 的不读字段，B10 越出目录的不进 `Read`，B13 照原名的适配器，B14 不读字段：都红；B5–B7、B11、B12 见上；R-diff "没有标志、解得开的都按 GBK 读"差分红），核对第 1 轮在 `cbda8435` 上把第一轮的 24 条（S3-1–S3-10、R-diff、B1–B12）重跑，全红；C 的 MC-2、MC-3、MC-4、MC-6、MC-7 红，MC-5 等价（"接受"）。

## 核对

**核对第 1 轮**（2026-11-17）：三位各把自己的工作树挪到 `cbda8435`，之后干净、没有 `*.mutbak` 与探针。三位的旧发现都解决，都没有 Important、Minor；新的 2 条 Nit 只在文档，主持者定照审查者改，都在本提交：

- **A**：N1–N4 解决。N2、N4 的变异修之前绿、修之后红；`riverHeldJobSQL` 一条语句，`lost` 先读行、后读 River，窗口确实没了，三个测试竞态 `-count=3` 9 次全过；第一轮的反向对照在 `cbda8435` 上重跑照红。新的：
  - **P3S3A-C1-N1**：3.23 第 13 条草稿说矩阵的种子"在起应用之前写"，不对：`prepareMatrix` 在 prepare 子测试里先起一个应用，注册各列之后才以两条语句种排队导出。它不怕赛跑，是因为那个应用启动时的 `lost` 在注册之前就跑完了、周期的收拾 5 分钟之后才跑，各格的应用在种完之后才在库的副本上起。本提交照这样改写（3.23 第 13 条），代码不改。
- **B**：M1、N1、N2、N3 解决。三个新种子各抓到至少一个变异；第一轮的 24 条变异在 `cbda8435` 上全红。新的：
  - **P3S3B-C1-N1**：`full[:12]` 的注释（`names_fuzz_internal_test.go:60-63`）与实施报告说"截在 `测` 的中间"，其实截在 CRC 与 `测` 之后：名字部分是完整的 `e6 b5 8b`，B7 下读出 `测`、合法，所以红；真截在中间的 `full[:11]` 读出的不是合法 UTF-8，抓不到 B7（B 核过：只留 `full[:11]` 时 B7 照绿）；今天的覆盖不受影响，`full[:len(full)-1]` 也抓 B7。本提交的文档照实写（3.23 第 6 条、10.3）；测试里的注释照旧，不改；`65315a27` 已推，不改。
- **C**：M1、N1、N2、N3 解决，没有新的。7 个提交在 C 的工作树里逐个单独为绿；MC-1 修之后红在新的两格，MC-4 下新的两格照绿；草稿的数与日志一致。

一轮之后没有 Important、Minor 留下，不开第二轮修复。

## 实测

本机 8 核 arm64，都经 `heavy.sh`：

- **新的整个程序的测试**（`go test -run '^Name$' -count=1 -v ./internal/bootstrap`，`a48ace20`，先编译一次）：`TestAnImportReadsNamesNotInUTF8` 1.36 秒、竞态 2.23 秒（B 另量 1.42、2.31、2.33 秒；设计推约 1–2 秒）；`TestAPersonsSixthJobIsRefusedAndAnothersStarts` 1.31 秒、竞态 2.17 秒。第七份竞态下约多 4.4 秒，加上 S1 的 4.6 秒共约 9 秒，超出 7.5 的"约 4–6 秒"（那是四个测试合起来的推断）；推断第七份在 EPYC 7763 上约 324 秒、余量约 126 秒，实施的 CI 上它落在 Xeon 6975P-C 上，Test 一步 4:10。
- **竞态一份**：差分竞态 0.14 秒（非竞态 0.02 秒），表 0.04 秒，模糊的种子 0.00 秒，archive 包竞态 1.32 秒；app 的 1,100 条一格竞态 0.88 秒（B）。
- **3.14.3 的四个测试 `-race -count=20`**（第 1 个提交上）：80 次全过，包 100.2 秒（A 重跑 101.5 秒）。A 的探针：20 次里启动时的 `lost` 都比 `newAcmeTeam` 返回早 697–729 ms，开完笔记本时 703–736 ms，所以这 80 次证明的是"不偶发"（3.23 第 13 条）。
- **模糊**：实施者提交前 30 秒 8,904,087 次、B 20 秒 2,213,874 次，都没有失败，没有写出 `testdata/fuzz`。
- **差分的下限**：13 个计数，测试自己的种子 1–3 与另 20 组的最少次数在 P3 设计 10.3 的表；B 照报告的方法重量，逐个相同。

## 门禁与 CI

- **逐个提交**（实施者在干净的临时 worktree 里逐个 `checkout --detach`，经 `heavy.sh`）：

  | 提交 | build | vet | lint | short | 另跑 |
  |---|---|---|---|---|---|
  | `62e1416e` | 0 | 0 | 0 | 0 | transfer 模块的库测试 0；四个测试 `-race -count=20` 0 |
  | `9aea99ec` | 0 | 0 | 0 | 0 | `gen-check` 0、knip 0；`test-web`、库测试、整个程序的那一个与分份是在改前的 `03b29219` 上跑的（只差格式），都是 0 |
  | `292b238f` | 0 | 0 | 0 | 0 | `gen-check` 0、库测试 0、`TestAnImportReadsNamesNotInUTF8` 竞态 0；knip、`test-web` 与分份是在改前的 `fae5c81e` 上跑的（只差 Go），都是 0 |
  | `a48ace20` | 0 | 0 | 0 | 0 | knip 0、`test-web` 0（4,614 个） |
  | `65315a27`–`de177838`、`aec2ec6a` | 0 | 0 | 0 | 0 | — |
  | `cbda8435` | 0 | 0 | 0 | 0 | knip 0、`test-web` 0（4,616 个） |

  之后已跟踪的改动都是 0。审查 C 在自己的工作树里把 11 个逐个再跑：build、vet、`make lint`、`make test-go-short` 都是 0；`9aea99ec`、`292b238f` 另跑 `gen-check`、knip、`test-web`（4,608、4,614 个），`a48ace20`、`cbda8435` 另跑 knip、`test-web`，都是 0。
- **推送之前**（每一步单独经 `heavy.sh`，之前 `df -h /`）：
  - 实施的头 `a48ace20`：`make lint gen-check knip build` 0（410 秒）；`make test-go` 0（组合根八份、竞态、成本与守卫，185 行 `ok`、没有 `FAIL`，3,772 秒，含在锁上排队的时间）；`make test-web` 0（4,614 个）；`make test-go-short` 0；`make e2e` 0（576 个通过，1 个照旧跳过：WebKit 的 A15 一格；TR4 的 `f\xff.md` 照旧 `name_not_utf8`）；分份脚本 OK。
  - 修复第 1 轮的头 `cbda8435`：改了组合根的三个测试，跑它们所在的第三、四、七份（`make test-go-bootstrap-3 test-go-bootstrap-4 test-go-bootstrap-7`）0，包 241、205、205 秒；`go test -race ./internal/modules/transfer/...` 8 个包 ok。
  - 合并 `main` 之后的 `7dfb248a`：`make lint gen-check knip build`、`make test-go-short`、`make test-web` 都是 0（4,616 个；`main` 带来了 `e2e/` 的配置与锁文件）。合并没有冲突（先以 `git merge-tree --write-tree --name-only` 看过），S3 的提交不碰 `main` 带来的文件，也不碰 `docs/`。
- **分支的 CI**（`ci.yml` 每次 19 项第一次 attempt 全绿，没有重跑，没有碰到已知的偶发；Test 与 E2E 是那一步的用时，CPU 是作业的 "Runner CPU" 一步）：

  | 作业 | `a48ace20`，run 43919318998 | 修复第 1 轮 `cbda8435`，run 43948816167 | 合并 `main` 的 `7dfb248a`，run 43969116486 |
  |---|---|---|---|
  | bootstrap-1 | 6:29，EPYC 9V45 | 5:17 | 6:04，Xeon Platinum 8573C |
  | bootstrap-2 | 4:17，Xeon 6975P-C | 4:13 | 3:40，EPYC 9V45 |
  | bootstrap-3（`TestTheQueuesLimitIsTheConfigured`） | 4:38，Xeon 6975P-C | 4:37，Xeon 6975P-C | 5:56，Xeon Platinum 8370C |
  | bootstrap-4（`TestServeFailsTheJobsLeftRunning`、`TestOneImportAtOnceThroughServe`） | 4:49，Xeon 6975P-C | 4:53，Xeon 6975P-C | 6:25，Xeon Platinum 8370C |
  | bootstrap-5 | 6:25，EPYC 7763 | 6:26 | 5:37，EPYC 9V74 |
  | bootstrap-6 | 5:19，Xeon Platinum 8573C | 4:34 | 6:06，EPYC 7763 |
  | bootstrap-7（两个新测试） | 4:10，Xeon 6975P-C | 4:05，Xeon 6975P-C | 4:05，Xeon 6975P-C |
  | bootstrap-8 | 4:17，Xeon 6975P-C | 4:08 | 5:05，Xeon Platinum 8573C |
  | race（差分与模糊的种子） | 9:08，Xeon 6975P-C | 9:04，Xeon 6975P-C | 9:11，Xeon 6975P-C |
  | e2e | E2E 一步 6:15 | 6:16 | 6:18 |

  三次的八份都在 450 秒之内（最慢的是合并之后第四份的 385 秒，落在 Xeon Platinum 8370C 上）；第七份三次都落在 Xeon 6975P-C 上，没碰到 EPYC 7763。修复第 1 轮那一列只记了 S3 改过的几份的 CPU。
- **文档提交 `9487403e` 的分支 CI**：run 43974677584，19 项第一次 attempt 全绿（第七份 Test 5:28，竞态一份 Test 14:38，比此前三次的约 9 分慢，原因没有查，合并之后的 `main` 回到 9:18；E2E 6:17）；S3 合并为 `7696d8df`，之后第一次 `main` 的 run 43978501165 同样全绿（第七份 Test 3:58，竞态 9:18，E2E 5:58）。
- 合并之后两次 `main` 的各份照 06-environment 第 4 节记进 00 号文档第 13 节。

## 分份

3.19 的脚本（实施者的 `shards.py`，`go test -race -list . ./internal/bootstrap` 的列表，照八份的 `-run`/`-skip` 判）：第 2 个提交上 557 个顶层，第 3 个与头上 558 个，145/50/119/120/60/15/34/15，每个恰在一份；`TestAPersonsSixthJobIsRefusedAndAnothersStarts`、`TestAnImportReadsNamesNotInUTF8` 只在第七份；修复第 1 轮没有新的顶层测试。审查 C 的 `shards_c.py` 不照抄规则、读每个 `test-go-bootstrap-N` 配方里写的 `-run`/`-skip`，结果相同；S1 的两个照旧在第七份，`TestServeFailsTheJobsLeftRunning` 照旧在第四份。

## 环境

实施、审查与核对各在自己的工作树里（实施者 `nerve-wiki-m12-p3-s3`，逐个提交与门禁在 `~/.cache` 下的临时 worktree；审查者 `nerve-wiki-m11-p1-s1-rev-{a,b,c}`，C 的变异在一份解出的拷贝上），`TMPDIR`、`GOTMPDIR` 与原始日志在 `~/.cache` 下各自的目录，用完删掉；探针（`zz_*_test.go`）从没提交。实施者一开始没设 `TMPDIR`，`/tmp` 的配额满了一次，之后照规矩都设在 `~/.cache`。要留的汇总在设计工作目录的 `m12/p3s3/`（`impl-report.md`、`logs/`、`tools/`、`review/`）。
