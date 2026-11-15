# M12/P1 S2（工具链与安全更新）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m12-p1-s2`。实施的 12 个提交，从 `main` 的 `0da2fb5d`（S1 合并之后）开始，审查时在 `4a0c2585`：<br>Go 1.27.4、Dockerfile 的断言与守卫 `6dc814ea`；PostgreSQL 18.7 五处与守卫加 README `69de9e08`；node 与 distroless 的摘要 `a7987bd0`；setup-node 7.1.0、upload-artifact 7.0.2 `597b0729`；`make vuln`、`make outdated`、`tools/outdated.sh`、`vuln.yml` `ed5c1f56`；<br>`vuln.yml` 第一次运行报出的：`klauspost/compress` 1.20.2 `63de975d`、`moby/go-archive` 0.3.0 `0f61a57f`、dompurify 3.4.17 `f3e76505`、katex 的公告进 `audit.ignore` `c654db5f`（决定 353）；<br>`vuln.yml` 不取不存 Go 的缓存 `2811d13d`；第三、第五份各挪一个给第八份 `28980104`（决定 354）；两处注释比的"S1 之前"改为"S2 之前的 `main`" `4a0c2585`。<br>修复一轮在它之上 6 个提交，头 `80d76191`：竞态的九份才用 Go 的缓存 `0844464d`（决定 355）；守卫读整行的断言 `42e6ff6f`；拒绝与干活同一行、`outdated.sh` 自己拒绝 `e8b115c9`；`outdated.sh` 只对读到的写"没有过时的"、写出出错的命令 `d9ff11f6`；`vuln.yml` 的 `paths` 加 `package.json` `f480a9e5`；第八份的帮助文字 `80d76191`。<br>分支共 18 个提交，19 个文件，加 452 行、减 37 行；没有改产品的行为（两个 Go 模块只在测试里，dompurify 是 mermaid 底下的补丁版本） |
| 审查方式 | 三位审查者（Opus）并行，各在自己的分离的工作树里（挪到 `4a0c2585`），重的都经 `heavy.sh`，不碰实施者的工作树；联网只照决定 308（读 CI 的作业日志、`git ls-remote`、`docker buildx imagetools inspect`），没有跑 `govulncheck`、`pnpm audit`、`pnpm outdated`：<br>A 工具链、镜像与 Go 模块：守卫的变异（r1–r10、x 系列），摘要与 SHA 重读，`go list -deps ./cmd/nervewiki`，go.dev 的发布说明。<br>B 分份、缓存与 CI：S2 之前两次 `main` 与分支五次 `ci.yml` 的 119 个 server 作业日志（`b/analyze.py`：每份取回的键、"Cache Size"、Test 一步、包的 `ok` 行、下载、post 一步存没存），`gh cache list`，本机竞态与不带竞态的缓存实验，Go 自己的匹配器核分份，三份本机重量，五个提交逐个 `make lint`、`make test-go-short`。<br>C `make vuln`、`make outdated`、`vuln.yml`、npm 与文档草稿：设计 3.12 与代码逐字对照，五个桩（`go`、`pnpm`、`gh`、`docker`、`curl`，每次 `type -P` 先核对）下的各种情形与变异，`vuln.yml` 五次运行的日志，实施报告第 13 节的草稿 |
| 日期 | 2026-11-15（实施、审查、修复一轮、修复核对一轮） |
| 结论 | Important 1：合并之后 `main` 的 Go 缓存键会由不带竞态的 `short` 存下，组合根各份照旧每份从头编译（P1S2B-I1）。Minor 4：守卫不核对断言报错里写的版本（P1S2A-M1）；报告与提交说明把"没取回缓存"说成"取回 3 MB"（P1S2B-M1）；`make -i` 绕过本机的拒绝（P1S2C-M1）；`outdated.sh` 一项也没读到时照样写"没有过时的"（P1S2C-M2）。Nit 11，Q 1。三位都确认：挪份每个顶层测试恰在一份；摘要与 SHA 相同；两个 Go 模块不在二进制里；变异 9–15 与实施者加的都抓到；抽查的提交各自为绿。代码一侧由修复一轮 6 个提交改，文档一侧在本提交。修复核对一轮：三位都没有 Important、Minor，第一轮的发现都关上；新的 Nit 一条（P1S2C-N7，只在文档），在本提交 |

## 审查的发现与处置

