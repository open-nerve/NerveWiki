# 远程开发机：工具、命令与规矩

Claude 在负责人的 macOS 笔记本上工作。它的临时目录、本机记忆、锁文件的路径在远程开发机上都不存在，用得到的都已放进本仓库（本目录与 [tools/](tools/)）。仓库根的 [README.md](../../../README.md) 是开发环境的权威，这里只补上推进时要知道的。

## 1. 工具链

照 README 的"需要安装"一节装：

- **Docker**（含 Compose v2）。适配器测试、整个程序的测试与 e2e 都经 testcontainers 起 PostgreSQL 18，没有 Docker 只能跑短测试。
- **Go**：1.26 或更高，`GOTOOLCHAIN=auto`。第一次在 `server/` 下执行 Go 命令时，会自动下载 `go.mod` 的 `toolchain` 锁定的那个版本。竞态检测要 cgo，Linux 上要有 gcc。
- **Node 24**：执行一次 `corepack enable`，pnpm 12.8.1 由 `package.json` 锁定；在仓库根执行 `pnpm install`。
- **`make tools`**：把 golangci-lint 2.14.0 装进 `./bin`。`make lint` 用它，不用系统里的。
- **Playwright 的三个浏览器**：`pnpm --filter @nervewiki/e2e exec playwright install chromium firefox webkit`。Linux 上照 CI 的写法带 `--with-deps` 装系统库。
- **`make test-go-short`** 要 bash、git、jq 与 awk：发布脚本的测试直接执行 `deploy/` 下的脚本（M12/P5/S1）。
- **`make lint`** 的网页一半另对着桩跑收集许可证的两个脚本（`tools/licenses/check.mjs`），也要 bash（M12/P5/S2）。
- **`make image-smoke`** 另要 curl、jq、openssl。下面的工具要 python3。

装好之后先在 `main` 上跑一遍下面的门禁，确认机器本身是绿的，再开 S3。下载依赖与工具链是正常的开发准备；不下载的是语料与负责人的数据（[04](04-owner-decisions-and-preferences.md) 第 4 节）。

## 2. 命令

`make help` 列出全部。常用的：

| 要做的 | 命令 | 要 Docker |
|---|---|---|
| 一个提交单独是否为绿 | `cd server && go build ./... && go vet ./...`，改动的模块的 lint，`make test-go-short` | 否 |
| 只跑服务端的单元测试 | `make test-go-short`，或 `cd server && go test -short ./internal/modules/page/...` | 否 |
| 一个整个程序的测试 | `cd server && go test -race -count=1 -run '^TestName$' ./internal/bootstrap/` | 是 |
| 组合根的一份（量时间时用） | `make test-go-bootstrap-1`（`-2` 到 `-8` 同理） | 是 |
| 全部 Go 测试 | `make test-go`（`make test` 再加 `test-web`） | 是 |
| 网页的一个测试文件 | `pnpm --filter @nervewiki/web exec vitest run <文件>` | 否 |
| 全部 e2e | `make e2e`（先做 `make build`） | 是 |
| 一个故事 | `make build`，然后 `cd e2e && pnpm exec playwright test stories/<分组>/<文件>` | 是 |
| 生成物 | 改了契约或 SQL 之后 `make gen`，然后 `make gen-check` | 否 |
| 推送分支之前 | `make lint gen-check knip build`、`make test`、`make test-go-short`、`make e2e` | 是 |

- **worktree**：每个 Step 在自己的 worktree 里做，例如 `git worktree add ../nerve-wiki-m10-p1-s3 -b m10-p1-s3 main`。新的 worktree 里先 `pnpm install --frozen-lockfile --prefer-offline`；`make build` 会把前端拷进 `server/internal/platform/webui/dist/`，这个目录被忽略。
- **`make image`、`make image-smoke` 不能在 worktree 里跑**（镜像要取 `.git` 里的提交信息），要在普通的克隆里跑。它们是 M 的收尾与 M12 的事，Step 里不跑。
- 开发时看效果：`make dev`（开发库、后端、前端热更新），`make dev-db-down` 停库。

## 3. 几个 agent 共用一台机器时

