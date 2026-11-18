# M12/P5 S1（版本号与流水线的构建）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m12-p5-s1`。实施的 9 个提交，从 `main` 的 `5e58e844` 开始，审查时在 `38916978`：<br>版本号的守卫 `68777b97`；MCP 的版本与 `make test-go-short` 盖版本号 `41df19a2`；镜像的三个 label、`make image`、`image-smoke.sh` `53332084`；`ci.yml` 只在分支上跑 `6504c341`；`make lint-actions` `b937d820`；`-format` 的换行 `2db96b0a`（决定 557）；闸门与说明 `ff453005`；`release.yml` 与守卫 `96bf93e1`；闸门与说明的测试补格 `38916978`（决定 558）。<br>修复第 1 轮 8 个提交，头 `a14d9ee5`：`make help` 又列出 `test-go-short`、smoke 的清单写上 label `e08b98b0`；`make image` 在 git 读不出时停下 `bc71d5fe`；守卫在每一处 `${{` 上核对 `59cbf8b8`；守卫守着 actionlint 那一步 `4ebb781f`；smoke 不写 `if`、演练不引用 secret 不登录 `86293682`（决定 560）；闸门的测试检出 tag 的提交 `300f8957`；MCP 版本的测试分开两种失败 `719c59de`；说明在同一个版本的第二个标题处结束 `a14d9ee5`。<br>修复第 2 轮 7 个提交，头 `ce8aaf64`：演练的 secret 与登录换了写法也抓到 `ee8f2ab4`；smoke 各恰一步、那一行后面没有别的 `c3734c36`；actionlint 那一步恰是三个键 `299953dd`；矩阵的值写死 `beb8a1b8`；闸门的测试检出 `GITHUB_SHA` 的提交 `c34cb297`；说明认的标题与闸门数的同一种 `fb3101df`；`image-smoke.sh` 的"需要"加 git `ce8aaf64`。<br>合并 `main` 一次：`b34e7ac3`（`0918ad73`：P4/S4、P4/S5、L1 的旁支与负责人的五问；带来的只有 `docs/`、`web/…/reading/app-links.test.ts` 与 `e2e/stories/links/l1-links.spec.ts`，没有冲突）。<br>分支共 24 个提交（不计合并与本提交），相对 `5e58e844` 13 个文件，加 1,424 行、减 24 行：`.github/workflows/ci.yml`、`release.yml`（新）、`Makefile`、`README.md`、`deploy/Dockerfile`、`image-smoke.sh`、`release-gate.sh` 与 `release-notes.sh`（新，可执行）、`bootstrap/mcp.go`、`bootstrap/mcp_version_test.go`、`platform/buildinfo/version_test.go`、`release_test.go`、`mcpserver/handshake_test.go`；不加依赖，契约、迁移与网页不动，`docs/` 在本提交 |
| 规格 | [P5 设计](../05-P5-deploy-release.md)第 0 节 S1 一行，2.1、2.3（决定 430–452），3.1–3.9，3.21，3.22，6.1，6.4，7.1–7.5，第 8 节；[00 号文档](../00-M12-design.md) 4.10、4.11，决定 308、324，第 11 节负责人 2026-11-17 的第 11 问 |
| 审查方式 | 三位审查者（Opus）并行，各在自己分离的工作树里（A `nerve-wiki-m12-p5-rev-a`、B `nerve-wiki-m12-p2-rev-b`、C `nerve-wiki-m12-p4-rev-c`，都挪到 `38916978`），重的经 `heavy.sh`，不碰实施者的工作树；`gh` 只读；本机不跑 actionlint；不推送、不登录、不提交：<br>A 版本号、MCP、镜像（`68777b97`、`41df19a2`、`53332084`）：五处默认值与守卫读的文件；`make test-go-short` 盖的版本号是否进了测试的二进制、写死默认值的变异；普通的克隆里 `make image`、`make image-smoke` 与直接 `docker build`，读 label；`make help`。<br>B `release.yml`、闸门与说明（`ff453005`、`96bf93e1`、`38916978`）：演练只读（run 44077699224 的作业、步骤、日志、注解与 artifact），tag 上的作业逐个对 3.8，`run:` 里的注入，`release.yml` 用的七个 action 的 SHA 与 BuildKit 的摘要（`docker buildx imagetools inspect`）；闸门与说明的探针 37 种情形加一格注入（假的 `gh`、临时的仓库）；守卫与闸门的变异 21 个。<br>C `ci.yml`、actionlint、变异、提交与文档（`6504c341`、`b937d820`、`2db96b0a` 与实施报告）：`ci.yml` 的触发与 Lint workflows 一步，`make lint-actions` 的本机拒绝（假的 `go`、`pnpm`），裸的 `\n` 对 actionlint v1.7.12 的源码，CI 上四次 Lint workflows；7.2 的变异抽 21 条、自加 14 条；九个提交逐个跑 6.1 那一组；用时对 7.4；待编号的小决定与文档草稿 |
| 日期 | 2026-11-17（实施）；2026-11-18（审查、修复两轮、核对两轮、合并 `main`） |
| 结论 | 没有 Important。Minor 7 条、去重 5 条：`make help` 不再列出 `test-go-short`（P5S1A-M1 = P5S1C-M1）；守卫查 `run:` 里的表达式，遇到含 `}` 的就漏（P5S1B-M1）；闸门的测试停在 `main` 上、生产里检出的是 tag 的提交，"祖先"比错了 ref 的变异照过（P5S1B-M2 = P5S1C-M2）；没有东西守着 `ci.yml` 还在跑 actionlint（P5S1C-M3）；文档草稿漏了总体设计第 15 节一行与 00 号文档第 12 节的一半（P5S1C-M4）。Nit 12：A 3、B 4、C 5。三位都确认：五处默认值一致、守卫读的是真文件；盖的版本号进了测试的二进制；label 逐字对；演练只读（令牌只有 `Contents: read`，没有登录的一步，`push: false`，artifact 0 个）；`release.yml` 与 3.8 只差占位符；闸门与说明的探针该拦的都拦下；7.2 抽查的都抓到，g6x 等价；九个提交各自为绿。修复第 1 轮 8 个提交；核对第 1 轮：第一轮的 19 条关上 18 条、P5S1B-N1 关了一半，新的 Minor 1（P5S1B-M3，C 的 N7 同一条）、Nit 9；修复第 2 轮 7 个提交；核对第 2 轮：上一轮的都关上，新的只有 Nit 4 条（P5S1A-N6、B-N10、C-N9 是同一条，交 S2，决定 562；B-N11 在本提交）。没有推任何 tag、镜像，也没有建 Release |

## 审查的发现与处置