编号照审查者的报告。审查时的代码位置是 `4a0c2585` 的。修复一轮照主持者的修复说明（`p1s2/review/fix-1.md`）在 `4a0c2585` 之上加 6 个提交，普通推送、不改写已推的历史，每个单独为绿；提交与结果见"修复一轮"与"核对"两节。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P1S2B-I1 | 重要（CI 的缓存） | 实施者以为分支上组合根在 EPYC 7763 上多出的约 50 秒是 `vuln.yml` 先存下的 3 MB 缓存带来的，`2811d13d` 让 `vuln.yml` 不存，合并之后"由 `ci.yml` 的一份存，同 S1 之前"。不对：慢的是"竞态的份取回的缓存里没有竞态的编译结果"。S2 之前 `main` 的 254 MB 键是 2026-10-17 的 `bootstrap-1` 存的（那时还没有 `short` 一份）；S2 改了 Go 与 `go.sum`，七次运行里 16 份中最先跑完的都是不带竞态的 `short`，合并之后由它存新键；本机实验：不带竞态填满的缓存对竞态构建零收益（22.7 对 22.9 秒）。不改的话，合并之后每一次 `main`，7763 上第一、四份稳态约 443–454 秒 | 主持者的决定 355：`ci.yml` 的 server 矩阵只让竞态构建的九份（`race`、`bootstrap-1`…`8`）取存 Go 的缓存，另外七份不取不存；`vuln.yml` 照旧 `cache: false`，注释改成同一个道理（`0844464d`）。主持者推修复之前删了分支上 3 MB 的键（id 10539544808），修复的那次 CI 是冷的：`bootstrap-2` 是第一个跑完的竞态份，存下 269 MB 的键，`short` 没有存。核对 B：关上 |
| P1S2B-M1 | 中（报告与说明） | 报告第 7、8 节与 `2811d13d` 的说明写"五次的组合根都取回 3 MB"：第一、二次每份都是 "Cache is not found"，第三到五次才取回 3 MB，Test 减包两种一样；`vuln` 一项在分支上一次也没存过 | 已推的说明不改写，`0844464d` 的说明写一句更正；本提交照实写（P1 设计 3.16 第 2 条、10.2）。核对 B：关上 |
| P1S2A-M1 | 中（守卫） | 守卫第 3 条的正则停在 `\|\|`，只比较 `= (go\S+)`，同一行报错里的"go.mod 要 go1.27.4"不在核对之内（变异 r1 照过）：下次只升比较的版本、漏了报错里的，构建报出自相矛盾的一句 | 照 A 的第一种（`42e6ff6f`）：第 2、3 条合成一个跨行的正则，读到整行，tag、比较的版本、报错里写的版本都与两份 `toolchain` 比较；Dockerfile 不变。核对 A：关上 |
| P1S2A-N1 | 细节（守卫） | 守卫不管断言在哪个阶段、第几行（r2 挪到 `go mod download` 之后、r3 挪进 web 阶段都照过），决定 348 的位置没守住 | 同上一个提交：跨行的正则要 `FROM golang:… AS server` 之后只许注释行、接着就是断言；另核对整个 Dockerfile 恰有一行断言（否则合并之后会放过"断言写两遍"）。核对 A：关上 |
| P1S2A-N2 | 细节（守卫） | README 一条只写个数（变异 12 看不出 README 写的是 1.27.1）；`only` 匹配到几行时不写是哪几行 | 同上：README 用 `指定的 Go (\S+?)。` 取出写着的版本放进报错；`only` 多于一个时列出每个匹配的分组，一个也没有时列出以给定前缀起头的行与行号。核对 A：关上 |
| P1S2A-N3 | 细节（说明） | 已推的 `6dc814ea` 说 1.27.2–1.27.4 修的 `html/template` 在服务端的路径上：二进制不链接它（`go list -deps ./cmd/nervewiki` 里是 `text/template`）；`63de975d` 说"二进制两个都不链接"：二进制链接 goose 的 11 个包，不链接的是 `klauspost/compress` | 不改写已推的说明；P1 设计 3.16 第 8 条与 10.2 照实写，10.2 加一句更正（1.27.2 修了 crypto/tls、html/template、net/http、net/textproto、os 与 go 命令，1.27.3 修了 crypto/x509，1.27.4 修了 cgo、go 命令、编译器、链接器与 net/http）。核对 A：关上 |
| P1S2B-N1 | 细节（文档稿） | `06-environment.md` 草稿的"平常约 33 秒"只是 7763 的（6975P 22–25，9V45 21–22，9V74 26） | 本提交写"7763 上约 33 秒（别的 CPU 约 21–26 秒）"。核对 B：关上 |
| P1S2B-N2 | 细节（帮助文字） | `Makefile:402` 第八份的帮助文字两次写"第三份让出的" | 并成一句（`80d76191`）；分份不变。核对 B：关上 |
| P1S2B-Q1 | 问 | 要不要删分支上的 3 MB 键，在分支上就核对 I1 的修正 | 主持者删了（id 10539544808），修复的那次 CI 由竞态的份存键；暖的读数在本提交自己的分支 CI（"门禁与 CI"） |
| P1S2C-M1 | 中（拒绝） | `@$(ci-only)` 单独一行，`make -i vuln`、`MAKEFLAGS=i make vuln`、`make -i outdated` 忽略它的失败、接着跑下一行：桩下两个、十四个调用都发生了；直接执行 `tools/outdated.sh` 也不拒绝 | 照 C 的第一种（`e8b115c9`）：判断与干活写进同一个 shell 行（`@$(ci-only); \` 接干活的行，`outdated` 写成 `@$(ci-only); tools/outdated.sh`），`tools/outdated.sh` 自己也判 `GITHUB_ACTIONS`。核对 C：关上 |
| P1S2C-M2 | 中（`outdated.sh`） | 第 4、5 节只看有没有列出过时的：`gh`、`docker` 都失败时照样写"没有过时的（5 处引用）""（3 个镜像）"；"5 处引用"其实是 5 个不同的 action | `d9ff11f6`：每节数总数与读到的，全读到才写"没有过时的（5 个 action）"，读到一部分写"没有过时的（读到 k 个，共 n 个）"，一个也没读到只有 `!!` 行。核对 C：关上 |
| P1S2C-N1 | 细节（`outdated.sh`） | 管道在 `pipefail` 下报的是 jq 的 4，不是 curl 的 22、7 或 docker 的 1 | 同上：先取、再解析，`!!` 行写出是哪个命令与它的退出码。核对 C：关上 |
| P1S2C-N2 | 细节（`vuln.yml`） | `paths` 不含根 `package.json`，它的 `packageManager` 定了跑 `pnpm audit` 的 pnpm | 加上（`f480a9e5`），注释写明。核对 C：关上 |
| P1S2C-N3 | 细节（文档稿） | 第 2 节读的是 `go.dev/dl/?mode=json&include=all`，设计写 `?mode=json` | 3.16 第 5 条写明。核对 C：关上 |
| P1S2C-N4 | 细节（文档稿） | 草稿说每个提交都在干净的 worktree 里为绿：`4a0c2585` 只在实施者的工作树里跑了 `make lint` 与份的核对 | 10.2 照实写。核对 C：关上 |
| P1S2C-N5 | 细节（文档稿） | 决定 353 的理由写"`:122` 与 `:318` 都把我们的选项展开在后面"：是 `:123`（`{ ...given, ...options }`）；`:318` 是 `{ ...options, displayMode: displayed }` | 00-M12 第 11 节照实写。核对 C：关上 |
| P1S2C-N6 | 细节（文档稿） | 05-lessons 草稿说 setup-go 的键"只看 Go 的版本与 `go.sum`"：键里还有系统、架构与 runner 的镜像，哈希是 `cache-dependency-path` 所列文件的 | 05-lessons 与 3.16 第 2 条照 C 写。核对 C：关上 |
| P1S2C-N7 | 细节（文档，修复核对） | `tools/outdated.sh` 自己拒绝是新的检查，它的反向对照（变异 15d、15e）与 15c、split 没进 3.14 的表与 7.2 | 本提交：3.14"在本机拒绝"一行加"`tools/outdated.sh` 自己拒绝：直接执行一次；变异 15d、15e"，7.2 加 15b–15e、split 与 x4、x5 |

## 主持者的决定

负责人可以改判：

- **353 katex 的 GHSA-238p-pmpm-9mq7 写进 `audit.ignore`**（`c654db5f`）：`math.ts:44` 的 `trust: false` 是选项对象自己的属性，`:123` 把 mermaid 给的选项放在前、我们的在后（`{ ...given, ...options }`），`:318` 只有我们的选项与 `displayMode`（`{ ...options, displayMode: displayed }`），继承来的 `trust` 盖不过它；公告剩下的风险要先有别的原型污染；0.16 线没有修，升 0.18 是 00-M12 4.3 定了不升的功能性大版本、补丁要重做；发布的闸门用 `--audit-level=high`，本来就不挡它。`vuln.yml` 的下一次运行（43664040257）为绿，"1 ignored: 1 low"，是键写对了的证据。katex 0.18/0.19 与补丁重做，收尾时写进 v0.2 的 `M12-release.md`。
- **354 审查之前就在 S2 里挪份**（`28980104`）：照 M11 决定 272、276 的先例。主持者的说明以"带竞态的各份慢了 5–14%"为前提，实施者查下来慢的是编译（I1 之后照实：竞态的份没取回竞态的编译结果），照 7763 上包的时间折算，只有第三、第五份过约 400，各挪一个给第八份，不分第九份。
- **355 `ci.yml` 的 server 矩阵只让竞态构建的九份用 Go 的缓存，另外七份不取不存；`vuln.yml` 照旧 `cache: false`**（`fix-1.md` 第 4 条，P1S2B-I1）：七份今天取回的竞态缓存本来就没给它们省下编译，只少约 2 秒的模块下载；另给它们一个键改动大，收益不在 450 的线上。
- 推修复之前，主持者删了分支上 3 MB 的 Go 缓存键（id 10539544808，P1S2B-Q1），所以修复的那次 CI 是冷的。
- 修复核对之后不再开第二轮：三位都没有 Important、Minor；新的 C-N7 在本提交改文档。

## 实施中定的

实施报告第 11 节第 6 条（三位审查者各在自己的范围里判过）：

- `outdated.sh` 的输出格式与出错的认法（每节一行 `== ` 标题，出错打 `!! ` 一行、照样往下，末行 `outdated: <n> sections, <k> with errors`）。修复一轮另定：只对读到的写"没有过时的"，取与解析分开跑。
- Go 一侧两个只在测试里的模块照样升，不当作"与发布物无关"留着：补丁或 v0.x 的小步，`go mod tidy` 只多动 `moby/sys/user` 一个补丁版本；`govulncheck ./...` 本就扫 `pgtest`，留着它每天的运行就红。
- 两个 Go 模块各一个提交，dompurify 三个公告一个提交。
- ~~`vuln.yml` 不存 Go 的缓存，不改 `ci.yml` 的缓存~~：后一半**作废**，由决定 355 取代（P1S2B-I1）；前一半照留，理由改为"键归先存的那一份，竞态与不带竞态的编译结果不共用"。挪份照修了缓存之后的数，只挪两个、不分第九份，照留（以决定 355 为前提，核对 B 判成立）。
- 挪份挑稳定的、不是所在份第一个跑的，不挑两种用时的 TestTheMCPBatchIsRefusedWhole（同 M11 第 276 条）。

## 接受与推后的

- **接受**：
  - 修了的最小版本 `klauspost/compress` v1.20.2、`moby/go-archive` v0.3.0 取自 govulncheck 的 "Fixed in"，没有直接读 vuln.go.dev（审查 A 的未核实的怀疑）；testcontainers-go v0.44.0 配 go-archive v0.3.0 由分支 CI 的竞态与成本几份证明。
  - 3.16 第 7 条"`pnpm update -r <包>@<版本>` 被 pnpm 12.8.1 拒绝"没有复现（要连 registry）；`pnpm outdated` 以 1 退出、什么也不打印时第 3 节会写"没有过时的"，真的 pnpm 会不会这样没有核（审查 C）。
  - katex 公告的残余只沿代码走过（`trust` 没有自有的 `default` 与 `processor`，我们传了自有的 `trust: false`），没有在浏览器里造原型污染（审查 C）。
  - 守卫对 CRLF、制表符、`FROM` 后空一行、注释缩进、行尾空格都报错（失败在安全的一边；`.gitattributes` 是 `eol=lf`），照设计 3.9 的原文逐字钉住（核对 A）。
  - `costs-3` 末行 `go -C server/tools test -race` 不取缓存，约多 10 秒（26.9、22.3 → 36.1、33.4 秒），作业 16–18 分钟、时限 40 分钟（核对 B）。
  - 变异 15c（只去掉 `Makefile` 的判断）从行为上看不出：`outdated.sh` 第二层守住；15e 证明两层都去掉才会调到（核对 C）。
  - p1、p1b（挪份漏了老规矩）只由实施者的 `shards.py` 抓到，仓库里没有核对分份的测试（同 M11 挪份时）。
- **推后、交出去的**：
  - 合并之后第一次 `main` 是冷的（新的 Go 与 `go.sum`），由第一个跑完的竞态份存键；第二次起竞态的份取回它，7763 上 Test 减包应回到约 33 秒。看那一份 post 一步的 "Cache saved with the key"，并看 `short` 没有存；决定 352 的那一次若是冷的第一次，减去约 50 秒的编译再比。
  - 开发库：PostgreSQL 18.x 之间不要 dump/restore，`make dev-db` 拉 `postgres:18.7-trixie`、沿用旧的卷即可（18.7 的发布说明；本机没试）。
  - katex 0.18/0.19 与补丁重做、Node 26、功能性的大版本与 Dependabot 的版本更新，收尾时写进 v0.2 的 `M12-release.md`（决定 353）。

## `vuln.yml` 报出的与 `outdated` 的输出

| 运行 | 头 | `vuln` | `outdated` |
|---|---|---|---|
| 43657896140 | `ed5c1f56` | 失败 0:51：Go 2 个、npm 4 个 | 0:42，六节、0 with errors |
| 43660640444 | `f3e76505` | 失败 0:41：Go 0 个；npm 只剩 katex（low） | 0:46 |
| 43664040257 | `c654db5f` | **为绿** 0:50：Go "No vulnerabilities found"；npm "1 ignored: 1 low" | 0:36 |
| 43672581385 | `28980104` | **为绿** 0:53 | 0:57，六节逐字同第一次 |
| 43676717960 | `4a0c2585` | **为绿** 0:51 | 0:43 |
| 43681898372 | `80d76191`（修复之后） | **为绿** 0:44 | 0:47："没有过时的（5 个 action）""（3 个镜像）"，`0 with errors` |

- **Go 一侧**（第一次，`govulncheck` 退 3）：GO-2026-6865 `github.com/klauspost/compress` v1.19.2（修在 v1.20.2），GO-2026-6253 `github.com/moby/go-archive` v0.2.0（修在 v0.3.0）；另 3 个在导入的包、6 个在要求的模块里调用不到。两个都是间接依赖，只经 testcontainers-go 到 `pgtest`；`go list -deps ./cmd/nervewiki` 里没有它们，发出的二进制不含。都不在"带连带的"四个（River、x/text、pgx、goose）之内，照 3.13 升到修了的最小版本；`moby/go-archive` 另带 `moby/sys/user` v0.4.0 → v0.4.1。
- **npm 一侧**（`pnpm audit --prod --audit-level=low` 退 1；不装 `node_modules` 也跑得了，3.12 的疑问由此答了）："4 vulnerabilities found, Severity: 2 low | 1 high | 1 critical"：dompurify（`web>mermaid>dompurify` 锁着 3.4.16）的 GHSA-cg2g-xh64-v4q7（critical）、GHSA-mrv2-3pqr-j5jr（high）、GHSA-c6gr-qvv4-5v3f（low），都修在 3.4.17：`pnpm update -r dompurify@3.4.17` 被拒（"is not a direct dependency"），不带版本的 `pnpm update -r dompurify` 在 mermaid 声明的 `^3.3.3` 之内取满一天的最新 3.4.17（3.4.18 不满一天），锁文件只改 dompurify 三处；katex（`web>katex` 与 `web>mermaid>katex`，0.16.47）的 GHSA-238p-pmpm-9mq7（low，修在 0.18.2）照决定 353 进 `audit.ignore`。
- **`outdated` 一项的输出**（43657896140，设计 7.1 要贴出；之后各次六节相同，修复之后第 4、5 节照新的写法）：

  ```
  == 1. Go 模块（server 与 server/tools 的直接依赖）
  server: github.com/getkin/kin-openapi v0.149.0 -> v0.153.0
  server: github.com/jackc/pgx/v5 v5.11.0 -> v5.13.0
  server: github.com/knadh/koanf/providers/env/v2 v2.0.1 -> v2.0.2
  server: github.com/knadh/koanf/v2 v2.3.7 -> v2.4.1
  server: github.com/oapi-codegen/runtime v1.7.0 -> v1.9.0
  server: github.com/pressly/goose/v3 v3.28.0 -> v3.30.1
  server: github.com/riverqueue/river v0.47.0 -> v0.51.2
  server: github.com/riverqueue/river/riverdriver/riverpgxv5 v0.47.0 -> v0.51.2
  server: github.com/riverqueue/river/rivertype v0.47.0 -> v0.51.2
  server: github.com/testcontainers/testcontainers-go v0.44.0 -> v0.45.0
  server: github.com/testcontainers/testcontainers-go/modules/postgres v0.44.0 -> v0.45.0
  server: github.com/yuin/goldmark v1.8.6 -> v1.8.7
  server: go.yaml.in/yaml/v3 v3.0.5 -> v3.0.6
  server: golang.org/x/crypto v0.57.0 -> v0.59.0
  server: golang.org/x/net v0.59.0 -> v0.62.0
  server: golang.org/x/sync v0.23.0 -> v0.25.0
  server: golang.org/x/term v0.46.0 -> v0.48.0
  server: golang.org/x/text v0.42.0 -> v0.44.0
  server: golang.org/x/tools v0.50.0 -> v0.52.0
  server/tools: github.com/getkin/kin-openapi v0.142.0 -> v0.153.0
  server/tools: github.com/oapi-codegen/oapi-codegen/v2 v2.8.0 -> v2.8.2
  server/tools: go.yaml.in/yaml/v3 v3.0.4 -> v3.0.6

  == 2. Go 工具链（server/go.mod 的 toolchain 与 go.dev 上同一小版本线的最新）
  go1.27.4：没有过时的

  == 3. npm 包（pnpm outdated -r）
  （31 个，其中 dev 17 个；每个三行。web 的 dependencies 里：@codemirror/autocomplete 6.20.3 => 6.20.4、
  @codemirror/merge 6.12.2 => 6.12.3、@codemirror/view 6.43.13 => 6.43.15、@codemirror/language 6.12.4 => 6.13.1、
  @lezer/markdown 1.7.2 => 1.8.1、katex 0.16.47 => 0.19.1、lucide-react 1.48.0 => 1.72.0、mermaid 11.17.2 => 12.3.0、
  mobx 7.0.5 => 7.1.7、mobx-react-lite 5.1.0 => 5.1.3、radix-ui 1.6.7 => 1.8.1、react 19.3.0 => 19.3.1、
  react-dom 19.3.0 => 19.3.1、react-router 8.4.0 => 8.5.1；全文在实施报告的 logs/outdated-run1.txt）

  == 4. actions（.github/workflows/ 里按 SHA 引用的与它们最新的发布）
  没有过时的（5 处引用）

  == 5. 基础镜像（deploy/Dockerfile 的摘要与 tag 现在指向的）
  没有过时的（3 个镜像）

  == 6. Node 与 PostgreSQL 的补丁版本（.node-version 的大版本；pgtest 的镜像）
  Node 24：线上最新 v24.22.3（镜像里的 Node 跟着第 5 节的 node 摘要）
  PostgreSQL 18.7：没有过时的

  outdated: 6 sections, 0 with errors
  ```

  第 1、3 节列的都不是 `vuln.yml` 报出的，不在 S2 升；River、pgx、goose、x/text、mermaid 12、katex 0.19 是 00-M12 4.3 不升的。

## 反向对照

工具：守卫的 `mut.py`（`NW_SERVER` 指向工作树，从 `.mutbak` 复原），拒绝与 `outdated.sh` 用 PATH 里的五个桩（每次先 `type -P` 核对，`unset GITHUB_ACTIONS MAKEFLAGS BASH_ENV ENV`，另设 `GOPROXY=off`、`GOTOOLCHAIN=local`、`GOVULNDB=file:///nonexistent`、`npm_config_registry=http://127.0.0.1:9/`），什么也没发出。先在没改的树上跑同样的命令：守卫与 `TestContainerMatchesTheOtherDatabases` 都 PASS，`make image` 为绿。之后 `git status` 干净、没有 `*.mutbak`。