- **重的运行一次只跑一个**：整个程序的测试、`make test`、`make e2e`、量时间。几个 agent（实施者、审查者）同时起 PostgreSQL 容器，彼此拖慢，量出的时间与计时守卫都不可信。用 [tools/heavy.sh](tools/heavy.sh) 包住它们，例如 `docs/v0.1/claude_to_codex/tools/heavy.sh make test`。逐个提交的核对（含 vitest 的计时断言与全量 lint）也经 `heavy.sh`，至少不与 `make test` 同时跑：两者同时跑，负载到 8 核的三倍，计时守卫会超界（M11/P1/S1）。锁是一个目录，固定在 `/tmp/nervewiki-heavy.lock`，不随 `TMPDIR` 变，可用 `NW_HEAVY_LOCK` 改。M12 之前它跟着 `TMPDIR` 走：几个 agent 各把 `TMPDIR` 放到自己的目录时，锁也各是各的，重的运行实际没有串行（M11 收尾的核对与 M12 的调查）。
- **`/tmp` 是 16G 的 tmpfs，带用户配额**：满了时写入答 `EDQUOT`（"disk quota exceeded"），`df` 却可能还显示有空。竞态构建写不进 `/tmp/go-build…`，浏览器写不进临时文件，e2e 以与代码无关的样子红（M11 收尾的第一轮修复，前两次门禁）。占着它的多是 vitest 留下的缓存目录。大的输出写完就 `gzip`；`make test`、`make e2e` 与长的变异把 `TMPDIR`、`GOTMPDIR` 与日志放到根分区上自己的目录（例如 `~/.cache/<任务名>`），用完删；只删自己的东西。
- **跑重的之前看一眼 `df`**：长时间的变异与竞态构建会让 Go 的构建缓存（`~/.cache/go-build`）涨到几十 G；根分区满了，testcontainers 里的 PostgreSQL 写不进 WAL（"No space left on device"），测试以别的样子失败（M11/P1/S2 的修复第二轮：缓存到 59 G，`make test-go-costs-2` 第一次失败）。满了先修剪自己的缓存，它可以重建，例如经 `heavy.sh` 删掉 12 小时没用过的条目（`find ~/.cache/go-build -mindepth 2 -type f -mmin +720 -delete`）；不动 Docker 的镜像、容器与卷。`make image` 与镜像的变异在 Docker 的构建缓存里越积越多（M12/P5/S2 的实施：两次 `make image` 与 13 个镜像的变异，构建缓存从约 10G 到 17.6G，平均每次约 0.5 GB，根分区从开工的 24G 到 17G）。构建缓存是共用的，`docker builder prune` 要负责人点头；镜像的变异一次做完，不要分几轮各建一遍。
- **只停自己起的进程与容器**，按 PID 或容器 id。不用 `pkill`、`killall`，不重启 Docker，不清别人的容器与卷。testcontainers 的容器由 Ryuk 收走；关掉 Ryuk 跑过的（`TESTCONTAINERS_RYUK_DISABLED=true`），只删自己那次运行留下的。
- 审查者在自己的副本或 worktree 里跑，不在实施者的树上改坏代码。

## 4. CI