编号照审查者的报告：P5S1 + 审查者（A、B、C）+ 级别与序号，核对两轮接着编号。审查时的代码位置是 `38916978` 的，核对第 1 轮是 `a14d9ee5` 的，第 2 轮是 `ce8aaf64` 的。修复两轮照主持者的修复说明（`p5s1/fix-1.md`、`fix-2.md`），在前一个头之上加提交，普通推送、不改写已推的历史，每个单独为绿。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P5S1A-M1、P5S1C-M1 | 中（`Makefile`） | `41df19a2` 把 `test-go-short: ## 只跑…` 写成了 `##只跑…`；`help` 的 awk 要 `## ` 带空格，`make help` 的 49 行里没有 `test-go-short`。README 让贡献者 `make` 看全部命令，又让只跑单元测试时用它。设计 3.3 没改这一行，提交说明也没提，是误改；没有测试或守卫看 `make help`，门禁全绿也看不出 | `e08b98b0`：改回 `## 只跑`，说明加"盖一个不是默认值的版本号"；`make help` 50 行，`test-go-short` 在里面，整个 `Makefile` 里 `##` 后不带空格的行是 0。核对 A、C：关上 |
| P5S1A-N1 | 细节（`Makefile`） | host 上的 git 失败时（例如 dubious ownership），命令替换不让一行失败：`make image` 以 0 退出、label 是空的；说明还写"只需要 Docker"。发布不受影响（`release.yml` 有自己的 Labels 一步，之后跑 smoke） | `bc71d5fe`：同一个 shell 行里先取 `revision`、`created`，任一失败就报"git 读不出提交与它的时间"并以 1 退出，再 `docker build`；说明改成"需要 Docker 与 git"。假的 `git` 以 128 退出时 rc 2、`docker` 没被调，`make -i`、`MAKEFLAGS=i`、`-j4` 也一样（核对 A 的 13 种情形）。核对 A：关上 |
| P5S1A-N2 | 细节（帮助文字） | README `:508` 与 `make help` 的 image-smoke 一行列出 smoke 核对什么，没有 label；设计 3.1 的 README 一行只列了 `:15-17`、`:29`、`:31` | `e08b98b0`：两处改为"实例、提交信息与镜像的 label"，"另需"加 git；设计 3.1 在本提交补（P5S1A-N5）。核对 A：关上 |
| P5S1A-N3 | 细节（测试的消息） | 盖的就是默认值时（例如 `TEST_VERSION` 被改成默认值），消息说"`-ldflags` 没有到"：测试照红，原因说错 | `719c59de`：两种失败分开报：盖的等于 `buildinfo.go` 的默认值时报 `TEST_VERSION` 要与它不同，二进制的版本号不是盖的那个时报 `-ldflags` 没有到；四种情形手跑过。核对 A：关上（c1–c8 八种情形；c7 的措辞见"接受与推后的"） |
| P5S1B-M1 | 中（守卫） | 守卫查 `run:` 里的表达式用 `\$\{\{\s*([^}]*?)\s*\}\}`，表达式里一有 `}` 这一处就整个匹配不上：`format('{0}', github.ref_name)`、`join(…, '}')` 写在 `release.yml` 与 `ci.yml` 的 `run:` 里都照过（b4–b4d），外来的文字照样进脚本 | `59cbf8b8`：不先截出 `${{ … }}`，从每一处 `${{` 起要匹配 `^\$\{\{\s*matrix\.[a-z_]+\s*\}\}`，不匹配的报出从它起的至多 60 个字节；b4–b4d 与对照 b2、b5、b8、b15 都红，三份工作流照过。设计 3.9 第 5 条在本提交改。核对 B：关上（另核 shell 的注释、单引号、`matrix.arch \|\| github.ref_name` 都报，YAML 的注释不报，函数没有死循环） |
| P5S1B-M2、P5S1C-M2 | 中（闸门的测试） | 测试的克隆停在 `main` 上（`release_test.go:192`、`:325`），`gate` 作业里 `actions/checkout` 检出的是 tag 的提交：第 3 条写成 `--is-ancestor "$sha" HEAD`（B 的 gc，即 C 的 cg1）29 格全过，到了 runner 上它对任何 tag 都成立 | `300f8957`：每格打 tag 之后 `checkout -q --detach` 到 tag 的提交，再写 `CHANGELOG.md`、跑闸门；gc、g2 由旁支一格抓到。核对第 1 轮之后照 P5S1B-N7 改成检出 `GITHUB_SHA` 的提交（`c34cb297`）。核对 B、C：关上 |
| P5S1B-N1 | 细节（说明） | 正式的一节之后紧接写坏了的 `## 0.1.0（草稿`：闸门只数以"）"结尾的标题，放行；说明把第二个标题当作新起点，草稿的正文并进了正式版本的说明 | `a14d9ee5`：awk 先判"已开始、遇到 `## `"再判起点，同一个版本的第二个标题也结束这一节；加一格整段比。核对 B 第 1 轮：关了一半，反过来的次序见 P5S1B-N5；第 2 轮随 N5 关上 |
| P5S1B-N2 | 细节（闸门） | 正式版本的日期只核对 `YYYY-MM-DD` 的形状，`## 0.1.0（2026-99-99）` 放行 | 不改脚本（决定 563）；设计 3.6 写明"只核对形状"（本提交）。核对 B：理由站得住；草稿把写日期的人写成了"负责人"，改为"主持者"（P5S1B-N9） |
| P5S1B-N3 | 细节（守卫） | 守卫没盖住三处：`cancel-in-progress: true`（b14）；`build` 的 smoke 一步加 `if: false`（b9）；演练以 `secrets.GHCR_PAT` 登录 GHCR（b10）。3.9 没要求，不算偏离 | 决定 560：加两条、不加 b14 那一条（`86293682`）；b9、b10 红。加的两条在修复第 2 轮照 P5S1B-M3、N6 扩开。核对 B：按主持者定的范围关上 |
| P5S1B-N4 | 细节（报告） | 实施报告 9.4 说跳过的矩阵作业"在 tag 的运行里是 `build (${{ matrix.arch }})`"，说反了：分支的运行里跳过的是 `build`，tag 的运行里应是 `rehearse`（推断） | 报告照实改了；设计第 9 节"P5/S2"写的是改后的一句（本提交）。核对 B：关上 |
| P5S1C-M3 | 中（守卫） | actionlint 只在 `ci.yml` 的这一步跑（决定 435），本机的门禁都不含它：删掉这一步（cw5）或 `if` 写成不存在的一份（cw6），守卫照过、CI 照绿，actionlint 就悄悄不跑了；`ci.yml` 的各份 M10、M11 都改过名 | `4ebb781f`：守卫加一条：`server` 作业里 `run` 含 `lint-actions` 的步骤恰有一步，`run`、`if` 逐字，没有 `continue-on-error`，`costs-1` 是这个作业的一份；cw5、cw6 与实施者的 fw2–fw5 都红。修复第 2 轮照 P5S1C-N6 收紧。核对 C：关上 |
| P5S1C-M4 | 中（文档稿） | 草稿漏了设计第 8 节第 5 条的总体设计第 15 节一行；00 号文档第 12 节只写"写到 S1"，没有审查的链接与合并的占位 | 本提交：第 15 节一行（"五处默认值"，P5S1C-N8）；第 12 节 P5 一行的三格。核对 C：关上 |
| P5S1C-N1 | 细节（测试） | MCP 的版本只测到 `mcpOptions`：`mcpServer` 里再覆盖 `o.Version` 的变异（cm1）在盖了版本号的 `make test-go-short` 下照过 | 不改（决定 563）：设计 3.3 有意只测 `mcpOptions`（决定 431），`mcpServer` 只剩一行 `mcpserver.New(mcpOptions(...))`，这一跳由代码的形状守着、不由测试守着（02-workflow 3.3"走不到最后一跳"）。核对 A、C：理由站得住 |
| P5S1C-N2 | 细节（文档稿） | 10.1 的用时只引了 `96bf93e1` 的演练、说"比推断的快"；7.5 要量的 Lint 一步、`short` 的 Test 一步、演练的 runner、`platform/buildinfo` 在 CI 上的用时没进去 | 本提交：10.1 的"用时"照几次推送的范围与 runner 写（另见 P5S1C-N8）。核对 C：关上 |
| P5S1C-N3 | 细节（文档稿） | "设计里随实施改的几处"漏了：2.1 第 10 条不用改；3.5 与 7.4 的"多 15–40 秒"实际 3 秒；7.2 加审查的变异；头的状态；13.5 的锚点 `:1289`、`:1291` 是 `86565dd6` 的行号（`38916978` 上是 `:1291`、`:1293`）；06-environment 加一句：actionlint 自己出错时注解只有 exit code 2 | 本提交照写（P5 设计 3.24 列出）。核对 C：关上 |
| P5S1C-N4 | 细节（文档稿） | 移交一行的草稿改了设计第 8 节第 7 条原文的标点，日期写成"<合并的日期>" | 照设计的原文（本提交）。核对 C：关上 |
| P5S1C-N5 | 细节（记录） | 第 5 个提交在 CI 的 Lint workflows 一步上红，原因与设计预料的"现有两份工作流过不了 actionlint"不同 | 写进本记录（"门禁与 CI"）与决定 557。核对 C：关上 |
| P5S1A-N4 | 细节（注释；核对第 1 轮） | `deploy/image-smoke.sh:8` 的"需要"没有 git，可它在 host 上跑 git；`make help` 与 README 已经加了 | `ce8aaf64`：改成"需要 Docker、git、curl、jq、openssl"。核对 A 第 2 轮：关上 |
| P5S1A-N5 | 细节（文档稿；核对第 1 轮） | 草稿没把 README `:508` 记进设计 3.1 的表 | 本提交：3.1 的 README、`Makefile`、`image-smoke.sh` 三行。核对 A 第 2 轮：关上 |
| P5S1B-M3 | 中（守卫；核对第 1 轮；C 的 P5S1C-N7 同一条） | "演练不引用 secret、不登录"只看 `rehearse` 作业本身、只认子串 `secrets.`：工作流顶层的 `env` 写 secret（演练继承它）加 `docker --config … login`（s4、s5）就整个绕过，效果同 b10；`secrets['GHCR_PAT']`、`toJSON(secrets)`、`SECRETS.GHCR_PAT`（s1–s3，C 的 cs1–cs3）、`Docker/login-action`（l2）也照过 | `ee8f2ab4`：`rehearse` 作业与顶层的 `env`、`defaults` 里每个字符串找 `(?i)\bsecrets\b`；演练每一步的 `uses` 与 `run` 合起来匹配 `(?i)login` 就报；s1–s5、l1、l2、cs1–cs3 都红，b10、fn2–fn4 照旧红。核对 B、C 第 2 轮：关上 |
| P5S1B-N5 | 细节（说明；核对第 1 轮） | 写坏的同版本标题在正式的一节之前时，闸门放行，`a14d9ee5` 的说明只剩草稿：说明从 `index($0, h) == 1` 的第一行起，闸门只数以"）"结尾的 | `fb3101df`：说明的两处 awk 都加 `&& /）$/`，认的标题与闸门数的同一种；加三格（写坏的在前、只有写坏的标题、同一个版本两个完整的标题）；`notesChangelog` 那一行改为以"）"结尾，n12 仍红。核对 B 第 2 轮：关上，第一轮 N1 余下的一半随之关上 |
| P5S1B-N6 | 细节（守卫；核对第 1 轮） | 守卫只管跑 smoke 的步骤不写 `if`：删掉这一步（k1、k3）或那一行后写 `\|\| true`（k2）照过，效果同 b9 | `c3734c36`：`rehearse` 与 `build` 各恰有一步跑 image-smoke，`release.yml` 里每个这样的步骤不写 `if`，`run` 里有一行匹配 `(?m)^deploy/image-smoke\.sh [^\|;&]*"\$VERSION"$`；k1–k3 红，k4、b9、fn1 照旧红。核对 B 第 2 轮：按报的写法关上；留着那一行、让它的失败不算的写法见 P5S1B-N10 |
| P5S1B-N7 | 细节（闸门的测试；核对第 1 轮） | `300f8957` 之后每格检出 tag 的提交，"闸门的提交取 `HEAD`"（gc2）照过；"GITHUB_SHA is another commit"一格与 runner 不一致 | `c34cb297`：检出 `GITHUB_SHA` 指的提交，没设时检出 tag 的，同 `actions/checkout`；gc、gc2、gc3、gc4 都红。核对 B 第 2 轮：关上（插桩：设了 `GITHUB_SHA` 的 28 格 HEAD 都等于它，没设的 1 格是 tag 的提交） |
| P5S1B-N8 | 细节（守卫；核对第 1 轮） | `run:` 里只许 `${{ matrix.* }}`，矩阵的值本身却可以是外来的：演练的 `include` 加 `note: ${{ github.event.head_commit.message }}`、smoke 一步 `echo "${{ matrix.note }}"`（e4）照过 | `beb8a1b8`：三份里每个作业的 `strategy` 的字符串都不许含 `${{`；e4 红。核对 B 第 2 轮：关上（YAML 的别名、`fromJSON` 的矩阵、经作业的 `env` 都挡住） |
| P5S1B-N9 | 细节（文档稿；核对第 1 轮） | 草稿三处：P5S1B-N2 那一句写日期的人；7.2 那句"cg1 从照过变抓到"对不上行；审查 B 未核实的第 1 条交 S2 与实施报告 9.4 的三条不在任何草稿里 | 本提交：3.6 写"主持者"；7.2 加 gc、gc2 两行；设计第 9 节"P5/S2"与 3.10。核对 B 第 2 轮：关上 |
| P5S1C-N6 | 细节（守卫；核对第 1 轮） | actionlint 那一步的守卫只看 `run`、`if`、`continue-on-error`：加 `shell: bash -c 'exit 0' {0}`（cx1）或 `env: {MAKEFLAGS: n}`（cx2），这一步什么也不跑、照绿，守卫照过；注释"with nothing to let it fail quietly"说过头 | `299953dd`：这一步要恰等于 `{name: Lint workflows, if: matrix.leg == 'costs-1', run: make lint-actions}`，`continue-on-error` 那一条并进去；cx1–cx6 都红；注释改为"of its own"。顺带照审查 B 第 1 轮的旁注，作业的 leg 改用 `field()` 一层层取，`matrix` 写成 `${{ fromJSON(…) }}` 时报错、不再 panic（fx1）。核对 C 第 2 轮：关上 |
| P5S1C-N8 | 细节（文档稿；核对第 1 轮） | 第 15 节一行写"四处默认值"，测试核对五处；10.1 的用时没收头 `a14d9ee5` 的数与 runner 的 CPU；"分支 CI"没写 `vuln.yml` 的 run 44164542725 | 本提交照写。核对 C 第 2 轮：关上；另一处措辞照它改：`continue-on-error` 是"失败了照绿"，不是"不跑" |
| P5S1A-N6、P5S1B-N10、P5S1C-N9 | 细节（守卫；核对第 2 轮） | smoke 那一行按行核对：前面一行 `set +e`（`build` 那一步在 smoke 之后还有一行架构的核对）、`true \|\| \`、`trap 'exit 0' ERR`、heredoc 里，或这一步、作业、工作流写不带 `-e` 的 `shell`（`shell: bash {0}`、`defaults.run.shell`、`shell: true {0}`），smoke 失败了这一步照绿或根本不跑，守卫照过。B 以 Actions 默认的 `bash --noprofile --norc -eo pipefail` 在本机核对了 `set +e` 等四种都以 0 退出；`shell: true {0}` 的效果是推断 | 决定 562：S1 不改，交 S2：S2 写 `publish`、`verify` 与它们的 `shell` 时，守卫一并不许跑 smoke 的步骤与作业写 `shell`、`defaults.run.shell`、`set +e`（设计第 9 节"P5/S2"与 3.10，本提交） |
| P5S1B-N11 | 细节（文档稿；核对第 2 轮） | 草稿"交给 S2"写"在 `build`（以及 `publish`）的 Labels 之前加一步"：设计 3.14 的 `publish` 没有 Labels，它检出仓库只为取脚本，不用 `HEAD` 构建 | 本提交：只写 `build` 的 Labels 之前；`publish` 的 Index 一步可以另加 `[ "$(label revision)" = "$SHA" ]`，直接核对推上去的镜像（设计第 9 节"P5/S2"，决定 561） |

## 主持者的决定

负责人可以改判；写进 [00 号文档](../00-M12-design.md)第 11 节"2026-11-18：P5/S1 的决定 555–563"。实施报告"修复第 2 轮"列的待编号的第 10–12 条（守卫与闸门测试在修复两轮里的扩开）并进 558、560，不另编号。

- **555 BuildKit 钉 `moby/buildkit:v0.34.2@sha256:c5f8862f52499773af85bc07695b2379ef08e8002352b0df2c4864c91d78dabe`，不取那时才发 6 天的 v0.35.0**（实施报告 7.2；审查 B、C 同意）：设计说取"S1 实施时的稳定版"（决定 447）。实施时 moby/buildkit 最新的正式版是 v0.35.0（2026-11-11），`buildx-stable-1` 仍指 v0.34.2（2026-11-03）：钉的就是 setup-buildx-action 不钉时那时会拉的那一个，不改变那时的行为。摘要是 index 的，含 linux/amd64 与 linux/arm64，两种 runner 都按它拉成功；审查 B 以 `docker buildx imagetools inspect` 读过 `v0.34.2` 与 `buildx-stable-1` 那时都指它。v0.35.0 以后照 4.3 的路升。
- **556 `actions/setup-go` 三份工作流照旧 v7.0.0，v7.1.0 一起升、走 4.3 的路**（实施报告 7.3；审查 C 同意）：3.21 的十一个 action 里只有它出了新版本。只升 `release.yml` 的 `gate` 会让闸门的 Go 环境与 `ci.yml` 的不同；`make outdated` 会一直列出它，不会忘。
- **557 `make lint-actions` 的 `-format` 末尾写裸的 `\n`**（实施报告 7.1；审查 C 对 actionlint v1.7.12 的源码核实）：actionlint 在 `template.Parse` 之前先把格式里的 `\n` 等反斜杠转义换成字符（`error.go` 的 `unescapeBackslash`），设计 3.5 写的 `{{"\n"}}` 就成了引号里一个真的换行，模板报"unterminated quoted string"、以 3 退出；裸的 `\n` 换成换行，正是要的，actionlint 自己的 GitHub 注解示例也这样写。第 5 个提交 `b937d820` 因此在 CI 的 Lint workflows 一步上红在我们自己的 `make lint-actions`，不是设计预料的"现有的工作流过不了 actionlint"（审查 C 的 N5）；照设计 3.5 的办法另起一个提交 `2db96b0a`，说明写真实的原因；之后三份工作流 0 错，没有 `build: the workflows pass actionlint` 那个提交。逐个提交的门禁照设计不含 actionlint，本机看不出。
- **558 闸门与说明的测试补设计的表没盖住的格子，每格照 runner 检出；说明认的标题与闸门数的同一种**（实施报告第 8 节与修复两轮；审查 B、C 同意，C 建议把检出写进同一个决定）：
  - 第 8 个提交 `38916978`：闸门加四格：同一提交另一个分支、另一种事件（`pull_request`）、另一个提交各有一次更新的绿运行（`select` 再核一遍分支、事件、提交），缺右括号的标题；"没有 IMAGE"一格要闸门自己的字样 `release-gate: 要 IMAGE`（否则说明的判断照样拒绝，盖住了闸门的）；说明的 `CHANGELOG.md` 加一行空格与一行前文，前文里出现 `### 0.1.0（草稿）`（守的是去掉结尾的空格行、只认行首的标题）。理由同 05-lessons 第 1 节：没有一格能让这一条单独变红，就等于没测；补之前 g15–g18、g20、n8、n12 照过。
  - 每格照 `actions/checkout` 检出：设了 `GITHUB_SHA` 时检出它指的提交，没设时检出 tag 指的，detached（P5S1B-M2 = P5S1C-M2、P5S1B-N7；`300f8957`、`c34cb297`）。
  - 说明的一节只从以"）"结尾的 `## X.Y.Z（` 起，"下一个 `## `"先判：同一个版本写坏了的标题在前不是起点，在后结束这一节，与闸门的"恰好一个"一致（P5S1B-N1、N5；`a14d9ee5`、`fb3101df`）；说明的格随之加四个。