**实施时**（`4a0c2585` 之前，设计 7.2 的 9–15 与实施者加的）：

| # | 变异 | 由什么抓到 |
|---|---|---|
| 9 | Dockerfile 的 golang tag 改回 1.27.1 | 守卫："the server stage is golang:1.27.1-trixie, want the modules' toolchain go1.27.4" |
| 10a、10b | `server/go.mod`、`server/tools/go.mod` 各自改回 1.27.1 | 守卫：10a 另四处各报一行；10b "server/tools/go.mod: toolchain go1.27.1, want go1.27.4 as in server/go.mod" |
| 11 | 断言比较的版本写 1.27.1 | 守卫："the server stage asserts go1.27.1" |
| 12 | README 的 Go 改回 1.27.1 | 守卫（修复之前只写个数，修复之后写出 README 写着的 1.27.1） |
| 13 | server 阶段的摘要换成原来的 `golang:1.27.1-trixie` 的 `3b77fc61…`（tag 与断言照旧） | `make image` rc=2，1 秒："golang 镜像是 go1.27.1，go.mod 要 go1.27.4：换摘要"，停在 `[server 2/9]` |
| 14 | README 的 PostgreSQL 改回 18.6 | `TestContainerMatchesTheOtherDatabases` |
| 15 | `ci-only` 的判断去掉 | 两个桩都被调到；原样时不被调到、rc=2；原样加 `GITHUB_ACTIONS=true`：两侧都跑，任一侧失败 rc=2，都 0 时 rc=0 |
| x2、x3 | 去掉断言；断言写两遍 | 守卫 |
| x4、x5 | `vuln` 的 Go、npm 一侧失败不计 | `GITHUB_ACTIONS=true` 时 rc 0（原样 2） |
| x6a–d | `pgtest.go`、`compose.dev.yaml`、`db.ts`、`image-smoke.sh` 各自改回 18.6 | 已有的守卫 |
| x7 | README 的那一句写两遍 | 守卫 |
| p1、p1b | 挪份的两个名字不进第七份的模式；第三份的 `-skip` 去掉第七份 | `shards.py` rc=1（两份都跑） |

