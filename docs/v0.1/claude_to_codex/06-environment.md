# 远程开发机：工具、命令与规矩

Claude 在负责人的 macOS 笔记本上工作。它的临时目录、本机记忆、锁文件的路径在远程开发机上都不存在，用得到的都已放进本仓库（本目录与 [tools/](tools/)）。仓库根的 [README.md](../../../README.md) 是开发环境的权威，这里只补上推进时要知道的。

## 1. 工具链

照 README 的"需要安装"一节装：

- **Docker**（含 Compose v2）。适配器测试、整个程序的测试与 e2e 都经 testcontainers 起 PostgreSQL 18，没有 Docker 只能跑短测试。
- **Go**：1.26 或更高，`GOTOOLCHAIN=auto`。第一次在 `server/` 下执行 Go 命令时，会自动下载 `go.mod` 锁定的 `go1.27.1`。竞态检测要 cgo，Linux 上要有 gcc。
- **Node 24**：执行一次 `corepack enable`，pnpm 12.8.1 由 `package.json` 锁定；在仓库根执行 `pnpm install`。
- **`make tools`**：把 golangci-lint 2.14.0 装进 `./bin`。`make lint` 用它，不用系统里的。
- **Playwright 的 Chromium**：`pnpm --filter @nervewiki/e2e exec playwright install chromium`。Linux 上照 CI 的写法带 `--with-deps` 装系统库。
- **`make image-smoke`** 另要 curl、jq、openssl。下面的工具要 python3。

装好之后先在 `main` 上跑一遍下面的门禁，确认机器本身是绿的，再开 S3。下载依赖与工具链是正常的开发准备；不下载的是语料与负责人的数据（[04](04-owner-decisions-and-preferences.md) 第 4 节）。

## 2. 命令

`make help` 列出全部。常用的：

| 要做的 | 命令 | 要 Docker |
|---|---|---|
| 一个提交单独是否为绿 | `cd server && go build ./... && go vet ./...`，改动的模块的 lint，`make test-go-short` | 否 |
| 只跑服务端的单元测试 | `make test-go-short`，或 `cd server && go test -short ./internal/modules/page/...` | 否 |
| 一个整个程序的测试 | `cd server && go test -race -count=1 -run '^TestName$' ./internal/bootstrap/` | 是 |
| 组合根的一份（量时间时用） | `make test-go-bootstrap-1`（`-2`、`-3` 同理） | 是 |
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

- **重的运行一次只跑一个**：整个程序的测试、`make test`、`make e2e`、量时间。几个 agent（实施者、审查者）同时起 PostgreSQL 容器，彼此拖慢，量出的时间与计时守卫都不可信。用 [tools/heavy.sh](tools/heavy.sh) 包住它们，例如 `docs/v0.1/claude_to_codex/tools/heavy.sh make test`。锁是一个目录，默认 `${TMPDIR:-/tmp}/nervewiki-heavy.lock`，可用 `NW_HEAVY_LOCK` 改。
- **只停自己起的进程与容器**，按 PID 或容器 id。不用 `pkill`、`killall`，不重启 Docker，不清别人的容器与卷。testcontainers 的容器由 Ryuk 收走；关掉 Ryuk 跑过的（`TESTCONTAINERS_RYUK_DISABLED=true`），只删自己那次运行留下的。
- 审查者在自己的副本或 worktree 里跑，不在实施者的树上改坏代码。

## 4. CI

- 工作流是 `.github/workflows/ci.yml`。同一分支的新推送会取消正在跑的那次，等它跑完再推。
- 读 CI 用 [tools/ci-wait.sh](tools/ci-wait.sh)：`tools/ci-wait.sh <完整的 sha>`。它找到这个提交的那次运行，等它完成，列出每个作业的结论与时间，以及 Test、E2E 两步的时间（组合根的四份看的就是各自 Test 一步，不是整个job时长）。装了 `gh` 并登录时用它；否则用公开的 REST API，每个 IP 每小时 60 次，所以它每 2 分钟才问一次。仓库默认 `open-nerve/NerveWiki`，可用 `NW_REPO` 改。 当前四分片配置为8个server leg加web/e2e/image共11项；本机测量在同一heavy锁下分别运行`make -j1 test-go-bootstrap-1`至`test-go-bootstrap-4`，每份两轮。不要用plain `make test-go`的内部`-j5`作为这些独立串行样本；旧三份/十项CI记录保留原口径。
- 失败的原因：作业日志要认证（`gh run view <id> --log-failed`）。不认证时，失败的作业的 annotations（`/repos/<repo>/check-runs/<作业 id>/annotations`）是公开的，Go 的失败与 Playwright 的 `github` 报告器都写在那里。
- 只在 Linux 上失败的，在 `golang:1.27.1-trixie` 容器里复现（`--cpus=2`、`-race`），见 [05](05-lessons.md) 第 7 节。

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

  `python3 docs/v0.1/claude_to_codex/tools/mut.py spec.json [名字 ...]`。在 worktree 里用时设 `NW_SERVER=<worktree>/server`。结果里 `FAILED(good)` 是抓到了；`PASSED(BAD)` 是活下来了，是一条发现；`BUILD-ERROR`、`CMD-ERROR`、`FAILED(unnamed)` 要手查。可选的 `also` 是同一文件里另外几处 `[[old, new], ...]`，每处也只能出现一次；`timeout` 是秒，默认 600。先在没改过的树上跑一遍同样的命令。
- **[heavy.sh](tools/heavy.sh)：串行化重的运行**，见第 3 节。
- **[ci-wait.sh](tools/ci-wait.sh)：等 CI**，见第 4 节。
- **[check_links.py](tools/check_links.py)：文档的链接检查。** `python3 docs/v0.1/claude_to_codex/tools/check_links.py [基准]`，基准默认 `origin/main`。它只看相对基准改过的 Markdown 文件，只列出新坏掉的相对链接，有就以 1 退出。文档提交之前跑。

## 6. 规矩

- **不碰真实的客户端配置**：`~/.codex/`、`~/.claude.json`、`~/.claude/`、任何项目里已有的 `.mcp.json`，也不对着它们运行 MCP 客户端。测试里的 MCP 客户端是官方的 Go SDK 与 TypeScript SDK，够用；真实的 Claude Code、Codex 只在负责人的人工清单里跑。
- **`main` 只推完成并核对过的状态**（合并加文档，CI 绿），从不强推，从不推做了一半的东西。分支随时可以推。
- **不提交**本机的东西：agent 的设置与临时目录、量时间的输出、变异的备份（`*.mutbak`）。提交之前看一眼 `git status`。
- **本目录是交接时的快照**，之后不必维护。进度写在仓库原本的地方：v0.1-design 12.6、各 00 号设计的第 12、13 节、各 Phase 设计的"结果"一节与审查记录。

M10/P2/S2 边界再平衡与恢复折叠之前的历史精确代码33b4，其CI40718770843全部11作业成功，四片Test444/419/302/271秒仅一个分支样本；两次main均值仍按上述规则另验。本机四份双轮及来源差异见[S2记录](../M10-llm-wiki/reviews/P2-S2-source-hash-review.md#33b4最终代码控制与本机验证)，不以本机量替代CI。 最新代码/门禁/CI及分支、main配对以[S2当前验证](../M10-llm-wiki/reviews/P2-S2-source-hash-review.md#最终验证与集成)为准，旧数不自动套用到后续修复头。