- **559 README 的"需要安装"写"bash、git、jq 与 awk"**（实施报告 7.5；审查 C 建议给号）：设计写"加 jq"、括号里说 `make test-go-short` 也要 bash 与 git；测试同样检查 awk，四样都列出来。06-environment 第 1 节同此。
- **560 守卫加两条、不加一条，加的两条照核对第 1 轮扩开**（审查 B 的 N3，`fix-1.md` 第 5 条；核对第 1 轮 B 的 M3、N6，C 的 N7，`fix-2.md` 第 1、3 条）：
  - 加：`release.yml` 里跑 image-smoke 的步骤不写 `if`（`86293682`）；修复第 2 轮起 `rehearse` 与 `build` 各恰有一步跑它，`run` 里有一行 `deploy/image-smoke.sh … "$VERSION"`、后面没有别的（`c3734c36`）。S2 的 `verify` 加进来同样受它管。
  - 加：演练不引用 secret、不登录（`86293682`）；修复第 2 轮起 secret 查 `(?i)\bsecrets\b`，范围是 `rehearse` 作业与工作流顶层的 `env`、`defaults`（每个作业都继承它们），登录查演练每一步 `uses` 与 `run` 合起来的 `(?i)login`（`ee8f2ab4`）。演练推不出东西靠令牌只有 `contents: read`；一个别人的令牌（secret）加一次登录就越过它，这两条挡的是这一种。会多报的方向安全（"login""secrets"这两个词），今天的演练里都没有。
  - 不加："tag 的运行不取消"（b14，`cancel-in-progress: true`）。取消发生在 `build` 之前时什么也没推；在它之后，GHCR 里留下按摘要推的、没有 tag 的摘要，与失败时相同（决定 444）；版本号的 tag 在证明与签名之后才打、Release 在两种架构上核对过才公开（S2），被取消的运行留不下按版本号拉得到的东西，重跑照 3.23。`86293682` 的说明末句写成"取消之后什么也没推"，不严（核对 B 第 1 轮），见"更正"。
  - 同一守卫照 Minor 与 Nit 改的几处（`run:` 里在每一处 `${{` 上核对，P5S1B-M1；actionlint 那一步恰是三个键、`costs-1` 是作业的一份，P5S1C-M3、N6；矩阵的值写死，P5S1B-N8）是发现的处置，列在上表，不另编号。