**修复一轮**（"之前"在修复的上一个提交，守卫、`Makefile`、`outdated.sh` 与 `4a0c2585` 相同；"之后"在修它的提交）：

| # | 变异或情形 | 之前 | 之后 |
|---|---|---|---|
| r1 | 只改报错里的版本（`go.mod 要 go1.27.1`） | **活** | 死："the assertion's message says go.mod asks for go1.27.1, want the modules' toolchain go1.27.4" |
| r2 | 断言挪到 `RUN go mod download` 之后 | **活** | 死：0 个匹配，列出 `FROM` 与 `RUN test` 两行与行号 |
| r3 | 断言挪进 web 阶段 | **活** | 死：同上 |
| x7b | README 另有一处"指定的 Go 1.27.1。" | **活** | 死："2 matches …; found ["1.27.4" "1.27.1"]" |
| x11 | `FROM … AS server` 之后先插一行 `WORKDIR /go` | **活** | 死 |
| 9–12、x2、x3、x10 | 同上 | 死 | 死，报错写出找到的 |
| r6–r8 | 另有 `golang:1.27.1 AS tools` 的阶段；注释里写旧的；`go.mod` 注释写旧的 `toolchain` | 不报 | 不报 |
| `make -i vuln`、`MAKEFLAGS=i make vuln` | 桩下 | rc 0，调到 2 个 | rc 0，0 个（"Error 1 (ignored)"） |
| `make -i outdated` | 桩下 | rc 0，调到 14 个 | rc 0，0 个 |
| 直接执行 `tools/outdated.sh` | 不设 `GITHUB_ACTIONS` | rc 0，14 个 | rc 1，0 个 |
| `make -n vuln` | | 只打印 | 只打印（判断与干活同一行） |
| 15b | `vuln` 去掉 `$(ci-only);` | — | 调到 2 个，抓到 |
| 15c | `outdated` 去掉 `$(ci-only);` | — | `outdated.sh` 自己拒绝，0 个：第二层守住，行为上看不出 |
| 15d | `outdated.sh` 去掉自己的判断 | — | 直接执行调到 14 个，抓到；`make outdated` 仍由 `Makefile` 拒绝 |
| 15e | 15c 加 15d | — | `make outdated` 调到 14 个，抓到 |
| split | 判断挪回单独的一行（原来的写法），`make -i vuln` | — | 调到 2 个，抓到 |
| O4 | `gh`、`docker` 都退 1 | "没有过时的（5 处引用）""（3 个镜像）"照样写，"退出码 4" | 只有 `!!` 行，"gh 退出码 1""docker 退出码 1" |
| O5 | 一个 action、一个镜像读不到，别的不过时 | "没有过时的（5 处引用）" | "没有过时的（读到 4 个，共 5 个）""（读到 2 个，共 3 个）" |
| O2、O6 | curl 退 22、7 | "退出码 4" | "curl 退出码 22""curl 退出码 7" |
| O7 | docker 对 distroless 回答 `{}` | — | "docker 的回答里没有摘要（jq 退出码 1）" |