- 工作流有三份：`ci.yml`（每次推送与 PR）、`release.yml`（每次推送的只读演练，tag 上的发布）、`vuln.yml`（每天、手动与改了依赖清单的推送）。`ci.yml` 里同一分支的新推送会取消正在跑的那次，等它跑完再推。
- 读 CI 用 [tools/ci-wait.sh](tools/ci-wait.sh)：`tools/ci-wait.sh <完整的 sha> [工作流的文件名]`。它找到这个提交在那个工作流（默认 `ci.yml`）里的运行，等它完成，列出每个作业的结论与时间，以及 Test、E2E 两步的时间（组合根的各份看的就是各自 Test 一步，不是整个作业的时长）。装了 `gh` 并登录时用它；否则用公开的 REST API，每个 IP 每小时 60 次，所以它每 2 分钟才问一次。仓库默认 `open-nerve/NerveWiki`，可用 `NW_REPO` 改。一个提交可以有几个工作流的运行（M12 起有每天的漏洞检查与发布的演练）；M12 之前它不分工作流、取最后建的那一次，就可能等到别的工作流的绿。
- **依赖的漏洞**：`make vuln`、`make outdated` 只在 GitHub 的 runner 上跑：不在 runner 上，它们与 `tools/outdated.sh` 都拒绝，`make -i` 也一样。结论看 `vuln.yml`：`gh run list --workflow vuln.yml --branch main --limit 1`，或 `tools/ci-wait.sh <sha> vuln.yml`。每个 Step 合并之前看一眼（00-M12 4.3）。作业日志要认证：`gh run view <id> -R open-nerve/NerveWiki --log`（在仓库之外要带 `-R`）；`outdated` 一项带 `continue-on-error`，它的结论不算，读它的输出：六节各有结果、末行 `0 with errors`。
- **发布的工作流**：`release.yml` 在每次推送上跑两个只读的演练作业（两种架构的构建与 smoke），读法 `tools/ci-wait.sh <sha> release.yml`；tag 的那次运行用 `gh run list -R open-nerve/NerveWiki --workflow release.yml --branch <tag>`（同一个提交在 `main` 上还有一次演练）。amd64 的演练另只读地取出 `/licenses`、生成两份 SBOM，作业日志的 Check the SBOMs 一步打出两份的文档 `name`、包数，与前端 SBOM 里不在 `/licenses` 的 `@types/` 包（M12/P5/S2）。
- **actionlint**：`make lint-actions` 只在 runner 上跑（本机拒绝，`make -i` 也一样），结论看 `costs-1` 一份的 Lint workflows 一步的注解；作业日志里 `-verbose` 列出读了哪几份。actionlint 自己出错（模板、参数）时，注解只有 "Process completed with exit code 2"，原因只在作业日志里（M12/P5/S1 的第 5 个提交）。
- **分份**：组合根现在分八份（`Makefile` 的 `BOOTSTRAP_FIRST` 到 `BOOTSTRAP_EIGHTH`），CI 共 19 项：16 个 server 的 leg（race、bootstrap-1 到 bootstrap-8、costs-1 到 costs-3、costs-lint、guards-core、guards-sources、short），加 web、e2e、image。每一份两次 `main` 运行的 Test 一步的平均要在 450 秒之下，过线时先挪给邻份，挪不下就再分一份，不提高 `-timeout`。本机量时在同一把 heavy 锁下分别运行 `make -j1 test-go-bootstrap-1` 到 `test-go-bootstrap-8`，每份两轮；不要拿 `make test-go` 内部的 `-j6` 当这些串行的样本。更早的三份、四份与十项、十一项的记录保留当时的口径。组合根的 Test 一步含编译：Test 一步减 bootstrap 包的 `ok` 行的时间，取回的缓存与代码是同一棵树时只有 1–3 秒，缓存之后改过的包越多越长（S2 之前的 `main` 上，一个月前存下的键，7763 上约 33 秒，别的 CPU 约 21–26 秒）。多出几十秒时，看这一份取回的 Go 缓存是哪一份存的（存的那一次 post 一步的 "Cache saved with the key"）：不带竞态的份存的键对竞态的构建没有用，大小不说明什么（M12/P1/S2）。
- 失败的原因：作业日志要认证（`gh run view <id> --log-failed`）。不认证时，失败的作业的 annotations（`/repos/<repo>/check-runs/<作业 id>/annotations`）是公开的，Go 的失败与 Playwright 的 `github` 报告器都写在那里。
- 只在 Linux 上失败的，在与 `go.mod` 的 `toolchain` 同版本的 golang 镜像（`deploy/Dockerfile` 的 server 阶段那个）里复现（`--cpus=2`、`-race`），见 [05](05-lessons.md) 第 7 节。
- **e2e 的时限与系统包的缓存**（00-M12 决定 599–601，旁支 `m12-e2e-browsers-cache`）：
  - E2E 一步自己 18 分钟，是 00-M12 第 3 节第 11 条的界。注解是 "The action 'E2E' has timed out after 18 minutes"、作业 failure 时，是 E2E 本身变慢：不重跑，停下查。
  - 装浏览器一步 30 分钟、作业 55 分钟，盖住 apt 的镜像慢的时候：2026-11-18 AWS 上的 runner 走 `us-east-2.ec2.archive.ubuntu.com`，不命中缓存时这一步 25:06；2026-11-11 Azure 上解析不了 `azure.archive.ubuntu.com`。这一步超时是外部故障：只重跑 e2e，记下 run、attempt 与作业日志开头的 `Cloud:`、`Region:`。
  - `--with-deps` 用 apt 取的 `.deb` 存在缓存里，键 `playwright-debs-<ImageOS>-<Playwright 的版本>-<ISO 周>`。一个分支取得到自己存的与 `main` 存的；`main` 还没存过时（合并之后的第一次）与升 Playwright 之后的第一次取不到，新的一周取回上一周的一份、只补取变了的。看命中："Restore the Playwright system packages" 一步的 `Cache restored from key`，装浏览器一步里 apt 的 `Need to get 0 B`。
  - 放回的 `.deb` 不核哈希（00-M12 决定 601）。怀疑缓存坏了时，删掉键以 `playwright-debs-` 开头的条目；那是写操作，先问负责人。