- **561 `build` 断言它检出的就是闸门放过的提交，交 S2**（审查 B 未核实的第 1 条；主持者在修复第 1 轮之后定；措辞照核对第 2 轮 B 的 N11）：`build` 用 `HEAD` 构建，不看 `needs.gate.outputs.sha`。推断 `actions/checkout` 在 tag 的事件上照 `GITHUB_SHA` 取，两个作业是同一个提交，tag 中途被改指也一样；v7.0.1 的源码没读，只有推 tag 才证明得了。S2 在 `build` 的 Labels 之前加一步 `test "$(git rev-parse HEAD)" = "$SHA"`（`SHA` 经 `env` 取 `needs.gate.outputs.sha`），就不靠这个推断；`publish` 不用 `HEAD` 构建，它的 Index 一步可以另加 `[ "$(label revision)" = "$SHA" ]`，直接核对推上去的镜像。S1 不改：要改 `release.yml`，与 S2 的 `publish`、`verify` 一起写；tag 的作业在 S2 合并之前走不到。
- **562 smoke 一步关掉 `bash -e` 的写法交 S2**（核对第 2 轮 A 的 N6、B 的 N10、C 的 N9，三位都提）：S2 写 `publish`、`verify` 与它们的 `shell` 时，守卫一并不许跑 smoke 的步骤与它们的作业写 `shell`，不许工作流与这些作业写 `defaults.run.shell`，不许这些步骤的 `run` 里有 `set +e`（写法可照 P5S1C-N6 对 actionlint 那一步的整个比）。理由：S1 的守卫防的是无心的改名与挪动（同 P5S1C-N6 的定级）；有意的写法列不尽（`trap`、heredoc、`BASH_ENV`，守卫也挡不全），要与 S2 加的 `shell` 一起看，S1 只写进本记录与设计。主持者看了 B 的 N10 先定"不改、只记"，再看了 A 的 N6（`build` 那一步自带第二行，一行 `set +e` 就让 smoke 的失败变绿）改判为交 S2。
- **563 P5S1B-N2、P5S1C-N1 不改代码**（实施报告"修复第 1 轮"；核对第 1 轮 A、B 判理由站得住）：
  - 闸门的日期只核对 `YYYY-MM-DD` 的形状（`2026-99-99` 放行）：`release-gate.sh` 与设计 3.6 逐字相同；它挡的是"忘了写日期"（决定 434 要防的那一种），日期由主持者在 3.23 第 7 步照当天写；`date -d` 回读是 GNU 的写法，加了也只多挡一种手误。设计 3.6 写明。
  - MCP 的版本测到 `mcpOptions`，`mcpServer` 只剩 `mcpserver.New(mcpOptions(...))` 一行，这一跳由代码的形状守着、不由测试守着（cm1 照过；决定 431 有意这样；02-workflow 3.3"走不到最后一跳"）。