## 核对

**修复核对一轮**（2026-11-15）：三位各把自己的工作树挪到 `80d76191`，之后干净、没有 `*.mutbak`。都没有 Important、Minor。

- **A**：M1、N1、N2 关上，N3 在文档稿里关上；没有新发现。r1、r2、r3、x11 失败，报错列出带行号的 `FROM golang:` 与 `RUN test` 两行；变异 12 写出 README 的 1.27.1，x7b、r10 也失败；x3、x12（web 阶段另有一行断言）由"恰有一行断言"抓到；r6–r8 照旧不报。CI 43681898393 的 `image` 一项 `[server 2/9]` 那一步照过，image-smoke 通过。CRLF、制表符等写法都红，失败在安全的一边（"接受与推后的"）。
- **B**：I1、M1、N1、N2 关上；没有新的 Important、Minor。表达式在 16 个 leg 上恰是九个为真，`make -n` 里带 `-race` 的也恰是这九个（另有 `costs-3` 的末行）；43681898393 里九份打出 `cache: true`、七份 `false`，七份没有取回、post 只有 "Post job cleanup."；存键的是 `bootstrap-2`（id 10545112376，281,885,677 字节），另八个竞态份 "Failed to save"，`short` 先跑完、没有存。合并之后存键的是第一个成功跑完的竞态份（八次里都是组合根的份，+252–316 秒，`race` 从来不是）。分份仍是 145/50/119/120/60/15/30/15。
- **C**：M1、M2、N1–N6 关上。不在 runner 上，`make vuln`、`make -i vuln`、`MAKEFLAGS=i make vuln`、`make --ignore-errors vuln`、`make -k vuln`、`make outdated`、`make -i outdated`、`MAKEFLAGS=i make outdated`、直接执行与 `bash tools/outdated.sh`、`GITHUB_ACTIONS=1 tools/outdated.sh` 都拒绝、桩 0 次；15–15e、split、x4、x5 与修复报告相同；`outdated.sh` 各种情形照新的写法；`vuln.yml` 43681898372 两项为绿。新 Nit P1S2C-N7（只在文档），在本提交。