## 5. tools/ 里的四个小工具

都是 Claude 用过的，照用或换成自己的。

- **[mut.py](tools/mut.py)：变异。** 一份 JSON 列出变异，每一条在 `server/` 下改一个文件的一处，跑一个命令，期望它失败，再从备份复原：

  ```json
  [{"name": "guard-lets-every-step-by",
    "file": "internal/modules/page/app/mode_guard.go",
    "old": "\t\treturn domain.Refuse(v.Err, v.NodeID)",
    "new": "\t\treturn nil",
    "cmd": "go test -count=1 -run '^TestTheModeGuard' ./internal/modules/page/app/",
    "timeout": 600}]
  ```

  `python3 docs/v0.1/claude_to_codex/tools/mut.py spec.json [名字 ...]`。在 worktree 里用时设 `NW_SERVER=<worktree>/server`。结果里 `FAILED(good)` 是抓到了；`PASSED(BAD)` 是活下来了，是一条发现；`BUILD-ERROR`、`CMD-ERROR`、`FAILED(unnamed)` 要手查。可选的 `also` 是同一文件里另外几处 `[[old, new], ...]`，每处也只能出现一次；`timeout` 是秒，默认 600。先在没改过的树上跑一遍同样的命令。改 `.github/workflows/` 下的文件时，`.mutbak` 放在旁边会被 `TestOnlyATagsJobsMayWrite` 当作又一份工作流（它要目录里恰是三份），备份要放在别处（M12/P5/S1）。`file` 可以写 `../deploy/…`、`../tools/…`（相对 `server/`，M12/P5/S2）。
- **[heavy.sh](tools/heavy.sh)：串行化重的运行**，见第 3 节。
- **[ci-wait.sh](tools/ci-wait.sh)：等 CI**，见第 4 节。
- **[check_links.py](tools/check_links.py)：文档的链接检查。** `python3 docs/v0.1/claude_to_codex/tools/check_links.py [基准]`，基准默认 `origin/main`。它只看相对基准改过的 Markdown 文件，只列出新坏掉的相对链接，有就以 1 退出。文档提交之前跑。

## 6. 规矩

- **不碰真实的客户端配置**：`~/.codex/`、`~/.claude.json`、`~/.claude/`、任何项目里已有的 `.mcp.json`，也不对着它们运行 MCP 客户端。测试里的 MCP 客户端是官方的 Go SDK 与 TypeScript SDK，够用；真实的 Claude Code、Codex 只在负责人的人工清单里跑。
- **`main` 只推完成并核对过的状态**（合并加文档，CI 绿），从不强推，从不推做了一半的东西。分支随时可以推。
- **不提交**本机的东西：agent 的设置与临时目录、量时间的输出、变异的备份（`*.mutbak`）。提交之前看一眼 `git status`。
- **本目录是交接时的快照**，之后不必维护。进度写在仓库原本的地方：v0.1-design 12.6、各 00 号设计的第 12、13 节、各 Phase 设计的"结果"一节与审查记录。
- **升 pgx 时**核对 `server/internal/modules/linking/adapter/postgres/prepared_source_text_test.go` 的两个常量：`backgroundReadBuffer`（8192）对 `pgconn/internal/bgreader` 的 `bgRead` 取的 `iobufpool.Get`；`backgroundReadHeader`（24）对 `internal/iobufpool` 的 `New` 返回的 `*[]byte`。pgx 改了缓冲的大小，`TestPreparedSourceDoesNotCopyTextForComparison` 就减不掉后台读取器的缓冲，在竞态一项上又会偶尔红。pgx 不再为每块分配 `*[]byte` 时，配对取到的只是平常的几个，守卫回到原来那样严，也是偶尔红。两种情形都不会放过复制（00-M12 决定 357）。

M10/P2/S2 边界再平衡与恢复折叠之前的历史精确代码33b4，其CI40718770843全部11作业成功，四片Test444/419/302/271秒仅一个分支样本；两次main均值仍按上述规则另验。本机四份双轮及来源差异见[S2记录](../M10-llm-wiki/reviews/P2-S2-source-hash-review.md#33b4最终代码控制与本机验证)，不以本机量替代CI。 最新代码/门禁/CI及分支、main配对以[S2当前验证](../M10-llm-wiki/reviews/P2-S2-source-hash-review.md#最终验证与集成)为准，旧数不自动套用到后续修复头。