## 实施中定的

实施报告第 7、8 节与修复两轮（三位审查者各在自己的范围里判过，主持者接受）：

- **作业的表写成函数 `releaseJobs()`**：gochecknoglobals 不许全局的 map；设计没有写形式。第 7 个提交第一次跑 `make lint` 时报出，改了之后才提交。
- **第 1 个提交的标题带 `docs`**：照 6.1 写的；它不改文档文件，它的守卫读 README。
- **3.21 的十一个 action** 以 `git ls-remote` 从公开仓库读过 tag，每个 SHA 都是表里写的（`anchore/sbom-action` v0.24.3 的 tag 对象 `fffec3d8…` 解开是 `66cbf4bc…`）；审查 B 以 `gh api` 另核了 `release.yml` 用的七个，CI 日志里 "Download action repository" 的 SHA 也相同。
- **注释**：`Makefile` 测试的总注释补"另盖一个不是默认值的版本号"；`image` 上面的注释补"判断与构建写在同一个 shell 行里"与 label 的来源（修复第 1 轮再补先取出来再构建的原因）；`image-smoke.sh` 的头注释加"镜像的 label"。
- **变异 m1** 写成 `buildinfo.Get().Version[:0] + "0.1.0-dev"`：直接写字面值时 `buildinfo` 没人用，编译不过。
- **工作流的变异**用把备份放在 `~/.cache` 的 `wmut.py`：`mut.py` 的 `*.mutbak` 放在 `.github/workflows/` 里，会被守卫的第一条（目录里恰是三份）当作第四份，第一次跑的 w 组全"抓到"、原因都不对；重跑之后每个由它该红的那一条抓到，顺带证明了"目录里多一个文件"那一条。06-environment 第 5 节加一句。
- **修复第 2 轮**：`c3734c36` 提交之后、推送之前改过一次说明（"k1–k4 现在红"改为"k1–k3 现在红，k4、b9、fn1 照旧红"：k4 改前就红）；`notesChangelog` 那一行照审查 B 的建议改成 `旧的一节并进了下面：### 0.1.0（草稿）`，以"）"结尾，"标题在行中"（n12）仍红。