一轮之后没有 Important、Minor 留下，不开第二轮修复。

## 实测

本机 8 核 arm64，竞态，都经 `heavy.sh`：

- **Go 还是 runner**：第五份（`make -j1 test-go-bootstrap-5` 加 `-v`，包的时间）交替两轮：本分支 Go 1.27.4、PG 18.7 223.0、223.1 秒；本分支 `GOTOOLCHAIN=local`（Go 1.27.1）、PG 18.7 224.1、224.7；`0da2fb5d` 的 Go 1.27.1、PG 18.6 222.4、221.4。本机分不出；CI 上包的时间也没变慢（第一份 369.0 → 347.9/361.5/355.7，第六份 340.3/350.7 → 336.3/336.2），Test 减包从 32–35 秒变成 81–88 秒：编译（P1S2B-I1）。
- **挪份**：八份各两轮（第一轮 247.7、205.5、235.8、197.1、223.4、238.2、188.6、188.2；第二轮 250.9、206.7、239.7、196.4、240.8、215.2、189.8、184.2；第五份的 240.8 是 TestTheMCPBatchIsRefusedWhole 慢的一种）。挪第三份的 TestTheSearchIsWhatEachReadAllows（9.95/9.93 秒）、第五份的 TestTheWikiToolsAnswerEveryCode（16.03/16.29）给第八份；挪后第三份 227.2/225.2、第五份 207.0/208.0、第八份 212.5/213.2。照各份 7763 上包与本机之比加约 33 秒，第三份约 396、第五份约 382（慢的一种约 411）、第八份约 352。`shards.py`：挪前 145/50/120/120/61/15/30/13，挪后 145/50/119/120/60/15/30/15，每个恰在一份；审查 B 用 Go 自己的匹配器另核。挪后分支 CI：第三份在 7763 上包 347.6 秒，第五份同种 CPU 上少 17、18 秒，第八份 6975P 上多 28 秒。
- **镜像**：`make image`（普通的克隆，基础镜像已在本机）48 秒，`make image-smoke` 8 秒；CI 的 image 一项 65–77 秒（之前 61、72）。node 的新摘要是 Node 24.22.3（原 24.21.0），读于 2026-11-15 UTC 00:02。