## 接受与推后的

- **接受**：
  - g6x（只看 `conclusion = success`）照过，与原样等价：GitHub 的接口在运行完成之前 `conclusion` 是 `null`；测试里不造接口不会有的格子（审查 C）。
  - 第 3 条读 `refs/heads/main`（cg3）测试照过：测试的克隆里有本地的 `main`，runner 上检出 tag 时没有，`merge-base` 报错、`\|\| fail` 拒绝每个 tag，失败即拒绝，第一次 rc 就看得见（核对 C 两轮）。
  - 作业级的 `env: {MAKEFLAGS: n}`（cx7、cx8）让整份的 `make` 都不跑，不是"actionlint 那一步"的事；演练把 `${{ vars.… }}` 写进 `~/.docker/config.json`（cs5）还要把 `push: false` 改成 `true`、把可写的令牌放进明文的变量。都照过，不记（核对 C 第 2 轮）。
  - 守卫多报、方向安全的几种：`${{ MATRIX.arch }}`、名字带 `-` 的矩阵键、`${{ '${{' }}`；`run`、`uses` 里出现 "login"，字符串里出现 "secrets"（核对 B 两轮），今天的三份里都没有。cx4（`if: ${{ … }}` 的等价写法）、cx5（`continue-on-error: false`）、cx9 与 cs6（改步骤的名字）也红，偏严、无害（核对 C）。
  - MCP 测试的 c7（`-X` 盖的与环境变量不同）也报"`-ldflags` 没有到"：经 `make` 时两者都取 `$(TEST_VERSION)`，分不开（核对 A 第 1 轮）。
  - `make -i image-smoke` 在 `image` 失败之后照跑 `image-smoke.sh`，可能核对到旧的同名镜像：`-i` 的语义，修复之前就是这样；真的 `image-smoke.sh` 先在 host 上跑 git，git 失败时自己停（核对 A 第 1 轮）。
  - CRLF 的 `CHANGELOG.md` 拒绝，信息"的标题有 0 个"让人摸不着头脑，方向是拒绝（审查 B 的探针 P19）；闸门的第 2 条在 runner 上几乎恒真（`actions/checkout` 把本地的 tag 对齐到 `GITHUB_SHA`，推断），没有害处（审查 B 未核实的第 1 条）。
  - `-format` 不转义 Message 里的 `%`、`\r`、`\n`：带了也只是一条注解断成两行（审查 C 未核实的第 1 条）。
  - 审查 A 的 l2、l3 两次构建 server 阶段是 `CACHED`：共用的构建缓存里有实施者同样内容的构建；smoke 先过了版本核对再红在 label 上，原因对（审查 A 未核实的第 2 条）。
- **推断、没核实的**（只有推 tag 才证明得了的，P5 设计 3.23）：
  - GHCR 里有没有包：三位的令牌都读不了（403）；仓库的 tag 与 Release 都是 0 个，`release.yml` 的运行只在分支上，演练没有登录的一步、`push: false`、artifact 0 个。
  - `ci.yml` 在 tag 上不再触发：照 GitHub 文档；rc 时看 `gh run list --workflow ci.yml --branch v0.1.0-rc.1` 是空的。
  - `gate` 的检出里有 `refs/remotes/origin/main`（`fetch-depth: 0`）：没有时闸门拒绝（审查 B 的探针 P24）。
  - 大写的 `SECRETS.`、`Docker/login-action` 在 Actions 里生效，`shell: true {0}` 让脚本不跑：都是推断；前两种守卫都抓到，后一种交 S2（决定 562）。
- **推后、交出去的**：
  - 决定 561、562 与实施报告 9.4 的三条（跳过的矩阵作业不展开名字；`build-push-action` 在本地写构建记录；根分区）：P5 设计第 9 节"P5/S2"。
  - 合并之后两次 `main` 的组合根、`costs-1` 的两步、`short` 的 Test 一步与 `main` 上的演练（设计 7.5、第 8 节第 1、3 条），主持者在记合并号的文档提交里补。
  - 合并之前看一眼 `vuln.yml` 在 `main` 上最近一次的结论（设计 6.4）。

## 更正

已推的提交说明不改写；不准处写在这里：

- `86293682` 的标题"the rehearsal without secrets or a login"比它的守卫宽（那时只认子串 `secrets.`、只看演练作业，P5S1B-M3）；末句"a tag's run cancelled is not held (nothing is pushed then …)"：取消发生在 `build` 之后时，GHCR 里已经有按摘要推上去的、没有 tag 的摘要（决定 560 写准了）。
- `c3734c36` 的标题"each smoke their image in one step that fails with it"说宽了：守卫按行核对，前一行 `set +e` 或不带 `-e` 的 `shell` 时 smoke 失败了照绿（决定 562）；正文的"nothing after it on the line"照实。
- `ee8f2ab4` 的说明把"上下文名不分大小写"写成了事实，是推断；守卫两种写法都抓到。
- `4ebb781f` 的注释"with nothing to let it fail quietly"说过头（P5S1C-N6），`299953dd` 改为"of its own"。
- 实施报告第 1 节"相对 `5e58e844`：17 个文件，+1,297 −48"是对 `main` 当时的头 `cfa31e00` 比的（多了它的文档）；相对 `5e58e844`，`38916978` 是 13 个文件，+1,288 −21，`ce8aaf64` 是 13 个文件，+1,424 −24。

## 变异与反向对照

- **设计 7.2 里 S1 的各条**（实施者；Go 的用 `mut.py`，工作流的用 `wmut.py`，先在没改过的树上跑同样的命令）：v1–v6、m1–m4、l1–l4、w1–w8、w10–w12、a1、a2、g1–g14、n1–n5、v-gate 都抓到。m1 只由盖了版本号的运行抓到，直接 `go test -short` 照过（设计写明）；等价的 g6x 照过；v-gate、a1、a2 手做，PATH 里放假的 `go`、`pnpm`，不跑 actionlint 与 govulncheck；l1–l4 在普通的克隆里经 `make image-smoke`，l1 在清了构建缓存之后从头构建，171 秒。
- **实施者自加**：v7、w17–w27（`build` 加 `id-token: write`、`gate` 去掉 `actions: read`、`build` 去掉 `needs`、顶层 `read-all`、tags 写 `v*`、闸门与 `make vuln` 对调、`ci.yml` 的作业另写权限、`ci.yml` 顶层 `contents: write`、`vuln.yml` 在 tag 上跑、`build` 的 `continue-on-error`、演练的 `if` 加 `\|\| true`）、g3b（只换端点）、g15–g20、n6–n12 都抓到；其中 g15–g18、g20、n8、n12 在第 8 个提交补了格子之后才抓到（决定 558）。原稿的闸门对着本稿的表，五格失败：两个"这一节是空的"放行，"没有 IMAGE"放行，"没有运行"与"端点答错"两格的消息里没有"单独推上 main"。
- **先红后绿**：`TestMCPServerInfoIsTheBuild` 在没有 `mcpOptions` 时编译不过；只设 `NWIKI_TEST_VERSION`、不盖版本号时它红。
- **审查 A**：默认值六条（a-v1–a-v6）都红；MCP 的 a-m1（盖了版本号）、H-m4（设环境变量、不带 `-ldflags`）、H-m8（盖默认值本身）、H-m9（`-X` 写错符号，链接器悄悄不理）、a-m2、a-m6、a-m7 都红，a-m1 直接 `go test` 照过（预期）；l2–l4 红在 label；直接 `docker build` 不传参数时 smoke 红在 label（版本核对已过）。
- **审查 B**：21 个（b1–b16、gc、gc2、gd）：b4–b4d（P5S1B-M1）、gc（M2）、b9、b10、b14（N3）照过；b16（`needs: [gate]`）是等价的对照；b2、b8 第一次写成了 YAML 的注释，是变异写错了，改写之后红。修复之后 b4–b4d、b9、b10、gc 都红，b14 照主持者的决定不加。
- **审查 C**：7.2 抽 21 条重跑都抓到；自加 14 条里 cw5、cw6（P5S1C-M3）、cg1（M2，即 gc）、cm1（N1）照过。
- **修复第 1 轮**（实施者，`fixspec.py`）：b4–b4d、w8a、w8b、fw1（先一个许可的、再一个不许的）、cw5、cw6、fw2–fw5、b9、fn1、b10、fn2–fn4、gc、g2、fn5 都抓到，其中 b4–b4d、cw5、cw6、b9、b10、gc 改前照过。S1 原有的几组在 `a14d9ee5` 上重跑：闸门、说明与守卫的 58 个抓到 57 个（g6x）；守卫的 25 个、默认值的 14 个都抓到；MCP 的 3 个同前。
- **核对第 1 轮**：A 以八种情形核对 MCP 测试的两条消息（c1–c8），以 13 种情形核对 `make image`；B 新加 18 个，s1–s5、l1、l2（M3）、k1–k3（N6）、gc2（N7）、e4（N8）照过；C 新加 11 个，cx1、cx2（N6）、cs1–cs3（N7）照过，cx6（`working-directory: web`，CI 会红、不悄悄）、cg3 不记。
- **修复第 2 轮**（实施者，`fix2spec.py` 直接读审查者的表）：s1–s5、cs0–cs3、l1、l2、fs1（顶层 `defaults.run.working-directory` 里的 secret）、k1–k3、fk1、fk2、cx1–cx5、fx1、e4、fe1、gc、gc2–gc4、fn5–fn7、n12 都抓到，改前照过的都在其中；第 1 轮的 22 个与 S1 原有的几组照旧（g6x 等价；n12、fn5 的原文随 `fb3101df` 改了，在新表里重写）。
- **核对第 2 轮**：A 的 a2-k1–a2-k5、B 的 k5–k10 与 gx1、C 的 k6、k7 照过（P5S1A-N6 = B-N10 = C-N9，决定 562）；C 的 cx7、cx8、cs5 照过、不记（"接受与推后的"）；以前的都照旧（B 重跑的 41 个里，b2、b8 的错写法、b14、b16 照过，fn5 改写成 fn5b 之后红）。
- 每次跑完工作树干净，没有 `*.mutbak`。

## 核对

- **核对第 1 轮**（头 `a14d9ee5`）：第一轮的 19 条关上 18 条，P5S1B-N1 关了一半；P5S1B-N2、C-N1 不改的理由站得住。新的 Minor 1（P5S1B-M3，C 的 N7 同一条）、Nit 9。A 在普通的克隆里重跑 `make image-smoke`（label 逐字对，`created` 等于 `git log -1 --format=%cI`）；B 重跑第一轮的 23 个变异与 37 种探针，插桩逐格记下 HEAD；C 重跑实施者的 22 个与自己的 27 个，抽三个提交（`bc71d5fe`、`4ebb781f`、`719c59de`）逐个跑 6.1 那一组，都是 0。
- **核对第 2 轮**（头 `ce8aaf64`）：上一轮的都关上（A 的 2 条、B 的 6 条、C 的 3 条），第一轮 N1 余下的一半随 N5 关上。新的只有 Nit：P5S1A-N6、B-N10、C-N9 是同一条（决定 562），B-N11 在本提交。A 在普通的克隆里再跑 `make image-smoke`；B 重跑以前的 41 个与 37 种探针；C 抽两个提交（`299953dd`、`fb3101df`）逐个跑 6.1 那一组，都是 0。
- 两轮之后没有 Important、Minor 留下，不开第三轮（02-workflow 3.4）。

## 门禁与 CI