## 门禁与 CI

- **逐个提交**（实施者在干净的临时 worktree 里，经 `heavy.sh`）：到 `28980104` 的 11 个各自 `make lint knip` 为绿，碰 Go 的 `make test-go-short`，`db.ts` 的用库的故事、锁文件的 web 测试与 L4、`Makefile` 的本机拒绝与份的核对都为绿；`4a0c2585` 只改注释，在实施者的工作树里 `make lint` 与份的核对为绿。修复一轮的 6 个在干净的 worktree 里各自 `pnpm install --frozen-lockfile`、`make lint` 为 0，改守卫的另跑 `make test-go-short`（40 秒）与守卫，改 `Makefile`、`outdated.sh` 的跑桩的各种情形。审查 B 抽查 5 个提交各自 `make lint`、`make test-go-short` 为 0。
- **合并之前的门禁**：头 `28980104`：`make lint gen-check knip build` 9 秒、`make test` 3,489 秒、`make test-go-short` 41 秒、`make e2e` 146 秒（564 个通过、1 个跳过），都为绿。头 `80d76191`：`make lint gen-check knip build` 8 秒、`make test-go-short` 40 秒；修复不碰产品代码与测试的分份，不再跑全量。
- **S2 之前的两次 `main`**（设计 7.5 的"之前"：S1 合并之后，组合根与 S1 之前相同，Go 1.27.1、PostgreSQL 18.6）：`bbf3eb26` 的 run 43650412259 与记合并号的 `0da2fb5d` 的 run 43653071437，19 项第一次 attempt 全绿。八份的 Test 一步 253/403、285/261、422/410、302/297、265/254、372/383、328/240、236/236 秒，平均 328、273、416、299.5、259.5、377.5、284、236，都在 450 之下；第三份两次都在 7763 上，最贴近；第五份在 6975P、9V45 上，决定 352 的那一次还没来。e2e 9:38、8:29（E2E 一步 8:08、7:11），第 50 条那一行两次都打出（waited 432 ms、213 ms）。"之后"是 S2 合并之后的两次 `main`，合并之后记在 00 号文档第 13 节。
- **分支的 CI**（`ci.yml` 每次 19 项第一次 attempt 全绿，没有重跑；`vuln.yml` 见上一节）：

  | 推送 | 头 | `ci.yml` | 组合根在 7763 上（Test 秒） | e2e（E2E 一步） |
  |---|---|---|---|---|
  | 1 | `ed5c1f56` | 43657895833 | 第一份 432、第四份 443、第五份 458、第六份 417 | 5:48 |
  | 2 | `f3e76505` | 43660640426 | 第一份 449、第四份 449、第八份 361 | 7:08 |
  | 3 | `c654db5f` | 43664040225 | 第一份 441、第六份 419 | 5:49 |
  | 4 | `28980104` | 43672581394 | —（八份都不在 7763 上） | 5:49 |
  | 5 | `4a0c2585` | 43676717979 | 第三份 436 | 5:28 |
  | 修复 | `80d76191` | 43681898393 | 第六份 431、第七份 369、第八份 389（冷的） | 6:34 |

  第一、二次各份 "Cache is not found"，第三到五次取回 `vuln.yml` 的 `outdated` 一项存下的 3 MB；7763 上 Test 减包都是 81–88 秒，S2 之前的 `main` 是 32–35 秒。