- **逐个提交**（6.1 那一组：`go build ./...`、`go vet ./...`、`make lint`、`make test-go-short`、`make gen-check`、`make knip`，之后核对没有已跟踪的改动；经 `heavy.sh`，在干净的 worktree 里逐个 `checkout --detach`）：实施的 9 个都是 0（`make test-go-short` 46–47 秒），修复第 1 轮的 8 个（40–42 秒）、第 2 轮的 7 个（40–41 秒）都是 0。审查 C 另跑了实施的 9 个，核对 C 抽了修复的 5 个，都是 0。不改网页，没有逐个跑 `make test-web`。actionlint 不在本机的门禁里（决定 435）：第 5 个提交本机为绿，CI 上红在它（决定 557）。
- **推送之前**（经 `heavy.sh`，每项之前判 `df`）：

  | 头 | 门禁 | 结果 |
  |---|---|---|
  | `38916978` | `make lint gen-check knip build`；`make test-go-short`；`make image-smoke`（普通的克隆）；`make test-go`、`make test-web`、`make e2e` 与 l1–l4（同一次 `heavy.sh`） | 都是 0：10 秒；52 秒（`main` 的 `cfa31e00` 45 秒）；22 秒，"通过（版本 0.0.0-p5s1，提交 389169786e57…，modified=false）"，label 逐字对；`make test-go` 185 个包、3,575 秒，`make test-web` 195 个文件、4,694 个，`make e2e` 576 个通过、1 个照旧跳过 |
  | `a14d9ee5` | `make lint`、`make test-go-short`；`make image-smoke`（`Makefile` 动了） | 都是 0：94 个包；18 秒 |
  | `ce8aaf64` | 同上（`deploy/` 的脚本动了） | 都是 0：94 个包；18 秒 |

  修复两轮只改测试、两个脚本与 `Makefile` 的配方与说明，没改网页与组合根，`make test-go`、`make test-web`、`make e2e` 照修复说明不重跑，CI 上整套跑过。
- **根分区**：实施中两次降到 15G 之下，照规矩停下：第一次负责人确认之后，主持者清了 Docker 的构建缓存与悬空的卷（13G → 26G）；第二次清了 Go 的构建缓存（14G → 54G）。
- **分支的 CI**（`ci.yml` 19 项、`release.yml` 两个演练；有结论的每次都是第一次 attempt，没有重跑）：

  | 推送 | 头 | `ci.yml` | `release.yml` | 备注 |
  |---|---|---|---|---|
  | 1 | `b937d820` | 44068674790：costs-1 红在 Lint workflows，其余被下一次推送取消 | — | actionlint 拒绝 `-format` 的模板（"unterminated quoted string"，exit 3）；注解只有 "Process completed with exit code 2"（另一条是容器诊断的 warning）。不是"允许的 actions"挡的 |
  | 2 | `2db96b0a` | 44069983898：Lint workflows 绿，整次被下一次推送取消 | — | 现有的两份工作流过了 actionlint |
  | 3 | `96bf93e1` | 44073973273：19 项全绿 | 44073973297：两个演练绿，`gate`、`build` 跳过 | `docker/setup-buildx-action`、`docker/build-push-action` 没被"允许的 actions"挡住；Lint workflows 读三份、0 错 |
  | 4 | `38916978` | 44077699226：19 项全绿 | 44077699224：两个演练绿 | 实施的头 |
  | 5 | `a14d9ee5` | 44164542578：19 项全绿 | 44164542691：两个演练绿 | 修复第 1 轮；`vuln.yml` 44164542725，success |
  | 6 | `ce8aaf64` | 44177949348：19 项全绿 | 44177952360：两个演练绿 | 修复第 2 轮；没改 `Makefile` 与依赖清单，没有 `vuln.yml` 的运行 |

  - 演练每次两份：`uname -m` 是 `x86_64`、`aarch64`，"The image is the runner's architecture"都过；`GITHUB_TOKEN` 只有 `Contents: read`（另有 Metadata）；没有登录的一步，Build 是 `push: false`；BuildKit 按钉住的摘要拉，`BuildKit version: v0.34.2`；构建记录只写在本地（约 70–74 KB），artifact 0 个，注解 0 条；最后一行都是"image-smoke: nervewiki:rehearsal 通过（版本 0.0.0-rehearsal.<n>，提交 <头>，modified=false）"。日志里的 "login" 只出现在 build-push-action 打出的 GitHub 上下文里（提交说明与用户名）。
  - Lint workflows 每次 2–3 秒：`go: downloading github.com/rhysd/actionlint v1.7.12`，"Collected 3 YAML files"（`vuln.yml`、`ci.yml`、`release.yml`），"Found 0 errors in 3 files"；shellcheck 照设计关掉，pyflakes 不在 runner 上。
  - 组合根八份的 Test 一步：`38916978` 231–384 秒，`a14d9ee5` 238–345 秒，`ce8aaf64` 245–388 秒，都在 450 秒之下。各份与用时在 [P5 设计](../05-P5-deploy-release.md) 10.1。
  - `vuln.yml` 另在 `b937d820`、`2db96b0a`（改了 `Makefile`）上各跑一次，success；`main` 上最近一次（`cfa31e00`，schedule）success（审查 C）。
- **合并 `main`**（`b34e7ac3`）：`0918ad73` 带来的只有 `docs/`、L1 的故事与它的单元测试，没有冲突；它与本提交一起推送，分支 CI 是本提交的。
- **本提交的分支 CI**：（待填：本提交的分支 CI）。
- **没有推任何 tag、镜像，也没有建 Release**（负责人 2026-11-17 第 11 问：先跑 S1 的分支 CI 看）：tag 上的作业在分支上都跳过；仓库的 tag 与 Release 都是 0 个（三位审查者各核过）。`docker/login-action` 只在 tag 的 `build` 里，还没跑过；它与 S2 要的 `sigstore/cosign-installer`、`anchore/sbom-action` 和写权限，推 rc 之前照 00 号文档"需要负责人确认"第 2 项核对。

## 环境

实施、审查与核对各在自己的工作树里：实施者 `nerve-wiki-m12-p5-s1`（逐个提交的核对在 `~/.cache/m12-p5-s1/wt-verify`，镜像与 l 组在同一处的普通的克隆里，用完删了）；审查与核对 A `nerve-wiki-m12-p5-rev-a`、B `nerve-wiki-m12-p2-rev-b`、C `nerve-wiki-m12-p4-rev-c`。`TMPDIR`、`GOTMPDIR`、日志与假的 `go`、`pnpm`、`gh`、`git`、`docker` 在 `~/.cache` 下各自的目录，用完删了。本机没有跑 actionlint 与 govulncheck；`gh` 只读（运行、作业、日志、注解、artifact、tag 与 Release）；没有推送 tag 与镜像，没有登录。镜像都按名字删了，smoke 的容器由脚本的 trap 清掉；共用的 Docker 构建缓存与 Go 的构建缓存只在负责人确认之后由主持者清过（"门禁与 CI"）。本机 aarch64、8 vCPU。要留的在设计工作目录的 `m12/p5s1/`（`impl-report.md`、`tools/`、`msg/`、`review/`）；实施者的日志与变异表在 `~/.cache/m12-p5-s1/logs`，合并之后删。