- **修复一轮的缓存**（43681898393，冷的：主持者已删 3 MB 的键）：九个竞态份 `cache: true`，都是 "Cache is not found"；`bootstrap-2`（6975P，开跑后 +307 秒，第一个跑完的竞态份）存下 `setup-go-…-go-1.27.4-663139cb…`，约 269 MB；另八个 "Failed to save"；`short`（+180 秒最先跑完）与另六份 `cache: false`，不取不存，Test 一步里下载 89 个模块 0–3 秒。组合根在 7763 上 Test 减包 81–87 秒。e2e 照旧取回它自己的键（129 MB）。
- **本提交自己的分支 CI**（暖的第一次）：（待填：本提交的分支 CI 的 run 与 attempt；竞态的各份是否 "Cache restored from key: …-663139cb…"、约 269 MB；7763 上 Test 减包是否回到约 33 秒）

## 环境

实施、审查与核对各在自己的工作树里，`TMPDIR`、`GOTMPDIR`、桩与日志在 `~/.cache` 下各自的目录，锁固定在 `/tmp`，用完删掉。本机没有跑 `govulncheck`、`pnpm audit`、`pnpm outdated`，也没有跑去掉判断又没有桩的 `make vuln`、`make outdated`；权限分类器没有拒过什么。实施者按 id 删了自己的镜像 `nervewiki-m12-p1-s2:smoke` 与为核对 Node 版本拉的两个 node 镜像，`postgres:18.7-trixie` 留着（测试与开发库都用它），没有动共用的构建缓存与别人的容器。CI 的缓存只由主持者删过一个（上文）。要留的汇总在设计工作目录的 `m12/p1s2/`（`logs/`、`fix1/`、`review/a/`、`review/b/`、`review/c/`）。
