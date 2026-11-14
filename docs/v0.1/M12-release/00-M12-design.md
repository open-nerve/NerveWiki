# M12 发布：总设计

| 项 | 内容 |
|---|---|
| 里程碑 | M12 发布（`M12-release`） |
| 日期 | 2026-11-14 |
| 状态 | 初稿，待设计审查 |
| 依赖 | M0–M11 全部。直接改动或量的：M0/P6 的镜像与 image-smoke；M1 的续期（e2e 的 `goto`）；M4–M11 的成本守卫与夹具；M5 的事件流（每个账户的流数）；M6 的解析与标签（标题 id、`#//`）；M7 的导入（名称的编码）；M8 的撤销计划（第 302 条）与日志的读；M9 的 `log`；M11 的回滚 |
| 上级文档 | [v0.1 总体设计](../v0.1-design.md) 第 11、12.2、12.5、12.6、13、14、15 节；[文档约定](../../README.md)；[推进方式](../claude_to_codex/02-workflow.md) |
| 依据 | 负责人 2026-11-13 批准开工与四个回答（第 11 节）；三份只读调查 I1（发布、部署与依赖）、I2（性能与内存）、I3（打磨与历史的分级），基点 `5e8af5b5`；六份 `open` 的移交 |
| 代码位置的写法 | 同 [M11 总设计](../M11-upgrade/00-M11-design.md)：模块写作 `page/app/rollback.go:77`（`server/internal/modules/` 之下）；`platform/…`、`bootstrap/…`、`shared/…` 在 `server/internal/` 之下；前端写作 `web/…/app/confirm-dialog.tsx:177`（`web/apps/web/src/` 之下）；e2e 写作 `e2e/playwright.config.ts:3`；其余写仓库根起的路径。行号都是 `main` 的 `5e8af5b5` 的 |

---

## 1. 目标

M12 把 v0.1 发布出去。总体设计 12.2 的一行是："部署与备份文档、性能与内存实测（包括大笔记本下的页面树与搜索）、版本号与发布流程、发布 v0.1"。六份移交又加了打磨与历史的事项。负责人 2026-11-13 的四个回答定了做到什么程度（第 11 节）。

1. **地基**：发布之前把工具链与数据库升到带安全修复的版本（Go 1.27.4、PostgreSQL 18.7、三个基础镜像的摘要、两个 action）；让 e2e 的 `goto` 不再打断续期，`main` 的 CI 因此稳下来（4.3、4.4）。
2. **打磨，分级处理**（负责人第 1 条）：用户会碰到的错误行为、无障碍问题、误导的文字在 v0.1 修；纯体验的改进写进 v0.2 的移交，发布说明列为已知限制（4.5）。
3. **历史与服务端的正确性**：修好 M11 第 302 条的回滚撤销冲突；日志的读放进一个快照；标题键与标题 id 的两处改动赶在 v0.1.0 之前，那时还没有生产的库（4.6）。
4. **实测，超出设计预算才修**（负责人第 2 条）：量到设计写的规模；只修超出已写预算的；页面树的界线在量之前写死；其余记成已知限制，进 v0.2（4.7）。
5. **部署与备份文档**：一份照着就能做的运维手册 `deploy/README.md`，带 `deploy/compose.yaml` 样例；备份与恢复有命令、演练过（4.8）。
6. **版本号与发布流水线**：tag 触发的 `release.yml`，构建 amd64 与 arm64 两种镜像，推到 GHCR，带签名、SBOM 与来源证明，建 GitHub Release（负责人第 3 条；4.9、4.10）。
7. **发布**：收尾审查之后问负责人，推 `v0.1.0-rc.1`、校验，再推 `v0.1.0`；人工验收在发布之后，发现的问题出 v0.1.1（负责人第 4 条；4.11、4.12）。

## 2. 范围

**做**：

- **工具链与依赖**（只做与安全相关的，4.3）：
  - Go 的 `toolchain` 到 `go1.27.4`，golang 镜像随之；
  - PostgreSQL 到 `18.7`；
  - node、golang、distroless 三个基础镜像的摘要；
  - `actions/setup-node` 到 v7.1.0，`actions/upload-artifact` 到 v7.0.2；
  - `govulncheck` 报出的 Go 模块；
  - 两个守卫：Dockerfile 的 Go 版本等于 `toolchain`；README 的 PostgreSQL 镜像并进已有的一致性守卫；
  - `make vuln`（只在 CI 的 runner 上跑）与 `make outdated`。
- **e2e**：`goto` 之前等在途的续期与读答完的帮手，故事逐个换上，一条门禁不许直接调 `page.goto`；`global-setup.ts` 的网络等待打一行日志（移交 M5-polish 第 50、51 条，4.4）。
- **网页的打磨**：附录 A 列出的 A、B 两类，按组件分六个 Step（4.5）。
- **服务端的正确性与历史**（4.6）：
  - 第 302 条：M8 的撤销计划认出"同一个变更集更早删下的回收站根"；
  - 日志的五条读放进一个快照（H5）；
  - 标题键对切罗基文幂等（25.1），标题 id 带上自动链接与原始 HTML 的文字（10.2）；
  - 标签列表不列 `/`（10.1）；看不见的字符组成的名字（30.2）；`log` 与 `index` 对空串的约定（30.4、30.5）；
  - 每个账户的事件流数的上限（移交 M5-performance 第 6 条）；
  - 导入时名称不是 UTF-8 的条目按 GBK 回退（T3）。
- **实测**（4.7）：
  - 进仓库的设施：合成的大笔记本与搜索语料、并发驱动、内存采样、浏览器一侧的计时脚本、现有守卫的规模开关；
  - 量：4.7.1 表里的每一行；
  - 超出预算的修，带缩小规模的守卫。
- **部署文档**（4.8）：`deploy/README.md`、`deploy/compose.yaml` 与它的检查；根 README 的"部署"一节缩成入口；从开发版升级的附录（看负责人的回答，第 11 节）。
- **版本号与发布**（4.9、4.10）：
  - 四份默认值的一致性守卫；镜像补三个 OCI label；
  - `ci.yml` 不在 tag 上跑；
  - `release.yml`：闸门、两种架构的构建与 smoke、推送、合成索引、来源证明、SBOM、签名、Release、发布之后的核对；
  - 第三方许可证随镜像与 Release 发出；
  - `CHANGELOG.md`（发布说明与已知限制）、`SECURITY.md`。
- **人工验收的入口**（4.12）：读屏软件的清单；一份汇总的验收入口，链到输入法清单、MCP 的清单 A–K 与 M 类各项。
- **v0.2 的移交**：C 类与推后的事项，在收尾时写进 `docs/v0.2/handoffs/`（4.13）。

**不做**：

- C 类的 91 项、H3（会话里按时间分版本）、H4 的功能（永久删除）、T1（结束的任务行的保留期）、T2（按人的队列上限）、T5（总量配额）、T8（从解析不到的附件链接上传）：进 v0.2（4.5、4.13）。
- H1（撤销按次序值比较）、24.1（结果未知的查找）、T4（River 取到之后停掉的任务）：罕见又大，写成已知限制，进 v0.2（I3 第 7 节第 2 条）。
- 只从 ARIA 推断"念两遍"的约十项：移到发布之后的读屏验收（I3 第 7 节第 1 条）。
- 页面树的分层加载：只在 1 万页过不了 4.7.2 的界线时才做（4.7.2）。
- 搜索 300 ms 尾巴的改法、按页日志的迁移、共用的参与者界线、MCP 名额的配置项、慢的发送者与 Caddy 早答的服务端改法：除非实测超出 4.7.1 的判据，进 v0.2（I2-D3、D6、D7、D12、D13）。
- Node 26、功能性的大版本依赖（River 0.51、katex 0.19、`@lezer/markdown` 1.8、mermaid 12、pgx、goose、`x/text`）、Dependabot 的版本更新 PR（4.3）。
- `latest` 标签、独立的二进制附件、宣传 `go install`（4.9、4.10）。
- 多实例（4.8）；pprof 或 metrics 端点（I2-D14）。
- 网页上显示 MCP 地址：负责人 M9 的决定是"等打磨"，打磨分级之后它是 C（31.1、47），进 v0.2。
- 新的接口操作、MCP 工具与 e2e 故事分组。

## 3. 完成标准

总体设计 12.5 的通用标准之外，每条写出由什么证明。

1. **工具链**（P1/S2）：
   - `server/go.mod`、`server/tools/go.mod` 的 `toolchain` 与 `deploy/Dockerfile` 的 golang 镜像同是 1.27.4，由一个短测试核对；改其中一处，测试失败（反向对照，记在 S2 的记录）。
   - PostgreSQL 镜像在五处（`pgtest/pgtest.go:35`、`deploy/compose.dev.yaml:8`、`e2e/fixtures/db.ts:13`、`deploy/image-smoke.sh:86`、`README.md:629`）同是 `postgres:18.7-trixie`，由 `pgtest/image_test.go` 核对（今天只核对前四处，`pgtest/image_test.go:12-39`）。
   - `make vuln` 在 CI 的 `short` 一份里为绿；证据是 S2 的分支 CI 与合并之后两次 `main` 的 run。
2. **e2e 的 `goto`**（P1/S1）：
   - 故事里不再直接调 `page.goto`，由一条门禁守住，有反向对照。
   - 帮手有一个对照测试：把续期的答复延后，去掉帮手时页面落到 `/sign-in`，带着帮手时不落（05-lessons 第 6 节："要复现这种竞态，把答复延后，不要靠重复跑"）。
   - P1/S1 合并之后到 M12 收尾，`main` 的 e2e 没有一次因为被跳回登录页而红；收尾记录逐次列出。
3. **打磨**（P2）：
   - 附录 A 的每一项 A、B 各有一个在没修的代码上失败的测试（vitest 或 e2e），或在 Step 的记录里写明为什么只能人工核对；每个 Step 有变异表。
   - 焦点的修复在三种 Playwright 引擎里探查过（05-lessons 第 6 节）；改到的故事照需要进 `threeBrowsers`。
   - 移到发布之后的项都在 `manual/` 的清单里；C 类都在 v0.2 的移交里。
4. **第 302 条**（P3/S2）：
   - `TestRevertingARollbackConflictsUnderANewFolderHoldingBaselineAndNewPages`（`bootstrap/upgrade_rollback_revert_test.go:30`）的 nested 一格（`:87`）翻过来：撤销成功，树回到回滚之前；测试随之改名。
   - 一个新的性质测试：随机的"先删一页、后删它的上级"的变更集，撤销之后等于变更集之前；期望由生成器自己的模型算出。去掉新规则的变异让它失败。
   - M8 原有的撤销测试与性质测试照过。
5. **其余的服务端正确性**（P3/S1、S3）：
   - 25.1：`TitleKey` 幂等的样本含切罗基文；只差切罗基文大小写的两个标题在同一父页下被拒。
   - 10.2：标题里有自动链接与原始 HTML 时，id 含它们的文字。
   - 10.1：标签列表不列 `/`。
   - H5：日志的读与一次撤销的提交交错时，读到的要么都有、要么都没有（确定的交错测试，13.4 第 4 条）。
   - 流数：一个账户开第 17 条流时，最老的那条被重置（行为测试）。
   - T3：中文 Windows 打包、没有 UTF-8 标志的 zip 导入之后名字对，报告写明按 GBK 解读。
6. **实测**（P4）：
   - 4.7.1 表的每一行在 P4 的记录里有：机器、规模、种子、两轮的数、对照预算的结论（过、超、无预算）。
   - 页面树的三条界线在量之前写进 P4 的设计，量之后逐条对照。
   - 每一个超出的已修，带缩小规模的守卫；或经负责人改判。其余写进已知限制与 v0.2 的移交。
7. **部署文档**（P5/S3）：
   - `deploy/README.md` 覆盖第 7 节"移交的落实"里每一条部署项，逐条对照表在 S3 的记录里。
   - `deploy/compose.yaml` 由 `make deploy-check` 核对，CI 的 `image` 一份跑它。
   - 照 `deploy/README.md` 从零装一遍、备份再恢复一遍，在本机做过，记录写命令与结果；推 rc 之后用 GHCR 的镜像再装一遍。
8. **版本号**（P5/S1、发布）：
   - Makefile、buildinfo 与 Dockerfile 两个 `ARG` 的默认值一致，由短测试守住，有反向对照。
   - `0.1.0` 的镜像里 `nervewiki version`、`/api/v0/instance` 与 MCP 的 `serverInfo.version` 都是 `0.1.0`：两种架构上的 image-smoke 核对。
   - 发布之后 `main` 的默认值是 `0.1.1-dev`。
9. **发布流水线**（P5/S1、S2、发布）：
   - `release.yml` 在分支上的演练两种架构都绿。
   - 本机以一个本地的 registry 演练过推送、合成索引、签名与 SBOM（不上传任何东西）。
   - `v0.1.0-rc.1` 与 `v0.1.0`：Release 在；镜像两种架构都能拉；`gh attestation verify` 与 `cosign verify` 都过；SBOM 与许可证在 Release 的附件里。证据是收尾记录的"发布"一节，带 run 号与摘要。
10. **移交与文档**：
    - 六份移交都改为 `done`；v0.2 的移交在 `docs/v0.2/handoffs/`。
    - `v0.1-design.md` 照第 7 节"总体设计的修订"改过，12.6 的 M12 是"实现完成，待人工验收"。
    - README、`CHANGELOG.md`、`SECURITY.md`、`manual/` 的清单都在。
11. **CI**：每个 Step 合并之后，两次 `main` 的组合根每一份的 Test 一步平均在 450 秒之下（06-environment 第 4 节）；e2e 在它 20 分钟的时限之内（`.github/workflows/ci.yml:263`）。

## 4. 关键决定

决定的编号在初稿里是临时的 `M12-n`，审查之后接着 303 编正式号（第 11 节）。

### 4.1 次序：地基先行，实测在代码定下之后

- **P1 先做**：
  - e2e 的 `goto` 挡发布。发布要 `main` 的 CI 为绿，而 Playwright 不重试（`e2e/playwright.config.ts` 没有 `retries`）；M11 收尾之后 `main` 的 run 43339247136 已经因它红过一次（[M11 收尾审查](../M11-upgrade/reviews/M11-closeout-review.md)"门禁"一节）。它先做，后面每个 Step 的 CI 才稳。
  - 工具链与 PostgreSQL 的升级随后。之后的实测、计划测试与成本守卫都在最终的工具链与数据库上跑（I1-D12）。
- **P2、P3、P4/S1、P5/S1 可以并行**：它们改的部件基本不重叠（第 7 节"次序与并行"）。重的运行照旧经 `heavy.sh` 串行。
- **页面树的结论先于 P2/S6**：P2/S6 改 `PageTreeStore` 的几项（12.12、27.2、22.14、23.10）。1 万页过不了 4.7.2 的界线时，树要改，它们排在改法之后（I3 第 8 节）。所以 P4/S2 先量页面树。
- **服务端的实测（P4/S3）在 P3 合并之后**：P3 改了撤销的计划（302）、日志的读（H5）与标题键（25.1），量在改之后才算数。
- **部署文档（P5/S3）在 P4/S3 之后**：它要内存、磁盘、WAL 与容量的数。发布说明的已知限制取自 P2–P4。

### 4.2 heavy.sh 的锁（P1 之前的一个提交）

- 现在锁的默认是 `${TMPDIR:-/tmp}/nervewiki-heavy.lock`（`docs/v0.1/claude_to_codex/tools/heavy.sh:8`）。几个 agent 各设 `TMPDIR` 时，锁就各是各的，重的运行不再串行。M12 的调查期间就发生过（I2 第 8 节第 6 条）。
- 改为固定的 `/tmp/nervewiki-heavy.lock`，仍可由 `NW_HEAVY_LOCK` 改（主持者的决定 M12-6）。
- `06-environment.md` 第 3 节写两件事：`/tmp` 有用户配额，大的输出与 `TMPDIR` 放 `~/.cache` 下自己的目录；改 `TMPDIR` 不再改变锁。
- 它是 M12 的第一个提交，写 `docs: heavy.sh's lock stays in /tmp whatever TMPDIR says (M12)`，直接进 `main`。

### 4.3 工具链与依赖：只做与安全相关的更新

照 I1-D10、D12（主持者的决定 M12-7）。

- **Go 1.27.1 → 1.27.4**：1.27.2–1.27.4 修了 `net/http`、`crypto/tls`、`crypto/x509`、`html/template` 等的安全问题，都在服务端运行时的路径上（I1 第 5 节）。
  - 改 `server/go.mod:5`、`server/tools/go.mod:5` 的 `toolchain`，`deploy/Dockerfile:23` 换成 `golang:1.27.4-trixie` 的摘要，`README.md:16` 随之。
  - golang 镜像设了 `GOTOOLCHAIN=local`：只改 `toolchain`、漏改 Dockerfile 时，镜像悄悄用旧的 Go 构建（I1 第 5 节）。加一个短测试，核对 Dockerfile 的 golang 镜像版本等于两份 `go.mod` 的 `toolchain`。
  - 核对两处写着 1.27.1 的代码注释：`platform/httpserver/mcpserver/edge.go:106`（`encoding/json/v2` 在默认打开的 GOEXPERIMENT `jsonv2` 之后，M9/P1 要求升级时核对它仍打开），`page/domain/grep.go:149`（引了标准库 `regexp/syntax` 的行号）。
- **PostgreSQL 18.6 → 18.7**：18.7 有一批 CVE 与 GIN 的修复，正对着 `page_search` 的索引（I1-D12）。
  - 改守卫里的四处与 `README.md:629`；`pgtest/image_test.go` 把 README 也核对进去（I1 第 8 节第 6 条）。
  - 总体设计 7.1 的例子随之改（第 7 节"总体设计的修订"）。
  - 计划测试（13.4 第 9 条）在 18.7 上跑一遍；计划换了，在 S2 里查清，不放宽测试。
- **三个基础镜像的摘要**：node（Node 24 线到 24.22.3）、golang、distroless（I1 第 5 节）。
- **两个 action**：`actions/setup-node` v7.1.0、`actions/upload-artifact` v7.0.2，按 SHA（I1 第 5 节的表）。
- **漏洞扫描**（主持者的决定 M12-5）：
  - 本机不跑 `govulncheck` 与 `pnpm audit`：前者会把模块清单发给 vuln.go.dev（本机被权限分类器拒过），后者把依赖清单发给 registry（I1 头的表）。
  - 加 `make vuln`：`go run golang.org/x/vuln/cmd/govulncheck@<固定版本>`，在 `server/` 与 `server/tools/` 各跑一次。不放进 `server/tools/go.mod`：那里的 kin-openapi 等跟着 oapi-codegen 的要求（`server/tools/go.mod` 的注释），新的工具依赖会牵动它们。
  - CI 的 `short` 一份跑 `make vuln`，发布的闸门也跑（4.10）。
  - S2 先推一次分支，读 `make vuln` 的结果，再升报出的模块。报出的若是 River、`x/text` 这类有连带的模块（新的迁移、换 Unicode 表就要 reindex），在 S2 的记录里写明连带，照做或请主持者定。
  - npm 一侧靠 GitHub 的依赖图与 Dependabot alerts，由负责人在仓库设置里开（第 11 节"需要负责人确认"）。
- **不升的**（I1-D9、D10、D11）：
  - Node 26：Node 只用于构建与 e2e，运行时镜像里没有；Node 24 维护到 2028-04-30；换 26 要改 corepack 的四处、`engines` 与 `@types/node`。移交 M0-P6 第 4 条的"Node 26 成为 LTS 之后评估"由此了结：评估过，v0.1.x 之后再换。
  - 功能性的大版本：River 0.51（可能带新的迁移，`server/migrations/river_test.go` 会失败）、katex 0.19 与 `@lezer/markdown` 1.8（要重核补丁，13.5 第 5 条）、mermaid 12、pgx、goose、`x/text`（换 Unicode 表就要 reindex，总体设计第 14 节）。
  - golangci-lint 2.15、pnpm 12.18：只用于开发，不涉安全。
- **`make outdated`**（I1-D11）：只读地列出过时的 Go 模块、npm 包、actions 与镜像，做的就是 I1 第 5 节那张表。每次发布之前人工跑一次。Dependabot 的版本更新 PR 不开：每个 PR 都触发 19 项的 CI，版本精确固定、补丁要人工核对，自动的 PR 大多会红。要不要开由负责人定（移交 M0-P6 第 4 条写明"由负责人决定"；第 11 节）。

### 4.4 e2e 的 `goto`

照 I3 第 7 节第 14 条（主持者的决定 M12-8）。

- **成因**：故事在页面的读还在途时调 `page.goto`，在途的续期被打断。服务端已换了令牌、浏览器没收到，再用旧令牌就算重复使用，会话被撤销，页面落到 `/sign-in`（移交 M5-polish 第 51 条；M1 第 10 节接受了这个后果，`docs/v0.1/M1-auth/00-M1-design.md:193`）。`e2e/stories` 里 109 个文件、259 处 `goto`（I3 第 9 节第 51 条）。
- **帮手**：
  - `page` 夹具记下每个在途的 `/api/v0/` 请求（`request`、`requestfinished`、`requestfailed`），事件流除外。
  - 帮手在 `goto`、`reload`、`goBack`、`goForward` 之前等这些请求答完，其中一定等 `POST /api/v0/auth/refresh`。后三个同样换掉文档、同样打断在途的请求。
  - `pageWatch`（`e2e/fixtures/test.ts`）认出没有预期的 `/sign-in` 落点，失败时写明"被跳回登录页"。
- **门禁**：故事里不许直接调这四个方法，只许经帮手。由 lint 或一个读源码的检查守住，带反向对照（13.4 第 1 条）。
- **对照测试**：把续期的答复延后，再 `goto`。去掉帮手时落到 `/sign-in`，带着帮手时不落。
- **不改产品**：M1 第 10 节的取舍照旧。用户在续期途中刷新同样会被登出，写进已知限制。
- **第 50 条**：`e2e/global-setup.ts:14-66` 的网络等待触发时打一行日志，看 CI 的 runner 上它到底触发了没有（I3 第 9 节第 50 条）。

### 4.5 打磨：分级与范围

照负责人第 1 条与 I3 的分级（主持者的决定 M12-9）。

- **分类**（I3 第 0 节）：
  - A：用户会碰到的错误行为或数据问题；
  - B：无障碍问题或误导的文字；
  - C：纯体验的改进；
  - D：用户碰不到的（测试、结构、日志）；
  - M：只能人工确认的。
- **照 I3 第 9 节的分类**。备注里"给主持者"的条目不改分类，除了下面几条。依据是负责人第 1 条的字面：A、B 进 v0.1。
- **移出 v0.1 的**：
  - 只从 ARIA 推断"念两遍、念得急"的九项（21.15、21.16、22.10、26.1、26.2 的后一半、28.3、16.8、18.4a、49）移到发布之后的读屏验收；确认有问题，在 v0.1.1 修（I3 第 7 节第 1 条）。理由：负责人第 4 条已把读屏放在发布之后；这些没有实测，改了也要靠读屏核对。
  - 罕见又大的 A：H1、24.1、T4，写成已知限制，进 v0.2（I3 第 7 节第 2 条）。
- **移进 v0.1 的**：
  - 第 48 条（横幅出现时正文跳 94 像素）算 A，与 3、8.2、17.1 在同一个 Step，改的是同一个 effect（I3 第 7 节第 13 条）。
  - 23.17 与 28.2（`aria-disabled` 的按钮看着可按）是 C，随 23.16 一起改。不改的话，`ConfirmDialog` 换成 `aria-disabled` 之后，发送中的按钮看着能按，是一个新的误导（I3 第 9 节 23.17 的备注）。
  - H4 的界面一半：回收站与版本列表写明"v0.1 不能永久删除，回收站的项在期限之后由系统清除"（移交 M8-history 第 4 条要求"在部署文档与界面上写明"；I3 第 5 节第 8 条）。
- **几条做法**：
  - 英文单复数：照已有的 `*One` 键，每个要单数的地方加一个键，不引入 `Intl.PluralRules`（I3 第 7 节第 6 条）。
  - 第 7 条退出登录丢文字：做"仍要退出吗"，本标签页的编辑与经频道报告的都算；Web Locks 的登记进 v0.2（I3 第 7 节第 7 条）。
  - 第 40 条：两份 `AGENTS` 模板都改"三个 prompt"的说法，已有笔记本里的 `AGENTS` 不随之改（I3 第 7 节第 12 条）。
  - 33.1（Safari 窄屏的搜索页宽到 704）：在 P2/S3 里用 Playwright 的 webkit 只跑 SE1 的 375 一段排查。复现了就是 B（WCAG 1.4.10），在 S3 里修；没复现，留在发布之后的人工清单（I3 第 7 节第 15 条）。
- **D 类**：只做挡发布或顺手的几项：50、51（P1/S1）、14d（随 P2/S4）、32.3（随 P2/S3）、24.8 与 24.10（随 P2/S6，TB2 偶发的根源）、26.15（随 P2/S6）。其余进 v0.2。
- **13.2 第 12 条**："`ConfirmDialog` 发送中仍用 `disabled`，留到 v0.1 之后的打磨"随 P2/S1 改写。
- **"v0.1 之后"的说法**：13.2 第 12 条与打磨移交里的"v0.1 之后"原意是"M0–M11 之后、在 M12"（I3 第 5 节第 2 条）。M12 的文档一律写"M12（v0.1）"或"v0.2"，不再写"v0.1 之后"。
- **逐项的去处**在附录 A：进 v0.1 的约 112 项（A 31 项、第 48 条、B 80 项），每项写到 Step。

### 4.6 服务端的正确性与历史

#### 4.6.1 第 302 条：回滚的变更集撤销答冲突（H6）

- **字面的改正**（主持者的决定 M12-1）：M11 第 302 条写"v0.1 之内不修"（`00-M11-design.md:949`），本意是"M11 之内不修"。总体设计 5.8 第 6 步写"M12 修"（`v0.1-design.md:580`）。M12 在 v0.1 之内，照负责人第 1 条它是 A，在 M12 修。00-M11 的第 302 条加一句括注指过来，不改写原文（第 7 节"总体设计的修订"第 15 条）。
- **形状**（移交 M8-history 第 6 条）：
  - 回滚每一遍先在自己的单元、自己的时刻删掉不含基线节点的新建子树（`page/app/rollback.go:117-122`），下一遍才删收着基线节点的新建节点。
  - agent 新建一个文件夹、把原有页移进去、又在里面写新页时，新页先被删成一个回收站根，下一遍才删文件夹。
  - 撤销时，M8 的计划在结构上把新页的顶定为文件夹；回收站里文件夹的组却不含新页。`leftDeleted` 判文件夹的组不是这个变更集删的那些（`page/domain/revert.go:502`，`groupIsDeleted` 在 `:536`），又判新页是 `TrashRoot`（`:506`），两个都答 `ConflictChanged`。
- **修法**（I3 第 7 节第 3 条的推荐）：改 M8 的撤销计划，让它认出"同一个变更集更早删下的回收站根"，把它并进顶的组。
  - 只认这一种：这个根的删除是这个变更集的条目；它删得不晚于顶；它的 `Before` 的父节点在顶的子树里，也被这个变更集删了。
  - `RevertGroups`（`revert.go:196`）要一并读这些根的组；`groupIsDeleted` 比较顶的组与这些组的并；放回的次序是先顶的组、再里面的组。
  - 不改 M11 的"一个任务一个变更集"与任务答复的 `changeset_id`，不改契约（I3 第 7 节第 3 条的备选甲要改这两样）。
- **测试**：
  - 性质测试照 M11 的 `TestRollbackPlansPutTheBaselineBack` 与 M8/P3 的随机变更集：随机的树、随机的"先删一页（或一棵子树）、后删它的上级"的变更集；撤销之后的树等于变更集之前的；期望由生成器的模型算出，不读被测的计划（05-lessons 第 1 节）。
  - 变异：去掉新规则；放宽成"任何回收站根都并进来"（应当被一个"别的变更集删的根"的反例抓住）；放回的次序反过来。
  - `TestRevertingARollbackConflictsUnderANewFolderHoldingBaselineAndNewPages` 的 nested 一格翻过来，测试照新的结论改名，仍在第八份（`Makefile:328`）。
- **一并看的**：M8 的后台恢复与后台撤销同样是一个任务一个变更集、分批删，会不会造出同一形状没有造过（M11 收尾审查 A 的存疑）。P3/S2 的 spec 先造这个形状，造得出就一并兜住、加测试。
- **放弃要不要输入名称**：第 302 条让放弃照旧不要求输入名称。修好之后理由更足，不改。
- **P3/S2 先写 spec**：计划是所有撤销都走的路，改动的边界要先定清。

#### 4.6.2 日志的读放进一个快照（H5）

- 日志的读不开事务、不加锁（`page/app/list_changesets.go:51` 的注释）。现在是五条读：`ListChangesets`（`:192`），加 `page/app/history.go:165-180` 的 `ChangesetFacts`、`FirstChanges`、`FirstIngestSources`、`DisplayNames`（I3 第 3 节 H5）。读跨过一次撤销的提交时，可能列出"已撤销"却没有那条撤销。
- 修法（I3 的推荐）：`ListChangesets.page` 包一层 `WithinSnapshot`（`platform/postgres/tx.go:137-144`，搜索已在用，`page/app/search.go:222`）；`HistoryReads` 加 `Snapshots`。MCP 的 `log {path}` 也走 `page`，一并得到。
- 代价：每次读多占一个只读快照的连接。P4/S3 的日志一项照新的读量。

#### 4.6.3 标题键与标题 id：赶在 v0.1.0 之前（25.1、10.2）

两处都改已有的键或 id。发布之前没有生产的库，改动只碰开发库；之后再改，就要每个部署都 reindex（I3 第 7 节第 4、5 条；主持者的决定 M12-10）。

- **25.1**：`shared.TitleKey` 对切罗基文不幂等（`shared/title_key.go:18-28`）：`cases.Fold()` 把切罗基文的大写折成小写、小写折成大写，`TitleKey(TitleKey(x))` 不等于 `TitleKey(x)`。
  - 三处同样的折叠一起改：`shared/title_key.go`、`shared/title_key_context.go:29`、`linking/domain/index.go:80,97`（I3 第 5 节第 6 条：移交只写了第一处）。
  - 改为在折叠之后把切罗基文的小写换成大写，同 `searchFold` 的映射；然后删掉 `searchFold` 的映射（`page/domain/search.go:245-256`）。
  - 不升分词的版本，也不升 `Extractor`：改了之后 `searchFold` 的映射不再起作用，分出的词不变；升版本只会让 reindex 白白重建全部搜索的行（I3 第 5 节第 7 条；README:534：30 万页约 26 分钟）。
  - 13.1 第 28 条（标题键）随 P3/S1 改。发布说明写一句：v0.1.0 之前的试用库若有切罗基文的标题，跑一次 `nervewiki reindex`。
- **10.2**：标题的 id 丢了自动链接与原始 HTML 的文字（`platform/markdown/parse.go:83-103` 的 `PlainText` 跳过它们，`:174-177` 的 `AnchorID` 用全文）。改为带上它们的文字。
  - 要不要升提取规则的版本（13.3 第 6 条），由 P3/S1 的设计看链接的锚点存在哪里再定。
  - 发布说明写一句：这类标题的锚点 id 变了。

#### 4.6.4 小项

- **10.1**：标签 `/`（写作 `#//`）列得出、叫不到。最小的改法：`listTags` 不列 `/`（`linking/adapter/postgres/queries/reads.sql:87-94`），补全随之不列（I3 第 9 节 10.1）。
- **30.2**：`shared/name.go:36-38` 的 `Unshowable` 只去掉 Cc、Zl、Zp 与 Bidi_Control。由看不见的字符组成的客户端名与显示名可以冒充空白。扩到默认可忽略的字符与只有空白的名字，只管新的输入。
- **30.4、30.5**：MCP 的 `log {path:""}` 与 `{path:"/"}` 答"path is required"（`page/app/list_changesets.go:132-134`）；`index` 的筛选给 `""` 答 `too_short`（`linking/app/list_index.go:176`，工具说明写着 `"" is refused`，`linking/adapter/mcp/tools.go:62`）。定一种约定（主持者的决定 M12-11）：可选的参数省略即"不筛"；给了空串或 `"/"` 一律答字段错误，文字写"省略它就是整本"。细节在 P3/S1 的设计里。

#### 4.6.5 每个账户的事件流数（移交 M5-performance 第 6 条）

- hub 不按账户设上限（`events/app/hub.go:26-31`；`open` 在 `:100-109`，不数）。每条流占一个连接、一个 goroutine 与 64 格缓冲。网页一个浏览器只用一条。
- 加上限（I2-D9；主持者的决定 M12-12）：
  - 每个账户至多 16 条，配置项 `events.max_streams_per_account`（默认 16，至少 1；13.1 第 15 条）；
  - 第 17 条打开时，重置这个账户最老的那条；被重置的客户端照常重连；
  - 记一行 `info`，带账户 id 与条数，不带令牌（13.1 第 10 条）。
- 不选"答 429"：睡眠的笔记本电脑留下的半开的连接要等心跳写不出去才发现断了，其间新开的标签页会被拒。
- 重置用已有的 `reset` 帧（13.1 第 30 条），客户端照常重连、整体刷新。代价：一个账户同时开着 16 条以上活的流时，它们会轮流互相顶，每一次都整体刷新。网页一个浏览器一条流，人碰不到；P3/S1 的设计核对网页重连的退避，保证它不成为一个紧的循环。
- 测试：hub 的单元测试（第 17 条、同一账户的别的流不受影响、别的账户不受影响）；一个经 `/api/v0/events` 的整个程序的测试，名字点名进第七或第八份（第 7 节"分份"）。
- `events.yaml` 与 README 的事件流一节写明上限。13.1 第 30 条随之改。

#### 4.6.6 名称不是 UTF-8 的导入条目（T3）

- 现在跳过并写进报告：`transfer/domain/entry.go:97-101` 答 `name_not_utf8`；`transfer/adapter/archive/archive.go:305` 照原样取 `f.Name`，不看 `NonUTF8`。推断：旧的 Windows 资源管理器以 OEM 码页打包，中文 Windows 上中文名的页与附件会全部被跳过（I3 第 4 节 T3）。
- 修法（I3 第 7 节第 8 条；主持者的决定 M12-13）：条件是"没有 UTF-8 的标志、不是合法的 UTF-8、按 GBK 能解开"时，按 GBK 解读；报告写明哪些名字是这样解读的。CP437 进 v0.2：任何字节串都是合法的 CP437，要另定判别。`golang.org/x/text` 已是依赖。
- 外来格式的读取照 13.4 第 11 条：解读只改原本不是合法 UTF-8 的名字，有差分的测试；预读与标准库的对照照旧。
- 报告的新原因带两种语言的文字，与服务端一起在 P3/S3 加（13.2 第 4、11 条）。

### 4.7 性能与内存实测

照负责人第 2 条（主持者的决定 M12-14）："量到总体设计写的规模；只修超出设计已写预算的，例如页面树超标就改成分层加载；其余记成已知限制，进 v0.2"。负责人 2026-11-06 另说过："不要把这次当做性能问题继续无限优化了，调整测试用例量级或者时限即可"。

#### 4.7.1 预算与超出的判据

写下了预算的只有下表几行（I2 第 3 节）。没有预算的只量、记下。

| 编号 | 预算 | 出处 | 量的规模 | 超出的判据 | 超出时 |
|---|---|---|---|---|---|
| B1 | 搜索：每类查询第一页与"更多"的 p99 ≤ 300 ms；体积 ≤ 正文 3 倍；任务 ≥ 100 页/秒；30 万页从空建 ≤ 1 小时；一页的保存 p99 多 ≤ 2 ms | `M8-history-search/05-P5-search.md:890-902` | 30 万页，两种分布（200 本、5,000 本） | 300 ms 的尾巴与约 5 倍的体积是已接受的超出（`05-P5-search.md:894`、`:899`、`:187`）。只有超出 M8 已记的最大值才算回归：无统计 719 ms，建索引与积压时约 1.2 秒，5,000 本 827 ms（I2-D3）；其余几项照原预算 | 回归要修；没回归，维持已知限制 |
| B2 | 解析预算 8 MiB，最坏约 2.4 GB 堆；取不到等 `parse_max_wait` 2 秒后 503 | `server/configs/config.yaml:95-98`，`v0.1-design.md:1300` | k = 2、4、8、16 个并发的 5 MiB 阅读与保存，普通与病态两种正文 | Go 的堆峰值超过 2.4 GB 加空闲时的基线；或没等满 `parse_max_wait` 就 503 | 修 |
| B3 | 大请求体整个服务两个名额、每凭证一个、等 2 秒；两个合起来约 1 GB | `bootstrap/mcp.go:40-41`，`README.md:627` | 两个名额加排队的，几种形状 | 内存超过约 1 GB 加基线 | 修 |
| B4 | 每个请求 15 秒（`server.request_timeout`） | `server/configs/config.yaml:17` | 正常形状：导入上限的 6 万个节点（`config.yaml:153`）、每页几十条链接 | 正常形状下删除、移动、改名到了期限（I2-D6） | 修（照用户碰到的错误）；病态形状（20 万个节点的子树、400 页 × 2,000 条链接）记成已知限制 |
| B5 | 满负载下一个单元 ≤ 5 秒（`MaxUnitNodes` 1 万、`MaxUnitHistory` 40 万、`MaxRevertSteps` 1,500、`MaxJobItems` 6 万） | `v0.1-design.md:1171`；`page/domain/trash.go:57`、`:66`，`revert.go:58`，`jobs.go:29` | 各界线处，本机 | 本机两轮的较大中位数超过 5 秒 | 照 M8 的规则降界线 |
| B6 | lint 名额与期限 `min(10 秒, request_timeout/2)`，生产 7.5 秒 | `bootstrap/wire.go:263-264`，`M10-llm-wiki/00-M10-design.md:295` | 10 万页（设计规模）；30 万页只量 | 10 万页上每条规则与摘要过 7.5 秒 | 修 |
| B7 | 升级的发起与批准在请求期限的 3/4 之内，生产 11.25 秒 | `bootstrap/upgrade_start_cost_test.go:31-40`（M11 第 232 条） | 10 万页；30 万页只量 | 10 万页上过 11.25 秒 | 修 |
| B8 | 导入导出一个任务 ≤ `transfer.job_timeout` 6 小时 | `config.yaml:144` | 导入的上限：5 万个条目、512 MiB 的 zip、4 GiB 解压（`config.yaml:151-157`）；几 GB 的导出 | 过 6 小时 | 修 |
| B9 | 编辑器的段落、标题每次解析至多约 80 ms | `v0.1-design.md:1227` | Chromium 里的最坏形状 | 过 80 ms | 修 |
| B10 | 日志读的守卫 200 ms；`index` 的守卫 3.5 秒 | `v0.1-design.md:1275`；移交 M4-performance 第 18 条 | `index` 在 10 万页；按页的日志 10⁵、10⁶ 个变更集只量 | `index` 在 10 万页上过 3.5 秒 | 修；按页的日志约 13–25 万个变更集时过 200 ms，记成已知限制，迁移进 v0.2（I2-D7） |
| B11 | lint 十万页摘要 TotalAlloc 的"初始 250 MiB" | `v0.1-design.md:1275` | — | — | 不是产品的预算：它被守卫的 1,571 MiB 取代，358 MiB 记下，不修（I2-D8） |
| — | 页面树 | 第 14 节"上万页"（`v0.1-design.md:1316`），没有数 | 见 4.7.2 | 见 4.7.2 | 见 4.7.2 |

B4、B5 是硬界与定界线的规则，不是体验的目标：过了 15 秒是 500 或 503，什么也不写（I2 第 3 节）。

**只量、不判的**（移交的条目，归类照 I2 第 5 节）：M4-performance 第 1、5–8、10–13、15 的日志、16 的整个任务、18 的多数分项、19 的 30 万页、20、21 的 30 万页；M5-performance 第 3 条；M7-transfer 第 4、6 条；M8-history 第 2 条（版本的增长）；13.4 第 9、12 条的旧版本下的通用计划。它们进已知限制、部署文档的数与 v0.2 的移交。

#### 4.7.2 页面树的界线（量之前写死）

主持者的决定 M12-2：第 14 节的"上万页"定为 1 万页。下面的界线是设计者提的（设计者的取舍 M12-26），审查者核对；P4/S2 的设计照抄，量之前不再改。

- **规模与形状**：1 万页，三种形状：
  - 平：1 万页都在根下；
  - 深：10 个根、各 10 个文件夹、各 100 页，另有一个 1,000 个孩子的文件夹；
  - 带附件：深的形状另加一成附件（1,000 个）。
- **环境**：本机，Playwright 自带的 Chromium，生产构建（`make build`），默认视口，不降 CPU。
- **三条界线**：
  - **T1 冷打开**：从 `goto` 笔记本首页（已登录、HTTP 缓存清空）到左栏根下的行全部进入 DOM，≤ 1.5 秒。
  - **T2 别人改一次树**：另一个账户经 API 新建一页。从答复到这个标签页画出新的一行 ≤ 1.5 秒（含 refresher 的 500 ms，`web/…/events/handlers.ts:36`）；其间主线程最长的一个长任务 ≤ 200 ms。
  - **T3 展开**：展开那个 1,000 个孩子的文件夹，到孩子都画出 ≤ 500 ms。
- **只记的**：JS 堆（CDP 的 `JSHeapUsedSize`）；线上的字节，代理开压缩与不开各一次；CPU 降速 4 倍时的 T1–T3；持续改树（每秒一页、60 秒）时主线程忙的比例；5 万、10 万页的全部各项。
- **怎样判**：每条界线在每种形状上判。两轮，每轮 5 次取中位数，两轮的较大中位数对照界线。
- **结论**：
  - 1 万页三种形状都过：保留整棵树。5 万、10 万页的数记成已知限制，分层加载进 v0.2。
  - 有一条没过：在 v0.1 改，先写设计。改法看没过的是什么：数据一侧（读、比较、索引）多半是分层加载；只有"平"的形状在渲染上没过，多半是只渲染看得见的行（虚拟列表或分批）。改法定了之后，P2/S6 照它排。
- **代价最低的缓解**：部署文档的两份代理样例给 JSON 开压缩（I2 第 2.5 节）。它不改结论，只写进文档。
- 依据：I2 第 1.2 节的小样里，数据一侧在 1 万页约 20 ms，渲染没有量（I2 第 2.3 节）；I2-D1 的推荐。

#### 4.7.3 方法：机器、规模、种子、两轮

- **机器**（I2-D2；主持者的决定 M12-15）：以本机为参照（aarch64，8 vCPU，30 GiB，Parallels 虚拟机；I2 第 1.1 节）。CI 的 EPYC 7763 作"慢的主机"的代表，按移交里记的两者之比（lint 1.16–2.33 倍，守卫 2.1–4.6 倍）推算。部署文档写"以下数字在 X 上量得"。没有真的生产机，不另租。
- **之前**：`free -h`（交换已满，噪声会大）、`df -h /`（搜索一批约 20 GB，先修剪 Go 的构建缓存，06-environment 第 3 节）；其间不跑别的构建与测试；全部经 `heavy.sh`。
- **被量的**：生产构建的 `nervewiki serve`（`NWIKI_ENV=prod`，默认配置，另写的除外），PostgreSQL 18.7 的容器，建库参数同测试（builtin `C.UTF-8`、LZ4）。
- **种子**：每批一个固定的种子，写进记录；生成器同一个种子出同样的数据。
- **两轮**：每批两轮，各自重启服务进程。时延报两轮的 p50、p99，对照预算取较差的一轮；时长取两轮的较大中位数；内存取两轮的较大峰值。
- **内存**（I2-D14）：外部每 100 ms 读进程的 `VmRSS`、`VmHWM` 与容器的 cgroup `memory.peak`；Go 的堆用 `GODEBUG=gctrace=1`。产品里不加 pprof 或 metrics 端点。
- **浏览器**：Playwright 与 CDP（`Performance.getMetrics`、`PerformanceObserver` 的 `longtask`）。
- **批次**（I2 第 6 节）：A 页面树与浏览器各项、B 搜索、C lint 与 `index` 与升级、D 回收站与撤销与后台、E 并发与内存、F GB 级的导入导出、G 版本的增长、H 迁移在大库上的停机。合计约 10–16 小时机器时间（推断）。H 只在负责人说有 v0.1.0 之前的实例要升级时才量（4.8）。
- **CI 不跑**（I2-D5）：CI 不加 10 万、30 万页的守卫。只有修了东西的 Step 照负责人 2026-11-06 的话加缩小规模的守卫。

#### 4.7.4 设施进仓库

照 I2-D4（主持者的决定 M12-16）。可复现、可审查；"Codex 的证据不在 git 里"的教训。

- **Go 的生成器与基准**：在组合根写 `_test.go`，没有环境变量就跳过，同 `BenchmarkSearch`（`bootstrap/search_bench_test.go:66-69`）：
  - 合成的大笔记本：1 万、5 万、10 万、30 万个节点；链接、附件、历史可调；
  - 合成的搜索语料：写成 `BenchmarkSearch` 读的三个 TSV，200 本与 5,000 本两种分布；词表取仓库里的 Markdown（中文的设计文档与英文的代码注释），按 Zipf 抽样，不下载语料（`04-owner-decisions-and-preferences.md` 第 4 节）；
  - GB 级的 vault、几年的版本增长。
- **规模开关**：现有的守卫（升级、lint 等）加一个只在手跑时用的环境变量（例如 `NW_SCALE_PAGES`），默认仍是今天的常量（`upgrade_start_cost_test.go:41-44` 等）。
- **外部的驱动与采样**：放在 `server/tools/perf/`，只经 HTTP 打一个在跑的 `serve`：N 个编辑者自动保存加树写、k 个 5 MiB 的阅读与保存、MCP 的大请求体、N 条事件流同时重连、并发上传。采样的脚本同目录。
- **浏览器的计时脚本**：`e2e/perf/` 与它自己的 Playwright 配置，不进 `make e2e`；tsc、knip、lint 照常看它们。
- 全部合并之后，CI 只多编译，不多跑。

#### 4.7.5 修与不修

- 超出 4.7.1 的判据的，在 v0.1 修，各开一个条件的 Step（P4/S4 起），先写设计，带缩小规模的守卫。负责人可以改判为已知限制。
- 守卫的规模与时限照负责人 2026-11-06 的话调，不为它优化。
- 只量的，写进 P4 的记录、已知限制与 v0.2 的移交。
- 已经知道的超出项（搜索的尾巴、日志的 13–25 万个变更集、`DeleteNode` 与单个 `move` 的病态形状、lint 的 TotalAlloc）照 4.7.1 的处置，不重开。

### 4.8 部署与备份文档

照 I1-D13、D15–D18（主持者的决定 M12-17）。

- **`deploy/README.md`** 作运维手册，目录：
  1. **要求**：硬件分两档，默认配置与小机器（`page.parse_budget_bytes` 5 MiB 加 `GOMEMLIMIT`），数取自 P4；PostgreSQL 18，以 builtin provider 的 `C.UTF-8` 建库、带 LZ4（`platform/postgres/check.go:53-66` 不满足就拒绝启动），托管的 PostgreSQL 要能这样建；磁盘：正文约 5 倍给搜索、版本的增长、导出与导入的临时文件、WAL；v0.1 只支持单实例。
  2. **安装**：拉镜像并校验签名；建库；两个角色与 `deploy/runtime-grants.sql`；签名私钥；配置（`NWIKI_ENV=prod`、`server.public_url`、`server.trusted_proxies`）；`migrate up`；第一个管理员（`nervewiki users`，链到 README 的命令）；`serve`；`deploy/compose.yaml` 样例。
  3. **反向代理与 HTTPS**：nginx 与 Caddy 的整站样例，从根 README 移过来（`README.md:554-616`）；JSON 开压缩；几个 GB 的导出下载的超时与缓冲（移交 M7-transfer 第 3 条）；大的 MCP 请求体与正文写较多时推荐 nginx，经 Caddy 时早答的拒绝成 502 的代价写明（I1-D15、I2-D13；服务端的改法进 v0.2）。
  4. **备份与恢复**：命令（`pg_dump -Fc`、`pg_dumpall --roles-only`、先库后 `blobs/`）；恢复到 builtin 的库；恢复之后先 `ANALYZE` 再开放服务；私钥与配置也要备份；一次演练的步骤。
  5. **升级与回退**：通用的升级（拉新镜像并校验、`docker stop -t 40`、备份、`migrate up`、补授权、有说明时 `reindex`、启动、看 `/readyz` 与版本号）；回退靠恢复备份，`migrate down` 不保证数据；PostgreSQL 的小版本（18.7 的 GIN 修复：怀疑损坏时 REINDEX）与大版本（`pg_upgrade` 或 dump/restore、builtin locale、复核"一个事务里几条 `NOTIFY` 只取一次全局锁"，移交 M5-performance 第 5 条）。
  6. **运行与监控**：探针；日志；磁盘与 507；停机的次序与 `docker stop -t 40`；`server.shutdown_timeout` 20 秒短于一次 `/mcp` 调用（至多约 45 秒），停机时还在传大请求体的调用被切断、整个回滚，要它们做完就把两者一起调长（移交 M7-transfer 第 6 条）；连接数；`GOMEMLIMIT`；连接池代理（PgBouncer）只能用会话模式（移交 M4-performance 第 10 条）；校时：下载地址的到期按客户端的时钟比较（移交 M7-transfer 第 4 条）。
  7. **容量**：P4 的数（内存、磁盘、WAL、版本的增长）。
  8. **运维相关的已知限制**：单实例；没有总量配额，由管理员监控磁盘；结束的任务行不清理；没有永久删除与它的变通（I3 第 3 节 H4）；队列按人不设上限，管理员可以取消别人的任务（T2）；River 取到之后停掉的任务占名额约 7 小时（T4，`README.md:193`）。
  - 附录"从 v0.1.0 之前的开发版升级"：只在负责人说有这样的实例时写（第 11 节"需要负责人确认"），照它实际所在的迁移号核对；那时 I2 的批次 H 才量。没有的话，根 README 的六节开发版升级说明（`README.md:526-543`）直接删掉，它们在 git 的历史里。
- **单实例**（I1-D18）：写明 v0.1 只支持单实例与原因：附件在本地磁盘；限流、大请求体的名额、正在上传的导入只在进程内计数。多实例之间的时钟一条（移交 M5-performance 第 2 条）因此不适用。
- **根 README**：
  - "部署"一节（`README.md:497-632`）缩成几行，链到 `deploy/README.md`；
  - 示例从本地的 `nervewiki:0.1.0` 改为 GHCR 的地址与标签；
  - "项目处于 v0.1 开发阶段"（`README.md:9`）随发布改；
  - M10 的两段压缩写法（`README.md:499` 等）改成人读的句子或移走，"M12 实机复核"的说法照 P4 的结论改成已知限制（I1 第 8 节第 5 条）。
- **`deploy/compose.yaml`**：PostgreSQL（builtin locale）、一次性的 `migrate`、`serve`、卷、`stop_grace_period: 40s`。`make deploy-check` 跑 `docker compose -f deploy/compose.yaml config -q`，CI 的 `image` 一份跑它（13.5 第 1 条：新门禁同时进 Makefile 与 CI）。
- **演练**：P5/S3 照 `deploy/README.md` 在本机从零装一遍、备份再恢复一遍，用本地构建的镜像（`make image` 要在普通的克隆里跑，`Makefile:89`）；推 rc 之后用 GHCR 的镜像再装一遍。

### 4.9 版本号

照 I1-D8（主持者的决定 M12-18）。

- **唯一的注入点**是 ldflags 写 `buildinfo.version`。默认值 `0.1.0-dev` 有四份：`Makefile:12`、`platform/buildinfo/buildinfo.go:9`、`deploy/Dockerfile:29`、`:42`，没有守卫（I1 第 1 节）。
- **守卫**：一个短测试核对四份一致，带反向对照（P5/S1）。不去掉 Dockerfile 的 `ARG` 默认值：直接 `docker build` 的人仍得到一个能用的镜像。
- **打 tag 时不改版本号**：发布的版本由 tag 经 ldflags 注入（4.10）。
- **发布之后**，`main` 的四份默认值改为 `0.1.1-dev`，`api/modules/instance.yaml:80` 的示例随之（负责人第 4 条：验收发现的问题出 v0.1.1）。
- **Go 的模块**：`server/go.mod` 是子目录的模块，仓库根的 `v0.1.0` 不给它定版本；`go install` 出来的二进制没有内嵌前端（`webui/dist` 只提交了 `.gitkeep`）。不打 `server/v0.1.0`，发布说明不宣传 `go install`（I1 第 1 节）。
- **接口的版本**：`api/openapi.yaml` 的 `info.version: v0` 是接口的版本，与产品版本无关，不改。

### 4.10 发布流水线

照 I1 第 4 节与 D1–D7、D14（主持者的决定 M12-19）。

- **镜像名与标签**（I1-D1）：`ghcr.io/open-nerve/nervewiki`（仓库名有大写，镜像名必须小写）。`v0.1.0` 打 `0.1.0` 与 `0.1`；rc 只打 `0.1.0-rc.1`。**不打 `latest`**：升级要先停机、备份、`migrate up`，prod 不自动迁移；`latest` 会诱使人拉了直接重启，新的二进制碰上没迁移的库，`/readyz` 一直 503。`0.1` 随补丁版本前移，补丁版本同样可能带迁移：部署文档要求生产写确切的版本号（`0.1.0`），照升级的步骤换版本。
- **触发**（I1-D2）：
  - `release.yml`：`on: push: tags: ['v[0-9]+.[0-9]+.[0-9]+*']`。
  - `ci.yml` 改为 `on.push.branches: ['**']`，tag 不再触发（今天 `on: push` 不带过滤，`.github/workflows/ci.yml:3-5`，推 tag 会多跑一遍完整的 19 项）。
  - 不加 GitHub 的 environment 审批：负责人已要求推 tag 之前问他一次。
- **闸门**（第一个作业）：
  - tag 名是 semver，`VERSION=${GITHUB_REF_NAME#v}`；
  - tag 指的提交是 `origin/main` 的祖先；
  - 这个 SHA 在 `main` 上的 push 的 CI 已成功（经 `gh api` 查，要 `actions: read`）；
  - `make vuln` 为绿。
  - 闸门写成一个脚本（`deploy/release-gate.sh`），可以在本机与分支上对着假的输入跑。
- **两种架构**（I1-D3）：矩阵 `ubuntu-24.04` 与 `ubuntu-24.04-arm`，各自原生构建。Dockerfile 不必改成交叉编译；smoke 在真的 arm64 上跑；公开仓库的 arm64 runner 免费。每个架构：
  1. 检出，`persist-credentials: false`；
  2. `docker/setup-buildx-action`、`docker/login-action`；
  3. `docker/build-push-action`，`context: .`（**路径上下文**：它默认的 Git 上下文会忽略之前对文件的改动，推断也不带 `.git`，VCS 信息因此缺失、`image-smoke.sh` 会失败；要在演练里核实），按摘要推送、不打 tag，`provenance: false`、`sbom: false`（用 GitHub 的证明，免得两套）；
  4. 以 `deploy/image-smoke.sh ghcr.io/open-nerve/nervewiki@<digest> $VERSION` 拉回刚推的摘要来 smoke。smoke 过的就是推上去的；失败时 GHCR 里只留一个没有 tag 的摘要。
- **合并**：`docker buildx imagetools create` 用两个摘要建出多架构的索引，再打 tag。
- **来源证明与 SBOM**（I1-D4、D5）：`actions/attest`，`subject-name: ghcr.io/open-nerve/nervewiki`、`subject-digest: <索引的摘要>`、`push-to-registry: true`。
  - 镜像的 SBOM 由 syft（`anchore/sbom-action`）从镜像生成，覆盖二进制里的 Go 模块。
  - 前端的生产依赖另出一份：打包进二进制的 npm 包在镜像里看不出来（vite 的产物里没有 `package.json`）。用什么工具，P5/S2 的设计里先做个小实验。
  - 两份都作 attestation，也作 Release 的附件。
- **签名**（I1-D4）：`sigstore/cosign-installer` 加 `cosign sign --yes ghcr.io/open-nerve/nervewiki@<digest>`，keyless。负责人原话列了"签名、SBOM 与来源证明"三样，这样做一一对应。
- **第三方许可证**（I1-D14）：镜像里放一个 `/licenses`（Go 模块与前端生产依赖的许可证文本），同一份作 Release 的附件。理由（推断，非法律意见）：MIT、BSD、Apache-2.0 都要求分发时附上版权声明；SBOM 只列标识，不含文本。怎样收集在 P5/S2 的设计里定，只读本机的模块缓存与 `node_modules`，不往外发。
- **Release**（I1-D6）：runner 自带的 `gh release create "$GITHUB_REF_NAME" --verify-tag --notes-file <文件>`，rc 加 `--prerelease`。
  - 说明取 `CHANGELOG.md` 里这个版本的一节，后面接工作流生成的一段：索引的摘要与两条校验命令。
  - 附件：两份 SBOM、许可证。
  - 不附独立的二进制：部署文档只覆盖容器；附二进制要另做签名、证明与运行说明。
  - `v0.1.0` 不标 pre-release。
  - 不用第三方的 release action。
- **发布之后的核对**：最后一个作业在 amd64 上按 tag 拉索引，跑 `gh attestation verify oci://ghcr.io/open-nerve/nervewiki:<tag> -R open-nerve/NerveWiki` 与 `cosign verify …`（`--certificate-identity` 指 `release.yml` 在这个 tag 上，`--certificate-oidc-issuer=https://token.actions.githubusercontent.com`），再 smoke 一次。arm64 由本机核对（4.11）。
- **权限**：只用 `GITHUB_TOKEN`，不要别的机密。顶层 `permissions: {}`，各作业按需开：

  | 作业 | 权限 |
  |---|---|
  | 闸门 | `contents: read`、`actions: read` |
  | 每个架构的构建与 smoke | `contents: read`、`packages: write` |
  | 合并、证明、签名 | `contents: read`、`packages: write`、`id-token: write`、`attestations: write`、`artifact-metadata: write` |
  | 建 Release | `contents: write` |
  | 发布之后的核对 | `contents: read`、`packages: read`、`attestations: read` |

- **新加的 action**，按 SHA 引用、注释写 tag（13.5 第 2 条；版本与 SHA 见 I1 第 4 节的表与 `survey-i1/actions-new-shas.txt`）：`docker/setup-buildx-action` v4.4.1、`docker/build-push-action` v7.4.0、`docker/login-action` v4.8.0、`docker/metadata-action` v6.2.0、`actions/attest` v4.2.2、`sigstore/cosign-installer` v4.1.3、`anchore/sbom-action` v0.24.3、`actions/download-artifact` v8.0.2。实施时再核一次最新版。
- **镜像的 label**：补 `org.opencontainers.image.revision`、`created`、`description`（今天只有 title、version、source、licenses，`deploy/Dockerfile:43-46`）。由构建参数传入，`make image` 与 `release.yml` 都传。
- **`CHANGELOG.md`**（主持者的决定 M12-20）：仓库根，一个版本一节，中文（同 README）。v0.1.0 一节写：做了什么（按 M 一句）、已知限制（4.13）、人工验收在发布之后（4.12）、两处改了已有键与 id 的事（4.6.3）。
- **`SECURITY.md`**：怎样报告漏洞（经 GitHub 的私密漏洞报告，要负责人在仓库设置里开，第 11 节）、支持的版本（0.1.x）。
- **演练**（I1-D7）：见第 9 节"发布流水线怎样在不推送的情况下验证"。

### 4.11 发布的顺序与对外的步骤

主持者的决定 M12-3、M12-21。

| 步 | 做什么 | 谁 | 对外？要负责人点头？ |
|---|---|---|---|
| 1 | M12 收尾审查、修复、合并，`main` 的 CI 绿；`CHANGELOG.md` 写上日期 | 主持者 | 否（推 `main` 是日常） |
| 2 | 问负责人：推 `v0.1.0-rc.1` 与 `v0.1.0` 两个 tag，一起问（I1-D7）；同时请他做第 11 节列的 GitHub 设置里"推 rc 之前"的几项 | 主持者问 | **要点头** |
| 3 | 推 `v0.1.0-rc.1`：GHCR 里有了镜像（私有）、Sigstore 的公开透明日志里有了签名的记录、一个 pre-release 的 Release | 主持者 | **对外**（第 2 步已点头） |
| 4 | 核对 rc：发布之后的核对作业（amd64）；本机（arm64）登录拉取、`gh attestation verify`、`cosign verify`；照 `deploy/README.md` 用 GHCR 的镜像从零装一遍 | 主持者 | 否 |
| 5 | 负责人把 GHCR 的包设为公开（**不能撤回**），之后本机不登录再拉一次 rc | 负责人 | **对外**，负责人自己做 |
| 6 | 推 `v0.1.0`，同第 4 步核对 | 主持者 | **对外**（第 2 步已点头） |
| 7 | `main` 的默认值改为 `0.1.1-dev`（代码提交，CI 绿） | 主持者 | 否 |
| 8 | 收尾记录写"发布"一节（run 号、摘要、校验的输出）；12.6 写"实现完成，待人工验收"；推送；停下汇报 | 主持者 | 否 |

- **第 2 步的点头盖住什么**（主持者的决定 M12-21）：rc 有问题时，修了再推 `rc.2`、`rc.3`。只改发布流水线、部署文档或发布说明的，同一次点头盖住；改了产品代码的，再问一次。
- **rc 不过时**：不删 tag、不删 Release，标明作废，修了推下一个 rc。GHCR 里留着的 rc 镜像不影响 `0.1.0`。
- **`v0.1.0` 推了之后发现问题**：不重推同一个 tag，出 `v0.1.1`。
- **公开的时机**：包第一次推送时默认私有（GitHub 文档）。在 rc 核对过之后、`v0.1.0` 之前设为公开：这样匿名拉取在 rc 上就核对了，`v0.1.0` 的说明里的命令一发出就能用。

### 4.12 人工验收在发布之后

照负责人第 4 条（没有选推荐的那项）。

- **先发 v0.1.0**，之后做人工验收：
  - MCP 真实客户端的清单 A–I、J、K（`docs/v0.1/M9-mcp/manual/P5-mcp-clients-checklist.md`，K 是 M11 加的）；
  - 读屏软件：清单在 P5/S3 新写（`manual/screen-reader-checklist.md`），收 4.5 移过去的九项、M 类里与读屏有关的项与第 39 条；
  - M 类的其余各项（附录 A）。
- **输入法清单**（`docs/v0.1/M4-pages/manual/P6-ime-checklist.md`，M4–M8 的 21 步）：负责人第 4 条没有点名它。主持者的理解是它同样在发布之后（负责人 2026-10-13："IME 清单,我最后 统一验收就可以了"）；列在第 11 节请负责人确认。
- **汇总的入口**：`docs/v0.1/M12-release/manual/README.md` 链到上面每一份，写怎样记结果。
- **发现的问题**出 v0.1.1。v0.1.1 怎样组织（像一个小的 M，有自己的 00 号文档，还是只在 M12 下记），在验收之后、负责人批准开工时定（第 7 节"写给后面的 M"）。
- **状态**：M12 收尾之后 12.6 写"实现完成，待人工验收"；M4–M11 的状态不变；验收通过之后一起改为"已完成"。
- **发布说明写明**：人工验收在发布之后；还没有在真实的读屏软件、真实的输入法与真实的 MCP 客户端上逐项核对过。

### 4.13 已知限制与 v0.2

- **已知限制**写进 `CHANGELOG.md` 的 v0.1.0 一节，按主题写十来句，不逐条列（I3 第 2 节）。来源：
  - P2 推后的 C 类（编辑时没有大纲、没有标签总览、相对时间不刷新、健康度不自动刷新、网页上不显示 MCP 地址等）；
  - 4.5 的 H1、24.1、T4；
  - P3 推后的 CP437；
  - P4 只量的与超出而经负责人改判的；
  - 运维相关的（4.8 的第 8 项）；
  - e2e 揭出的那一条：续期途中刷新会被登出（4.4）。
- **v0.2 的移交**：M12 收尾时在 `docs/v0.2/handoffs/` 写（`status: open`、`from: M12`、`to: v0.2`），按主题分：
  - `M12-polish.md`：C 类 91 项、D 类其余、Web Locks 的登记（第 7 条）、CP437；
  - `M12-performance.md`：只量的各项与它们的数、已知的改法（页面树的分层加载若没做、搜索尾巴的三种改法、按页日志的迁移、共用的参与者界线、MCP 名额的配置项、慢的发送者与 Caddy 早答的服务端改法、附件列表的重读）；
  - `M12-history.md`：H1、H3、H4 的功能、版本的保留策略（带 P4 的增长数）；
  - `M12-transfer.md`：T1、T2、T4、T5、T8；
  - `M12-release.md`：Node 26、功能性的大版本依赖、Dependabot（看负责人的决定）、多实例、独立的二进制。
- `docs/README.md` 的目录结构加 `v0.2/handoffs/` 一行。v0.2 的总体设计开工时先处理它们。

## 5. 接口

M12 不加 REST 操作、MCP 工具与 prompt。改的是几处行为、一个配置项、几个 Makefile 目标与发布物。

### 5.1 REST 与 MCP 的行为

| 处 | 改动 | Step |
|---|---|---|
| 节点的标题键（`shared.TitleKey`，三处） | 切罗基文的小写在折叠之后换成大写；只差切罗基文大小写的两个标题从此冲突（`page.name_taken` 照旧） | P3/S1 |
| 阅读视图的标题 id | 含自动链接与原始 HTML 的文字；这类标题的锚点 id 变了 | P3/S1 |
| 标签列表（linking 的 `listTags`） | 不再列 `/`；`api/modules/linking.yaml` 的说明随之 | P3/S1 |
| 显示名与客户端名 | 由看不见的字符组成的新名字被拒，码照旧是字段的 `invalid`（或照 P3/S1 的设计） | P3/S1 |
| MCP 的 `log` 与 `index` | 可选的路径与筛选：省略即不筛；空串与 `"/"` 一律答字段错误，文字写"省略它就是整本"；`api/mcp/tools.json` 的说明随之 | P3/S1 |
| 日志（`listChangesets`、MCP 的 `log`） | 五条读在一个只读快照里；答复的形状不变 | P3/S1 |
| 事件流（`GET /api/v0/events`） | 一个账户至多 `events.max_streams_per_account` 条，超出时以 `reset` 结束最老的那条；`api/modules/events.yaml` 的说明随之 | P3/S1 |
| 撤销（`revertChangeset`、`startRevert`） | 嵌套的"先删、后删上级"的变更集照常撤销，不再答 `page.revert_conflict` | P3/S2 |
| 导入的报告 | 按 GBK 解读的名字另列，带一个新的原因；两种语言的文字随之 | P3/S3 |
| `GET /api/v0/instance` 的 `version` | 发布的镜像是 `0.1.0`；`main` 之后是 `0.1.1-dev` | P5、发布 |

改说明的每一处都 `make gen`、`make gen-check`（05-lessons 第 8 节）。

### 5.2 配置

- `events.max_streams_per_account`：整数，默认 16，至少 1（13.1 第 15 条：写进内置的 `config.yaml`，带注释；强类型、校验、日志表示）。P3/S1。

### 5.3 Makefile 与 CI

| 目标 | 做什么 | 在哪里跑 | Step |
|---|---|---|---|
| `make vuln` | `govulncheck`，`server/` 与 `server/tools/` 各一次，版本固定 | CI 的 `short` 一份、发布的闸门；**本机不跑**（它把模块清单发给 vuln.go.dev） | P1/S2 |
| `make outdated` | 只读地列出过时的 Go 模块、npm 包、actions 与镜像 | 本机，发布之前人工跑 | P1/S2 |
| `make deploy-check` | `docker compose -f deploy/compose.yaml config -q` | CI 的 `image` 一份 | P5/S3 |
| `make lint` 加 actionlint | 核对两份工作流 | 本机与 CI | P5/S1 |
| 性能的目标（名字在 P4 的设计里定） | 跑 4.7 的生成器、驱动与浏览器脚本 | 只在本机手跑 | P4/S1、S2 |

`ci.yml` 只在分支上跑（4.10）。`release.yml` 是新的工作流（4.10）。

### 5.4 发布物

- **镜像**：`ghcr.io/open-nerve/nervewiki:0.1.0` 与 `:0.1`，`linux/amd64` 与 `linux/arm64` 的索引；label 加 `revision`、`created`、`description`。
- **证明与签名**：GitHub 的来源证明与 SBOM（推进 GHCR）、cosign 的 keyless 签名。校验：
  - `gh attestation verify oci://ghcr.io/open-nerve/nervewiki:0.1.0 -R open-nerve/NerveWiki`
  - `cosign verify ghcr.io/open-nerve/nervewiki:0.1.0 --certificate-identity=https://github.com/open-nerve/NerveWiki/.github/workflows/release.yml@refs/tags/v0.1.0 --certificate-oidc-issuer=https://token.actions.githubusercontent.com`
- **Release**：说明取 `CHANGELOG.md` 的一节，加摘要与两条校验命令；附件是镜像的 SBOM、前端的 SBOM、第三方许可证。
- **仓库里的文件**：`deploy/README.md`、`deploy/compose.yaml`、`CHANGELOG.md`、`SECURITY.md`、`docs/v0.1/M12-release/manual/` 的两份。

## 6. 从 Nerve 借鉴

无。M12 不读兄弟项目 Nerve（`CLAUDE.md` 的规矩）。发布流水线、部署文档与实测都照本项目自己的：M0/P6 的镜像与 image-smoke、M4–M11 的成本守卫与夹具、README 已有的部署一节。

## 7. Phase 划分

五个 Phase、17 个 Step，另有条件的修复 Step，最后是收尾与发布。

| P | 名称 | Step | 交付 | 验证 |
|---|---|---|---|---|
| — | P1 之前 | 两个直接进 `main` 的提交 | heavy.sh 的锁（4.2）；六份移交的"处理进展" | — |
| P1 | 地基 | S1、S2 | e2e 的 `goto` 帮手；工具链与安全更新 | 帮手的对照测试与门禁；两个守卫、`make vuln` 为绿 |
| P2 | 网页的打磨 | S1–S6 | 附录 A 的 A、B 两类，按组件分 | 每项一个会在没修的代码上失败的测试；变异表 |
| P3 | 服务端的正确性与历史 | S1–S3 | 4.6 的各项 | 第 3 节第 4、5 条 |
| P4 | 性能与内存实测 | S1–S3，另有 S4 起的条件修复 | 设施；浏览器与服务端的实测记录；超出的修复 | 4.7.1 每一行有结论；超出的有缩小规模的守卫 |
| P5 | 部署文档与发布流水线 | S1–S3 | 版本号与流水线的构建；推送、签名、证明与 Release；部署文档与发布说明 | 演练两种架构为绿；本机的 registry 演练；从零安装与备份恢复的演练 |
| — | 收尾与发布 | — | 收尾审查；问负责人；rc 与 `v0.1.0`；`0.1.1-dev`；记录 | 第 3 节第 9、10 条 |

### P1 之前

1. **heavy.sh 的锁**（4.2）：`docs: heavy.sh's lock stays in /tmp whatever TMPDIR says (M12)`。
2. **移交的处理进展**（02-workflow 第 1 节第 1 步）：六份移交各加"处理进展"，写本节"移交的落实"的去处，状态仍是 `open`：`docs: M12's hand-offs, each item's place in M12 (M12)`。可以与初稿的提交合在一起，也可以在设计定稿时与总体设计的修订一起；初稿的建议是单独提交、在初稿之前。

### P1 地基

| Step | 内容 | 改的部件 | 单独合并为绿 | 组合根与 CI 的用时 |
|---|---|---|---|---|
| S1 e2e 的 `goto` | 4.4：夹具里的在途记录与帮手、`pageWatch` 认出 `/sign-in`、109 个故事文件换上帮手、门禁与它的反向对照、延后续期的对照测试；`global-setup.ts` 的一行日志（第 50 条） | e2e | 能：只改 e2e，CI 的 e2e 一份证明 | 组合根不变。e2e 作业预计多 0–30 秒（推断：多数 `goto` 之前没有在途的读；今天 6:52–9:23，时限 20 分钟） |
| S2 工具链与安全更新 | 4.3：Go 1.27.4（两份 `go.mod`、Dockerfile、README）；PostgreSQL 18.7（五处、守卫加 README）；三个镜像的摘要；两个 action；`make vuln` 与它报出的模块；`make outdated`；Dockerfile 的 Go 版本的守卫；两处注释的核对；总体设计 7.1 与 13.5 第 2 条 | server、deploy、e2e、CI、Makefile、README、docs | 能：一个分支、一次完整的 CI；PostgreSQL 的五处由守卫一起改，漏一处就红 | 组合根各份照新的工具链与数据库重量：合并之后两次 `main`，过 450 秒照 06-environment 第 4 节挪份。`short` 一份多 `make vuln`，推断 1–2 分钟。`image` 一份要拉新的基础镜像 |

S1 先做：S2 要一次干净的完整 CI，不该被 `goto` 的偶发打红。

### P2 网页的打磨

每个 Step 的逐项清单在附录 A。每个 Step 自带它加的文字的两种语言（13.2 第 4 条）、假服务端的字段与 knip 的干净。

| Step | 内容 | 改的部件 | 单独合并为绿 | 组合根与 CI 的用时 |
|---|---|---|---|---|
| S1 焦点与共用组件 | `ConfirmDialog` 发送中改用 `aria-disabled`，被拒之后焦点还给确认按钮或说明（17 个文件、25 处 `<ConfirmDialog`）；`useForm` 只有横幅的拒绝时焦点给横幅（`web/…/components/form-field.tsx:89-97`）；`button.tsx` 加 `aria-disabled:` 的样式（23.17、28.2）；到达的焦点（12.3、23.14、26.4、28.4、28.5）；其余的焦点项；第 7 条"仍要退出吗"；13.2 第 12 条改写 | web、e2e | 能 | 组合根不变；vitest 多几十个测试；e2e 改到的故事照需要进 `threeBrowsers`，推断多 1 分钟以内 |
| S2 文字 | 单复数（35.1、41.x，每个单数一个 `*One` 键）；根下的叫法（22.11、23.3）；撤销的用词（23.1、23.4、23.11、24.7）；其余误导的文字；第 40 条两份 `AGENTS` 模板；H4 的界面一句 | web、server（`wiki/domain/template-*-agents.md`） | 能 | 组合根不变（模板的测试在 wiki 模块里）；vitest 多一些 |
| S3 375 px、`bdi` 与对比度 | flex 项补 `min-w-0` 或 `[overflow-wrap:anywhere]`（26.17、32.x、35.8、38.10–38.12）；从右到左的名字进 `bdi`（26.12、32.4）；焦点环与补全说明、暗色高亮的对比度（12.4、26.9）；32.3 MC3 的 375 检查；33.1 在 webkit 上排查 | web、e2e | 能 | 组合根不变；e2e 加几处 375 px 的检查，33.1 的排查不进套件 |
| S4 阅读视图（**先写 spec**） | `reading-view.tsx:133-232` 那个 layout effect 的重写：更新不整段重写 `innerHTML`、不失焦（3、8.2、12.2、29）；每小时的整篇替换与全屏退出（17.1、17.8）；`editable` 与 `role` 不进那个 effect（48，改法见移交第 48 条）；后退前进到锚点（8.1）；mermaid 的 id（11.1）；失去角色后的勾选（6.2）；换语言时的失焦（11.6）；按职责拆开（14d） | web、e2e | 能；要对照 M6 以来依赖 `editable` 的增强逐一核对（移交第 48 条） | 组合根不变；e2e 加阅读视图的焦点与滚动检查，三种引擎（05-lessons 第 6 节） |
| S5 编辑器、附件、上传、粘贴与导入导出 | 附件一节、阅读视图里的附件、编辑器的上传与粘贴、导入对话框与任务列表的 A、B（附录 A）；上传的超时取多长在 S5 的设计里定（16.3） | web、e2e | 能；可按"附件""编辑器""导入导出"分三组提交 | 组合根不变 |
| S6 版本、回收站、历史、目录、后台任务与搜索的网页 | 22.14 与 23.10 的"在途的写进 store"；12.12 的 `Math.max(...)` 与 `push(...)` 的参数上限（`web/…/stores/page-tree.ts:116`、`:124`）；27.2；删除与新建之后的导航（2）；其余 A、B；24.8、24.10（TB2 的偶发）、26.15 | web、e2e | 能；**排在 P4/S2 的页面树结论之后**；可按版本与回收站、历史与目录、搜索分组 | 组合根不变 |

`en.ts`、`zh-CN.ts` 几乎每个 Step 都改：按序合并，后合并的先 `git merge main` 解决冲突（02-workflow 第 3.6 节）。

### P3 服务端的正确性与历史

| Step | 内容 | 改的部件 | 单独合并为绿 | 组合根与 CI 的用时 |
|---|---|---|---|---|
| S1 小的正确性 | 25.1（三处折叠，删 `searchFold` 的映射）、10.2、10.1、30.2、30.4 与 30.5 的约定（4.6.3、4.6.4）；H5 的快照（4.6.2）；每个账户的流数（4.6.5）；13.1 第 28、30 条 | server、api（说明）、web（若有新的文字） | 能 | 单元与适配器的测试为主；流数的整个程序的测试点名进第七或第八份，推断多 5 秒以内 |
| S2 第 302 条（**先写 spec**） | 4.6.1：撤销计划认出同一个变更集更早删下的回收站根；性质测试；nested 一格翻过来；后台恢复与撤销的同一形状；总体设计 5.8 第 6 步与 13.1 第 2 条 | server（page 的 domain 与 app） | 能；nested 一格在同一个提交里翻过来 | 性质测试在 page 的 domain，`-short` 照跑，时长定在 20 秒以内；组合根只改第八份已有的测试，推断不变 |
| S3 名称不是 UTF-8 的导入条目 | 4.6.6：GBK 回退、报告的新原因与两种语言的文字、差分的测试 | server（transfer）、api、web | 能 | 适配器的测试；整个程序的导入测试若加，点名进第七或第八份 |

P3 的三个 Step 与 P2 改的部件不重叠，可以并行。S1 与 S2 都改 page 模块，S2 在 S1 之后开或在合并时 `git merge main`。

### P4 性能与内存实测

| Step | 内容 | 改的部件 | 单独合并为绿 | 组合根与 CI 的用时 |
|---|---|---|---|---|
| S1 设施（Go） | 4.7.4：合成的大笔记本、搜索语料（三个 TSV）、GB 级的 vault、版本的增长；现有守卫的规模开关；`server/tools/perf/` 的驱动与采样 | server（`_test.go` 与 `server/tools/perf/`）、Makefile | 能：没有环境变量就跳过，CI 只多编译；lint 要过 | 组合根多编译，不多跑；推断各份不变 |
| S2 浏览器侧的设施与实测（**页面树先**） | `e2e/perf/` 的脚本与配置；先量页面树（4.7.2，量之前把界线写进 P4 的设计），结论先报主持者；再量 M4-performance 第 7、9、11、12 条，M5-performance 第 3 条的浏览器部分，M7-transfer 第 4 条；记录 | e2e（perf）、docs | 能：脚本不进 `make e2e`；记录只改文档 | 不变 |
| S3 服务端实测（P3 合并之后） | 4.7.3 的批次 B–G（H 看负责人的回答）；每一行对照 4.7.1；部署文档要的数；已知限制与 v0.2 的清单；总体设计第 14 节写着 M12 的各行照结论改 | docs | 能：只改文档 | 不变；本机约 10–16 小时机器时间（推断），独占 heavy 的锁 |
| S4 起：条件的修复 | 只在超出 4.7.1 或 4.7.2 的判据时开，各自先写设计、带缩小规模的守卫。可能的：页面树的改法、参与者的共用界线（B4）、附件列表的重读（I2-D10 判为 A 时）、搜索的回归 | 看改法 | 每个都要能 | 守卫照 13.4 第 8 条进成本或守卫的一份 |

S2、S3 的"实测"Step 照常有分支、审查与合并：审查者核对方法、种子与脚本，抽一两项在自己的副本上重跑，不重跑全部。S2 与 S3 可以写在同一时段，跑要串行（heavy 的锁）。

### P5 部署文档与发布流水线

| Step | 内容 | 改的部件 | 单独合并为绿 | 组合根与 CI 的用时 |
|---|---|---|---|---|
| S1 版本号与流水线的构建部分 | 四份默认值的守卫（4.9）；Dockerfile 的三个 label 与 `make image` 传它们；`ci.yml` 只在分支上跑；`release.yml` 的闸门（`deploy/release-gate.sh`）与两种架构的构建与 smoke，分支上以 `push: false` 演练（只在改了工作流或 `deploy/` 的推送上跑）；actionlint 进 `make lint` | deploy、CI、Makefile、server（守卫在 buildinfo 的测试） | 能：演练作业在分支上证明两种架构为绿；不推送 | 组合根不变；改了工作流或 `deploy/` 的推送多两个构建与 smoke 的作业（推断各 10–15 分钟，与别的作业并行） |
| S2 推送、签名、证明、SBOM、许可证与 Release | 合成索引与打 tag；`actions/attest`；cosign；镜像与前端两份 SBOM；`/licenses`；`gh release create`；发布之后的核对作业；本机以本地的 registry 演练推送、合成、签名（key 签、不上传透明日志）与 syft，记录 | CI、deploy | 合并时为绿（分支上不触发这部分）。这条路径**只有推 tag 才证明得了**，所以要 4.11 的 rc | 不变 |
| S3 部署文档与发布说明 | `deploy/README.md`；`deploy/compose.yaml` 与 `make deploy-check`；根 README 缩写与改地址；第 7 节"移交的落实"的每条部署项；开发版的附录（看负责人的回答）；`CHANGELOG.md` 的 v0.1.0 一节（已知限制取自 P2–P4）；`SECURITY.md`；`manual/` 的读屏清单与汇总入口；从零安装与备份恢复的演练；总体设计第 11 节 | docs、deploy、README、CI | 能：只改文档与一个检查 | `image` 一份多 `make deploy-check`，几秒 |

P5/S1 在 P1 之后就能开，与 P2–P4 并行。S3 等 P4/S3 的数。

### 收尾与发布

照 02-workflow 第 4 节与 4.11：

1. 三位审查者并行：A 服务端（跨 Phase 的一致性、302 的修法与撤销的其余路径）；B 网页与 e2e（打磨的回归、三种引擎的焦点）；C 完成标准、文档、移交的落实与发布流水线（对照第 3 节逐条，对照代码核对每份移交）。
2. 修复在分支 `m12-closeout`，提交写 `(M12 closeout)`；核对一轮一轮做到干净；合并，`main` 的 CI 绿。
3. 处理移交：六份改为 `done`；写 `docs/v0.2/handoffs/` 的五份（4.13）。
4. 4.11 的第 2–7 步。
5. 收尾记录 `reviews/M12-closeout-review.md`，加"发布"一节；12.6 写"实现完成，待人工验收"；推送；停下，给负责人写中文汇报（02-workflow 第 6 节）。

### 次序与并行

```
P1 之前（heavy.sh、移交的处理进展）→ 初稿 → 设计审查 → 定稿
P1/S1 → P1/S2 ─┬─ P2/S1 … S5（彼此按序合并）──────────────┐
               ├─ P3/S1 → P3/S2 → P3/S3 ─────────┐          │
               ├─ P4/S1 → P4/S2（页面树先）──┬───┼→ P2/S6 ──┤
               │                             │   └→ P4/S3 → （P4/S4…）→ P5/S3 ─┐
               └─ P5/S1 → P5/S2 ─────────────┴──────────────────────────────────┴→ 收尾 → 发布
```

- 同时在实施的 Step 不多于两个：本机 8 核，重的运行串行，两个 Step 的门禁与审查已经排队（06-environment 第 3 节）。
- P4/S3 的十几个小时要独占机器：安排在没有别的 Step 在实施与审查的时段，例如夜里。
- 页面树的结论若要改树，改法的 Step 排在 P2/S6 之前，P2/S6 随之等。

### 分份

- 组合根八份。收尾之后两次 `main`（43339247136、43344369124）的平均：325、326.5、402.5、312、425、326、257、267 秒；第五份两次都在 EPYC 7763 上，余量 25 秒。再前两次第四份在 7763 上平均 413.5 秒（[M11 收尾审查](../M11-upgrade/reviews/M11-closeout-review.md)"门禁"一节）。
- M12 新的整个程序的测试**逐个点名进第七或第八份的模式**（`Makefile:296`、`:328`），照收尾时 `TestRevertingARollback…` 的先例。第四份是补集，第五份收 `TestThe(MCP|Lint|WikiTools)`、`TestRebuild` 等（`Makefile:263`），新测试的名字要避开它们。
- P1/S2 合并之后两次 `main` 重看各份；过 450 秒照 06-environment 第 4 节先挪给邻份，挪不下再分一份，不提高 `-timeout`。

### 移交的落实

六份移交在 M12 开工时各写"处理进展"（P1 之前的第 2 个提交），做完的 Step 各加一行，M12 收尾时改为 `done`。

**[M0-P6-image-release](handoffs/M0-P6-image-release.md)**

| 项 | 落实 |
|---|---|
| 1 推送 | `release.yml` 按摘要推送、合成索引、打 `0.1.0` 与 `0.1`；`packages: write` 只在 tag 触发的作业里（4.10），P5/S2 |
| 2 多架构 | 两种原生 runner 各自构建、各跑一次 image-smoke；分支上 `push: false` 演练 P5/S1，推送 P5/S2；不交叉编译（I1-D3） |
| 3 签名、SBOM、来源证明、校验说明 | `actions/attest` 的来源证明与 SBOM、cosign keyless、前端的 SBOM，P5/S2；`deploy/README.md` 与 Release 的说明写两条校验命令，P5/S3、P5/S2 |
| 4 依赖、工具链与基础镜像 | 安全相关的更新与 `make outdated`，P1/S2；Node 26 评估过，v0.1.0 仍用 24（4.3）；Dependabot 由负责人定（第 11 节） |

**[M4-performance](handoffs/M4-performance.md)**（"只量"的都写进 P4 的记录、已知限制与 v0.2 的移交）

| 项 | 落实 |
|---|---|
| 头一段（M10/P3/S1 的成本、旧版本下的通用计划） | P4/S3 只量：生产形状的 MVCC（有旧版本、活着时 ANALYZE）下复测一次（13.4 第 9、12 条） |
| 1 共享锁与树写 | P4/S3 批次 E，只量 |
| 2 大正文的内存 | P4/S3 批次 E，照 B2 判；内存要求写进部署文档，P5/S3 |
| 3 预算先到先得 | 同第 2 项 |
| 4 删子树持锁 | P4/S3 批次 D，照 B4 判（I2-D6） |
| 5 活动的最后写入 | P4/S3，只量 |
| 6 链接多的页的自动保存 | P4/S3 批次 E，只量 |
| 7 补全读整本链接目标 | P4/S2，只量 |
| 8 很多同名的页 | P4/S3 批次 C，只量 |
| 9 编辑器解析的上限 | P4/S2，照 B9 判 |
| 10 ANALYZE、通用计划、连接池代理 | P4/S3 只量；恢复之后先 `ANALYZE`、只能会话模式，写进部署文档，P5/S3 |
| 11 阅读视图的大图 | P4/S2 只量；上传的像素上限进 v0.2 |
| 12 附件列表的重读 | P4/S2 复测（I2-D10）：能稳定让用户的保存失败就是 A，开条件的 Step 修；否则已知限制 |
| 13 每小时整篇替换 | 改法随 17.1，P2/S4；带宽并入第 11 项只量 |
| 14 回收站 | P4/S3 批次 D，照 B5 判；"目标硬件"照 4.7.3 |
| 15 整组撤销与日志 | P4/S3 批次 D，照 B5 判；按页的日志照 B10 记成已知限制（I2-D7） |
| 16 后台的恢复与撤销 | P4/S3 批次 D，每一步照 B5 判，整个任务只量 |
| 17 搜索 | P4/S3 批次 B，照 B1 与已接受的超出判（I2-D3） |
| 18 MCP | P4/S3 批次 C、E，照 I2 第 5.1.1 节的分项；Caddy 早答写进部署文档（I1-D15）；名额的配置项不做（I2-D12） |
| 19 M10 的 LLM Wiki | P4/S3 批次 C，照 B6 判；TotalAlloc 照 B11（I2-D8）；迁移的停机只在批次 H |
| 20 两个守卫的 CI 记录 | P4/S3 的记录写 CI 的余量 |
| 21 M11 的升级 | P4/S3 批次 C，照 B7 判 |
| 22 分份与 `apitest.Load` | M12 不主动做；某一份两次 `main` 的平均到约 430 秒时再做（I2-D11） |

**[M5-performance](handoffs/M5-performance.md)**

| 项 | 落实 |
|---|---|
| 1 多实例 | 部署文档写明单实例与原因（I1-D18），P5/S3 |
| 2 多实例之间的时钟 | 单实例下不适用，部署文档写明（I1-D18），P5/S3 |
| 3 重连与整体刷新 | P4/S3 批次 E，只量 |
| 4 自动保存与树写 | 并入 M4-performance 第 1 项 |
| 5 PostgreSQL 大版本 | 部署文档的大版本升级一节写复核这一条，P5/S3 |
| 6 一个账户的流数 | 4.6.5，P3/S1 |

**[M5-polish](handoffs/M5-polish.md)**

| 项 | 落实 |
|---|---|
| A、B 两类（第 1–49 条的子项） | 附录 A 的去处：P2/S1–S6，服务端的几项在 P3/S1 |
| 第 25 条（标题键）、10.1、10.2、30.2、30.4 | P3/S1（4.6.3、4.6.4） |
| 第 48 条 | 算 A，P2/S4（4.5） |
| 第 50、51 条（e2e） | P1/S1（4.4） |
| C 类 91 项 | v0.2 的移交 `M12-polish.md`；发布说明的已知限制按主题写（4.13） |
| D 类 | 4.5 列出的几项随各 Step；其余进 v0.2 |
| M 类 14 项与读屏推断的九项 | 发布之后的人工验收（4.12）；33.1 先在 P2/S3 排查 |

**[M7-transfer](handoffs/M7-transfer.md)**

| 项 | 落实 |
|---|---|
| 1 结束的任务行（T1） | 部署文档与发布说明写明不清理，v0.2 定保留期 |
| 2 几个 GB 的导出 | P4/S3 批次 F，照 B8 判 |
| 3 部署文档 | P5/S3：点名 `exports/`、`imports/`；导出下载经代理的超时与缓冲；其余 README 已有（I1 第 7 节），挪进 `deploy/README.md` |
| 4 轮询；下载地址按客户端的时钟 | 轮询 P4/S2 只量；校时写进部署文档，P5/S3 |
| 5 按人的队列上限（T2） | 写明由管理员处理（管理员可以取消别人的任务），v0.2 加上限 |
| 5 几个 GB 的导入 | P4/S3 批次 F，照 B8 判 |
| 5 部署文档 | P5/S3（README 已有大半） |
| 5 名称不是 UTF-8（T3） | GBK 回退，P3/S3；CP437 进 v0.2 |
| 5 River 取到之后停掉（T4） | 已知限制（`README.md:193` 已写） |
| 6 总量配额（T5） | 写明由管理员监控磁盘，v0.2 |
| 6 按凭证的并发上传 | P4/S3 批次 E，只量；上限进 v0.2 |
| 6 附件很多时节点树的大小 | P4/S2 的页面树带附件的形状 |
| 6 从解析不到的附件链接上传（T8） | v0.2 |
| 6 部署文档 | P5/S3，含"停机时 `/mcp` 的调用在 20 秒处被切断"一句 |

**[M8-history](handoffs/M8-history.md)**

| 项 | 落实 |
|---|---|
| 1 撤销的次序比较（H1） | 已知限制，v0.2 |
| 2 版本的保留策略（H2） | P4/S3 批次 G 量增长；部署文档的容量，P5/S3；v0.2 设计保留策略 |
| 3 会话里按时间分版本（H3） | v0.2 |
| 4 永久删除（H4） | 界面的一句 P2/S2；部署文档与发布说明写明与变通 P5/S3；功能进 v0.2 |
| 5 日志的读（H5） | 4.6.2，P3/S1 |
| 6 回滚的变更集的撤销（H6） | 4.6.1，P3/S2 |

**总体设计第 14 节写着 M12 的各行**：搜索 → P4/S3 批次 B；渲染与提取的内存、解析预算先到先得、`FOR SHARE` 与 MultiXact → 批次 E；lint 与 `index.md` → 批次 C；同一笔记本的保存串行与改名到期限 → 批次 E 与 D；附件与导入导出的配额与负载 → 批次 F、E 与部署文档；页面树 → 4.7.2；版本的增长 → 批次 G；升级的冻结时长 → 批次 C；`NOTIFY` 的全局锁 → 部署文档。每一行在 P4/S3 或 P5/S3 的文档提交里照结论改写（"总体设计的修订"第 9 条）。

### 写给后面的 M（v0.1.1 与 v0.2）

- **v0.1.1**：发布之后的人工验收（4.12）发现的问题；读屏验收确认的九项。怎样组织在负责人批准开工时定，初稿的建议是照一个小的 M 做：00 号文档、Step、审查与提交说明的写法照 M12，目录 `docs/v0.1/v0.1.1/`（`docs/README.md` 的目录结构随之加一行）。
- **v0.2**：`docs/v0.2/handoffs/` 的五份（4.13）。v0.2 的总体设计开工时先处理它们。

### 总体设计的修订

定稿之后用脚本逐条替换，断言每段旧文字在文件里恰好出现一次（02-workflow 第 1 节第 8 步）。下面写出的每段旧文字都已这样核对过（`5e8af5b5`）。标"随某 Step 改"的，在那个 Step 的文档提交里改。

1. **12.2** 的 M12 一行，设计定稿时：`发布 v0.1 | 轻 |` → `发布 v0.1 | 中（五个 Phase、17 个 Step：地基（两个）、网页的打磨（六个）、服务端的正确性与历史（三个）、性能与内存实测（三个，另有条件的修复）、部署文档与发布流水线（三个）；不加接口操作与故事） |`。
2. **12.3**，设计定稿时：在 `- M11 依赖 M10（模式契约、守卫的框架、lint 的显式范围、模式变化的订阅者、来源的哈希）、M8（撤销的计划、后台任务与分批，用于回滚）、M9（MCP 的注册表、\`batch\`、多轮的调度）。` 之后加一行 `- M12 依赖 M0–M11 全部（发布的是它们）；改动前面 M 的地方逐项写在 [M12 总设计](M12-release/00-M12-design.md)第 8 节。`
3. **12.6**：设计定稿时 `| M12 | 发布 | 未开始 | — |` → `| M12 | 发布 | 进行中 | [00-M12-design.md](M12-release/00-M12-design.md) |`；收尾之后照 4.12 是"实现完成，待人工验收"。
4. **7.1**，随 P1/S2 改：`例如 \`postgres:18.6-trixie\`` → `例如 \`postgres:18.7-trixie\``。
5. **13.5 第 2 条**，随 P1/S2 改：`Node 本身按大版本（\`.node-version\`）。` → `Node 本身按大版本（\`.node-version\`）。Dockerfile 的 golang 镜像的版本等于两份 \`go.mod\` 的 \`toolchain\`（golang 镜像设了 \`GOTOOLCHAIN=local\`），测试、开发与 smoke 的 PostgreSQL 镜像在五处（含 README）相同，都由测试核对（M12）。`
6. **13.5** 加第 6 条，随 P5/S1、S2 改：`6. **版本号与发布**（M12）：版本号只由 ldflags 注入，Makefile、buildinfo 与 Dockerfile 的默认值相同，由测试核对；发布之后改为下一个补丁版本加 \`-dev\`。发布由推 \`v*\` 的 tag 触发 \`release.yml\`，闸门只放过 \`main\` 上 CI 已绿的提交；镜像两种架构，带签名、SBOM 与来源证明，不打 \`latest\`。推 tag 之前问负责人。`
7. **第 11 节**，随 P5/S3 改：
   - 第一段，从 `**M10/P3/S1升级补充**` 到 `不以暖样本承诺生产停机。`（各恰好出现一次）整段换成 `部署、备份、升级与回退的步骤在 [deploy/README.md](../../deploy/README.md)（M12）；这里只列要点。`
   - 最后一段，从 `- **M10/P3来源表示升级（决定62）**` 到 `§3.0、§8.1a）。`（各恰好出现一次）整段删掉；开发版的升级若要写，在 `deploy/README.md` 的附录。
   - `**单实例**：v0.1 官方只支持单实例（附件在本地磁盘）。` → `**单实例**：v0.1 只支持单实例：附件在本地磁盘，限流、大请求体的名额与正在上传的导入只在进程内计数（M12）。`
   - `不可写时拒绝启动（M7）。` → `不可写时拒绝启动（M7）。M12 起发布到 \`ghcr.io/open-nerve/nervewiki\`：amd64 与 arm64，标签是版本号与"大版本.小版本"，不打 \`latest\`；带签名、SBOM 与来源证明，校验的命令在 deploy/README。`
   - `恢复时先恢复数据库，再放回附件目录。` → `恢复时先恢复数据库，再放回附件目录；恢复之后先 \`ANALYZE\` 再开放服务，命令与演练见 deploy/README（M12）。`
8. **5.8 第 6 步与 13.1 第 2 条**，随 P3/S2 改：
   - `回滚自己的变更集在"先删的子树落在收着基线节点的新建节点下"时也答冲突（M11 第 302 条，M12 修）。` → `回滚自己的变更集照常可以撤销（M11 第 302 条的冲突由 M12/P3/S2 修好，[M12 总设计](M12-release/00-M12-design.md) 4.6.1）。`
   - 13.1 第 2 条里从 `回滚的变更集由几遍、几个单元在不同时刻写下` 起的那一句（到 `第 6 条）。` 为止）换成照 P3/S2 实情写的一句：撤销计划认出同一个变更集更早删下的回收站根、并进顶的组。
9. **第 14 节**，随 P4/S3、P5/S3 改：写着 M12 的十一行（搜索、渲染的内存、`FOR SHARE`、解析预算先到先得、lint 与 `index.md`、`NOTIFY`、保存串行与改名、附件与导入导出、页面树、版本的增长、升级的冻结时长）照实测的结论改写："M12 实测"与"M12 压测"改为量得的数、结论与已知限制。旧文字的锚点：`M12 压测`、`M12 实测；不达标时改为分层加载`、`v0.1 全部保留；M12 实测后在 v0.2 设计保留策略`、`配额与负载留给 M12（M7 写的移交）`、`10 万页与 30 万页由 M12 实测`，各恰好出现一次；其余几行在 P4/S3 的文档提交里照同样的办法先核对。
10. **13.1 第 28 条（标题键）与第 30 条（实时推送）**，随 P3/S1 改：标题键加切罗基文那一步；实时推送加每个账户的上限与重置。
11. **13.2 第 12 条**，随 P2/S1 改：`` `ConfirmDialog` 发送中仍用 `disabled`，留到 v0.1 之后的打磨（M12 的打磨移交（`handoffs/M5-polish.md`）第 23 条）。`` → `` `ConfirmDialog` 发送中用 `aria-disabled`，被拒之后焦点还给确认按钮或说明；只有横幅的拒绝时焦点给横幅（M12/P2/S1）。``
12. **13.4 第 3 条（端到端测试）**，随 P1/S1 改：加一句"故事经帮手换文档（`goto`、`reload`、`goBack`、`goForward`），帮手先等在途的续期与读答完，由门禁守住（M12/P1/S1）"。
13. **1.2** 的末句，收尾时：`以下几项明确留给 v0.2：同步 CLI` 起的那一句末尾，加"；M12 的已知限制与推后的事项在 v0.2 的移交（`docs/v0.2/handoffs/`）"。
14. **第 15 节**：设计定稿时加一行，写这些修订与依据；之后每次修订各加一行。
15. **00-M11 第 302 条**（不是总体设计，同一个提交里改）：`负责人若要放弃也输入名称，在 M12 改。` → `负责人若要放弃也输入名称，在 M12 改。（M12 更正：这里的"v0.1 之内不修"本意是"M11 之内不修"；照负责人 2026-11-13 第 1 条在 M12 修，[00-M12](../M12-release/00-M12-design.md) 4.6.1。）`
16. **`docs/README.md`**：设计定稿时，Git 一节 `- M11 同 M10（…）` 之后加 `- M12 同 M11（[M12 总设计](v0.1/M12-release/00-M12-design.md)第 7 节），另有发布的 \`(M12 release)\`。`；收尾时目录结构加 `v0.2/handoffs/` 一行。

**文档与代码不符之处**（M12 里改文档，不改代码，除非另写）：

- 5.8 第 6 步写"M12 修"、00-M11 第 302 条写"v0.1 之内不修"：第 15 条与 4.6.1。
- 13.2 第 12 条与打磨移交的"v0.1 之后"：4.5，第 11 条。
- 移交 M4-performance 第 14、15 条的守卫时限写 12 秒，代码是 14、14、8 秒（`bootstrap/trash_cost_test.go:539`、`revert_cost_test.go:623`、`:670`；M10 第 153 条）；第 16 条写 1 秒、2.1 秒，代码是 1.3 秒、3 秒（`bootstrap/background_cost_test.go:936-937`；M11 第 249 条）；第 15–18 条的份名是旧的（I2 第 8 节第 1–3 条）。P4 照代码量，移交收尾时注明。
- 移交 M8-history 第 5 条写"两条语句"，现在是五条读（I3 第 5 节第 4 条）。
- 移交 M5-polish 第 25 条写"分词的版本随之提升"：不需要（4.6.3）。
- 移交 M5-polish 已修或部分修了的：12.7 的一半、24.10 的注释、22.18 的大半、21.12 的一半、第 3 条的公式与图、`NativeSelect` 的宽度（I3 第 5 节第 5 条）。附录 A 只列剩下的。
- `Makefile:12` 的注释"默认值与 buildinfo 中的相同"：Dockerfile 另有两份，P5/S1 的守卫之后注释照实改。
- `README.md:629` 写 `postgres:18.6-trixie`，不在守卫里：P1/S2。
- `README.md:499`、`:532`、`:539` 与总体设计第 11 节的首尾两段写着"M12 实机复核"：照 P4 的结论改成已知限制（第 7 条）。

### 提交说明

照 M11（`docs/README.md:105`），加发布的一种：

- **代码**：`<范围>: <说明> (M12/P<n>/S<m>)`，范围是部件（`server`、`api`、`web`、`e2e`、`deploy`、`build`、`docs`，多个用逗号），例如 `e2e: wait for the reads in flight before a story's goto (M12/P1/S1)`。
- **文档**：直接进 `main` 的写 `docs: <说明> (M12/P<n>)`，M 级的写 `(M12)`，例如初稿 `docs: the M12 design's first draft, as its review reads it (M12)`；Step 的分支上的每个提交，文档的也是，写 `(M12/P<n>/S<m>)`。
- **审查的修复**：Step 的分支上、合并之前的写到 Step；合并之后的 Phase 的写到 Phase；修复的核对写第几轮，`(M12/P<n>/S<m> fix check <k>)`；收尾 `(M12 closeout)`。
- **合并**：一个 Step 一次，`Merge branch 'm12-p<n>-s<m>': <说明> (M12/P<n>/S<m>)`，正文经 `git merge --no-ff -F <文件>`。`Merge main into <分支>: <一句说明>` 不带后缀。
- **旁支**：分支 `m12-<slug>`，写它所属的 Step 或 Phase；修的是更早的 M 的故事时写 `(M<k> story, found in M12)`。
- **发布**：收尾合并之后、为发布做的提交写 `(M12 release)`，例如 `build: main's version after v0.1.0 is 0.1.1-dev (M12 release)`。
- **tag**：带注释的 tag，`git tag -a v0.1.0-rc.1 -m 'Nerve Wiki v0.1.0-rc.1'`；不签 GPG（镜像由 Sigstore 签）。
- 分支名 `m12-p<n>-s<m>`。每个提交以空行加 `Co-Authored-By:` 一行结尾。

## 8. 本 M 建立与注册的扩展点

M12 不建立新的扩展点，也不注册（12.4 里没有 M12）。

改前面 M 的代码（12.1 第 6 条的例外，逐项写明；都不改扩展点的签名）：

- **page（M8、M9）**：撤销计划的 `RevertGroups`、`groupIsDeleted`、`leftDeleted`（`page/domain/revert.go:196`、`:536`、`:494`）认出同一个变更集更早删下的回收站根（P3/S2）；日志的读放进快照，`HistoryReads` 加 `Snapshots`（P3/S1）；MCP `log` 的路径约定（P3/S1）；`searchFold` 的切罗基文映射删掉（P3/S1）。
- **shared（M4、M9）**：`TitleKey`、`TitleKeyContext` 的切罗基文一步；`Unshowable` 扩到默认可忽略的字符（P3/S1）。
- **linking（M6、M9）**：`IndexKeyer` 的切罗基文一步；`listTags` 不列 `/`；`index` 的空串约定（P3/S1）。
- **platform/markdown（M4、M6）**：`PlainText` 带上自动链接与原始 HTML 的文字（P3/S1）。
- **events（M5）**：hub 按账户数流、重置最老的那条；配置一节（P3/S1）。
- **transfer（M7）**：导入的条目名的 GBK 回退与报告的新原因（P3/S3）。
- **网页（M4–M11）**：附录 A 的各文件（P2）。
- **e2e（M0 起）**：夹具与 109 个故事文件（P1/S1）；打磨的断言（P2）；`e2e/perf/`（P4/S2）。
- **构建与 CI（M0）**：`ci.yml` 的触发与两个 action、`make vuln` 的一步、`make deploy-check`；新的 `release.yml`；Dockerfile 的摘要与 label；Makefile 的新目标（P1/S2、P5）。

## 9. 测试策略

### 9.1 打磨：每一类的测试写法

每一项先写一个在没修的代码上失败的测试，再修（13.4 第 1 条）。

- **焦点**：
  - jsdom 做得到的（焦点落在哪个元素、被拒之后回到哪里）用 vitest；断言 `document.activeElement` 是那个元素，不只是"不是 `body`"。
  - jsdom 做不到的（Chromium 在下一次渲染时才做 HTML 的焦点修正，Safari 不让点过的链接获得焦点，05-lessons 第 6 节）进 e2e，在三种引擎里探查；改到的故事照需要加进 `threeBrowsers`（`e2e/playwright.config.ts:3-4`）。
  - 一处改了焦点的规则会波及别处：对照空闲退出、未读的内容、portal 里的对话框逐一核对（05-lessons 第 6 节）。
  - 变异：`aria-disabled` 改回 `disabled`；去掉被拒之后的还焦点；去掉 `watchReader`（13.2 第 26 条）。
- **文字**：
  - vitest 在 `en` 与 `zh-CN` 下渲染，整句比对，不比片段。
  - 单复数每个键两格：1 与 2（或 0）。
  - 新的码与原因照旧由 `problem-messages.test.ts` 读契约核对（13.2 第 11 条）。
  - e2e 按确切的名字找按钮与对话框（05-lessons 第 6 节）。
- **超宽（WCAG 1.4.10）**：
  - jsdom 没有布局，只能进 e2e。
  - 视口 375 px，种下最长的名字（128 字符的客户端名、没有空格的长标题、从右到左的名字），断言 `document.documentElement.scrollWidth <= 375`，并断言那个元素自己没有溢出。
  - Chromium 与 WebKit 两种；33.1 的排查只在 webkit 上跑 SE1 的 375 一段。
- **`bdi`**：vitest 断言名字在 `<bdi>` 里。
- **对比度**：e2e 取计算之后的颜色，算对比度；文字 ≥ 4.5:1，焦点环与非文字的 ≥ 3:1；亮与暗两种主题。
- **只从 ARIA 推断的**：不写自动测试，进读屏的人工清单（4.12）。
- **阅读视图（P2/S4）**：更新之后，选区、展开的 callout、焦点、正在播放的媒体都还在；17.1 的整点替换由 vitest 的假定时器推进触发，不真的等一小时。

### 9.2 服务端

- **第 302 条**（P3/S2）：
  - page 的 domain 里一个性质测试：随机的树，随机的"先删一页（或一棵子树）、后删它的上级"的变更集，加上 M8/P3 的随机变更集的形状；撤销之后的树等于变更集之前的。期望由生成器的模型算出，不读被测的计划（05-lessons 第 1 节）。
  - 下限：数"嵌套的早删"真的出现的次数，不是规则触发的次数；换几组种子（05-lessons 第 1 节）。
  - 反例：别的变更集删下的回收站根不并进来；放回的次序。
  - 组合根：nested 一格翻过来；后台的撤销同样的形状一格。
- **H5**（P3/S1）：确定的交错（13.4 第 4 条）：持住撤销要的行，让日志的第一条读先读，再提交撤销，再让后几条读；断言读到的一致。去掉 `WithinSnapshot` 的变异让它失败。
- **标题键**（P3/S1）：`title_key_test.go` 的幂等样本加切罗基文；同一父页下只差切罗基文大小写的两页被拒；搜索的结果不变（删掉 `searchFold` 的映射前后比）。
- **流数**（P3/S1）：hub 的单元测试；一个整个程序的测试经 `/api/v0/events` 开 17 条，最老的那条收到 `reset`；把上限交成很大的变异让它失败。
- **GBK**（P3/S3）：测试里生成没有 UTF-8 标志、名字是 GBK 字节的 zip；合法的 UTF-8 名字不变；不是 GBK 也不是 UTF-8 的照旧跳过；差分：解读只改原本不是合法 UTF-8 的名字（13.4 第 11 条）。

### 9.3 e2e

- P1/S1 的帮手有一个对照测试（4.4）。
- 门禁有反向对照：在一个故事里写一个直接的 `page.goto`，门禁失败。
- M12 不加故事分组。打磨的断言加进那一页已有的故事；没有的，在它的分组里加一个故事（照 10.1，每个故事一个文件）。

### 9.4 实测

方法在 4.7.3。每一项的记录写：

- 机器（`nproc`、内存、当时的 `free`、`df`），PostgreSQL 与 Go 的版本，提交；
- 规模与形状、种子、命令；
- 两轮的数（p50、p99、峰值、时长），取哪一轮；
- 对照的预算与结论（过、超、无预算）；
- 超出的去处（修的 Step，或负责人的改判）。

### 9.5 发布流水线怎样在不推送的情况下验证

推送、签名、证明与 Release 只有真的 tag 才走得到（I1-D7）。之前分四层核对：

1. **静态**：actionlint 核对两份工作流（`make lint`）；闸门脚本 `deploy/release-gate.sh` 在本机对着假的输入跑：tag 名不是 semver、提交不在 `main` 上、提交的 CI 不绿，各被拒一次（反向对照）。
2. **分支上的演练**（P5/S1）：`release.yml` 在改了工作流或 `deploy/` 的分支推送上跑构建一半：两种原生 runner 各自 `push: false`、`load: true` 构建，对本地的镜像跑 `image-smoke.sh`。核实路径上下文带上了 `.git`、VCS 信息在。
3. **本机的 registry 演练**（P5/S2）：起一个本地的 `registry:2` 容器，照 `release.yml` 的步骤按摘要推送、`imagetools create` 合成索引、syft 生成 SBOM（关掉它的更新检查）、cosign 用临时的 key 签名并 `--tlog-upload=false`、`cosign verify --key`。不往外发任何东西。演练完删掉容器与 key。
4. **rc**：推 `v0.1.0-rc.1`，走完整条路，照 4.11 第 4 步核对。keyless 签名、`actions/attest`、GHCR 与 Release 只在这一层第一次真的跑。

### 9.6 变异（每个 Phase 设计列全，这里是要列进去的）

- P1/S1：去掉帮手的等待（对照测试抓）；帮手漏等 `refresh`；门禁放过直接的 `goto`。
- P1/S2：Dockerfile 的 Go 版本与 `toolchain` 不同（守卫抓）；README 的 PostgreSQL 版本不同（守卫抓）。
- P2：9.1 列的几种，每个 Step 照它的项补全。
- P3/S1：`TitleKey` 不做切罗基文一步；三处只改一处；`PlainText` 仍跳过自动链接；`listTags` 列 `/`；日志的读不在快照里；流数不数或重置最新的那条；`Unshowable` 不认默认可忽略的字符。
- P3/S2：去掉新规则；放宽到任何回收站根；放回的次序反过来；`RevertGroups` 不读里面的组。
- P3/S3：不看 UTF-8 标志就按 GBK；合法的 UTF-8 名字也被解读；报告不写。
- P5/S1：四份默认值有一份不同（守卫抓）；闸门的三条各去掉一条（本机的反向对照）。

### 9.7 门禁与 CI

- 每个 Step 照 02-workflow 第 3.1 节：`make lint gen-check knip build`、`make test`、`make test-go-short`、`make e2e`，都经 `heavy.sh`；每个提交单独为绿。
- `make image`、`make image-smoke` 不能在 worktree 里跑（06-environment 第 2 节）：P1/S2、P5 要它们时在一个普通的克隆里跑。
- 每个 Step 合并之后两次 `main` 的分份照第 7 节"分份"。

## 10. 风险

| 风险 | 应对 |
|---|---|
| **发布前的升级**：PostgreSQL 18.7 换了某条语句的计划；Go 1.27.4 的运行时改了竞态下的时间；新的 Debian 摘要带来别的变化 | P1 最先做，之后的每个 Step、计划测试与实测都在新的版本上跑；计划测试（13.4 第 9 条）在 18.7 上跑一遍，换了的在 P1/S2 里查清，不放宽；同一个 trixie，glibc 的大版本不变；`jsonv2` 的实验开关由 `mcpserver` 的测试核对（4.3） |
| **第五份与第四份的余量**：第五份两次在 EPYC 7763 上平均 425 秒，余量 25 秒；第四份此前在 7763 上 413.5 秒 | 新的整个程序的测试点名进第七、第八份（257、267 秒）；第四、第五份不加；P1/S2 合并之后两次 `main` 重看；过线先挪份，不提高 `-timeout` |
| **e2e 的偶发挡发布**：Playwright 不重试，`main` 的 CI 红了就不能打 tag | P1/S1 最先做；`ERR_NETWORK_CHANGED` 加一行日志看它在 CI 上触发了没有；其余的偶发照 M11 第 277 条的做法只重跑 e2e，并在记录里写明 |
| **推 tag 之后才走得到的路径**：keyless 签名、`actions/attest`、GHCR 的推送与权限、Release；`build-push-action` 的上下文 | 9.5 的四层；rc 先走一遍；组织的 Actions 权限由负责人在推 rc 之前核对（第 11 节） |
| **GHCR 设为公开不能撤回** | 只在 rc 核对之后做；之前核对镜像里没有机密：最后一层只有二进制（`deploy/Dockerfile:41-47`），构建参数只有版本与 label，`.git` 只在构建的阶段；Release 的说明与附件在 rc 上先看一遍 |
| **先发布、后验收**：v0.1.0 发出去之后，人工验收可能在输入法、真实的 MCP 客户端或读屏软件上找到问题 | 发布说明写明人工验收在发布之后、还没逐项核对过；v0.1.1 的路先想好（第 7 节"写给后面的 M"）；部署文档要求生产写确切的版本号，照升级的步骤换版本 |
| **打磨的面宽**：约 112 项，焦点的修复常波及别处（M5–M7 的修复核对一再找到回归，05-lessons 第 8 节） | 按组件分六个 Step；焦点的修复在三种引擎里探查；阅读视图先写 spec；修复的核对一轮一轮做到干净 |
| **第 302 条的修法改的是所有撤销都走的计划** | 先写 spec；只认一种形状（4.6.1 的三个条件）；性质测试与反例；M8 原有的撤销测试照过；收尾审查 A 专看撤销的其余路径 |
| **标题键与标题 id 改了已有的键** | 发布之前做；发布说明写明试用库要 reindex、这类锚点变了；不升分词的版本，免得白白重建搜索的行 |
| **事件流的上限在一个账户 17 条以上活的流时互相顶** | 网页一个浏览器一条流，人碰不到；P3/S1 核对重连的退避；记一行日志看得到 |
| **GBK 回退把别的编码误读成中文** | 条件三条都满足才回退；报告列出按 GBK 解读的名字，用户看得见；CP437 不做 |
| **实测的时长与机器**：约 10–16 小时，搜索一批约 20 GB 的磁盘；交换已满，噪声大 | 独占时段跑、经 heavy 的锁；之前修剪 Go 的构建缓存；两轮取较差的；本机为参照，部署文档写明 |
| **页面树 1 万页没过** | 4.7.2 先量它，结论先于 P2/S6；改法要设计，是一个大的 Step，M12 随之变长；负责人可以改判为已知限制 |
| **并行的 Step 争机器与翻译文件** | 同时实施的不多于两个；`en.ts`、`zh-CN.ts` 按序合并、先 `git merge main` |
| **`/tmp` 的用户配额与 heavy 的锁** | 4.2 的改动之后锁固定在 `/tmp`；大的输出放 `~/.cache` |
| **第三方许可证收不全** | P5/S2 先做个小实验；收不全的列在 Release 的说明里，并进 v0.2 的移交 |

## 11. 负责人确认的决定

### 2026-11-13：负责人批准开始 M12 与四个产品问题

原话："开 M12 吧，按标准流程推进"。之后在同一次对话里回答了四个产品问题（下面的问法由设计者照主持者的记录整理，主持者定稿时换成原问）：

1. **打磨的范围**：问"移交里的打磨事项（`M5-polish` 等）v0.1 修到什么程度？" 答"分级处理"（推荐项）：用户会碰到的错误行为、无障碍问题、误导的文字在 v0.1 修；纯体验的改进写成 v0.2 的清单，发布说明列为已知限制。
2. **性能的目标**：问"性能与内存实测之后，什么样的结果要在 v0.1 修？" 答"超出设计预算才修"（推荐项）：量到总体设计写的规模；只修超出设计已写预算的，例如页面树超标就改成分层加载；其余记成已知限制，进 v0.2。负责人 2026-11-06 另说过："不要把这次当做性能问题继续无限优化了，调整测试用例量级或者时限即可"。
3. **发布的形式**：问"v0.1 以什么形式发布？" 答"Release + GHCR 镜像"（推荐项）：打 v0.1.0 的 tag，写 GitHub Release 的说明；推 amd64 与 arm64 的镜像到 GHCR，带签名、SBOM 与来源证明；主持者把一切备好，推 tag 之前再问负责人一次。
4. **人工验收**：问"MCP 真实客户端的清单与读屏软件的验收，在发布之前还是之后？" 答"先发布、后验收"（没有选推荐的"验收通过才发布"）：先发 v0.1.0；MCP 真实客户端的清单 A–I、J、K 与读屏软件的验收在发布之后，发现的问题出 v0.1.1。

### 2026-11-14：开始 M12 之前的主持者的决定 M12-1–M12-10、M12-12–M12-19、M12-22（负责人可以改判）

主持者在说明里写明的几条，加上三份调查里每个决定的推荐选项（主持者暂取）。正式编号在审查之后接着 303 编。

- **M12-1 第 302 条在 M12 修**。"v0.1 之内不修"本意是"M11 之内不修"；照负责人第 1 条它是 A。修法取 I3 第 7 节第 3 条的推荐：改 M8 的撤销计划，另做性质测试（4.6.1）。
- **M12-2 页面树的阈值是 1 万页**（I2-D1）。量之前写死每一条界线（界线由设计者提，审查者核，4.7.2）；1 万页过了保留整棵树；5 万、10 万只量、记成已知限制，分层加载进 v0.2；1 万页没过，在 v0.1 改，那时再设计。
- **M12-3 发布的顺序**：收尾审查先做；两个 tag 一起问负责人（I1-D7）；推 rc、校验，再推 v0.1.0；收尾记录与 12.6 写"实现完成，待人工验收"，人工验收在发布之后（4.11）。
- **M12-4 负责人自己在 GitHub 上做的设置**列在本节"需要负责人确认"，带"什么时候要"（I1-D19）。
- **M12-5 漏洞扫描**：`govulncheck` 在 CI 的 runner 上跑，npm 一侧靠 GitHub 的依赖图告警；本机不跑这两样（I1-D10，4.3）。
- **M12-6 heavy.sh 的锁**固定在 `/tmp/nervewiki-heavy.lock`，仍可由 `NW_HEAVY_LOCK` 改；06-environment 写 `/tmp` 的配额与"改 `TMPDIR` 不再改变锁"；P1 之前的一个提交（4.2）。
- **M12-7 工具链与依赖只做与安全相关的更新**（I1-D9、D10、D12）：Go 1.27.4、PostgreSQL 18.7、三个镜像的摘要、两个 action、`govulncheck` 报出的模块；Node 26 与功能性的大版本不升；加 `make outdated`（I1-D11，4.3）。
- **M12-8 e2e 的 `goto` 帮手**（I3 第 7 节第 14 条）：等在途的续期与读，逐个故事换上，门禁守住；M1 第 10 节的取舍不动（4.4）。
- **M12-9 打磨的分级**（I3 第 7 节第 1、2、6、7、12、13、15 条）：照 I3 第 9 节的分类；读屏推断的九项移到发布之后；H1、24.1、T4 写成已知限制；第 48 条算 A；单复数用 `*One` 键；退出登录问"仍要退出吗"；第 40 条两份模板都改；M 类并进发布之后的人工验收，33.1 先排查（4.5）。
- **M12-10 标题键与标题 id 在 v0.1.0 之前改**（I3 第 7 节第 4、5 条）：三处折叠一起改，删掉 `searchFold` 的映射，不升分词的版本（4.6.3）。
- **M12-12 每个账户至多 16 条事件流，超出重置最老的一条**（I2-D9，4.6.5）。
- **M12-13 导入的 GBK 回退**，CP437 进 v0.2（I3 第 7 节第 8 条，4.6.6）。
- **M12-14 实测的范围与判据**（负责人第 2 条；I2-D3、D6、D7、D8、D10、D11、D12、D13）：只有 4.7.1 的预算判"超出"；搜索的尾巴与体积照 M8 已接受的；`DeleteNode` 与单个 `move` 只在正常形状到期限时修；按页的日志、lint 的 TotalAlloc、MCP 名额的配置项、慢的发送者与 Caddy 早答都只记、进 v0.2；附件列表的重读能稳定让保存失败才修；`apitest.Load` 不主动做（4.7）。
- **M12-15 "目标硬件"**以本机为参照、EPYC 7763 为慢的主机的代表（I2-D2，4.7.3）。
- **M12-16 测量的设施进仓库、不进 CI**；内存从外部采样，不加 pprof 或 metrics 端点（I2-D4、D5、D14，4.7.4）。
- **M12-17 部署文档**：`deploy/README.md` 作运维手册，加 `deploy/compose.yaml` 与它的检查；根 README 缩成入口；开发版的升级说明看负责人的回答；单实例写明原因、时钟一条不适用；Caddy 的 502 只写代价、推荐 nginx；小项都写进去（I1-D13、D15–D18，4.8）。
- **M12-18 版本号**：tag 经 ldflags 注入，四份默认值有守卫，发布之后 `main` 是 `0.1.1-dev`；不打 `server/v0.1.0`，不宣传 `go install`（I1-D8，4.9）。
- **M12-19 发布流水线**：`ghcr.io/open-nerve/nervewiki`，标签 `0.1.0` 与 `0.1`、不打 `latest`；tag 触发、闸门三条加 `make vuln`、`ci.yml` 只在分支上跑；两种原生 runner；`actions/attest` 的来源证明与 SBOM 加 cosign keyless；前端另出一份 SBOM；镜像与 Release 带第三方许可证；Release 不附二进制、`v0.1.0` 不标 pre-release；先推 rc（I1-D1–D7、D14，4.10）。
- **M12-22 T1、T2、H4 写明、功能进 v0.2**（I3 第 7 节第 9、10、11 条）：结束的任务行不清理；按人的队列上限由管理员处理；永久删除写明与变通，界面写一句（4.5、4.8）。

### 2026-11-14：00-M12 的设计者的取舍 M12-11、M12-20、M12-21、M12-23–M12-28（主持者的决定，负责人可以改判）

- **M12-11 MCP 的可选参数给空串或 `"/"` 一律答字段错误**：省略即不筛，文字写"省略它就是整本"。理由：`index` 已经这样做并写在工具说明里（`linking/adapter/mcp/tools.go:62`），`log` 跟它一样；"path is required"是误导的文字（30.4，B）。细节在 P3/S1 的设计里（4.6.4）。
- **M12-20 发布说明是仓库根的 `CHANGELOG.md`，中文**。理由：`docs/` 按约定只放设计与过程文档；README 是中文；Release 的说明取它的一节，工作流在后面接摘要与校验命令（摘要在推送之前不知道）。另写 `SECURITY.md`（4.10）。
- **M12-21 第 2 步的点头盖住 rc 的重推**：只改发布流水线、部署文档或发布说明的 rc.N，同一次点头盖住；改了产品代码的再问。GHCR 设为公开在 rc 核对之后、`v0.1.0` 之前（4.11）。理由：rc 的用处就是找出流水线的问题，每次重推都问会让负责人的一次点头变成几次；产品代码的改动改变了被发布的东西，要再问。
- **M12-23 v0.2 的移交放在 `docs/v0.2/handoffs/`，五份按主题**；v0.1.1 照一个小的 M 做，目录 `docs/v0.1/v0.1.1/`，在负责人批准开工时定（4.13，第 7 节"写给后面的 M"）。理由：文档约定把移交放进目标的 `handoffs/`，v0.2 还没有 M 的划分；按主题分，v0.2 的设计者按主题读。
- **M12-24 人工验收的入口**：`docs/v0.1/M12-release/manual/README.md` 汇总，链到输入法清单、MCP 的清单与新写的读屏清单（4.12）。理由：先发布、后验收，负责人要一个地方看全部欠着的验收。
- **M12-25 新的整个程序的测试点名进第七、第八份**；第四、第五份不加（第 7 节"分份"）。理由：第五份余量 25 秒，第四份是补集、名字一不留神就落进去。
- **M12-26 页面树的三条界线**：T1 冷打开 ≤ 1.5 秒；T2 别人改一次树，新的一行 ≤ 1.5 秒出现、最长的长任务 ≤ 200 ms；T3 展开 1,000 个孩子 ≤ 500 ms；三种形状；两轮的较大中位数（4.7.2）。理由：I2 推荐的"第一次画完 ≤ 1 秒"没算网络与 2.7 MiB 的传输，本机走回环、生产走网络，1.5 秒给网络留了余地；200 ms 是用户感到卡顿的量级；T3 盖住"平的大文件夹"那一种渲染。审查者重点核这三个数。
- **M12-27 P4 分成设施、浏览器、服务端三个 Step，页面树先量**（第 7 节）。理由：页面树的结论挡 P2/S6；服务端的实测要等 P3 改完撤销、日志与标题键；设施是代码、实测是文档，分开审。
- **M12-28 网页上显示 MCP 地址进 v0.2**。负责人 M9 的决定是"显示 MCP 地址的网页界面等打磨"（`04-owner-decisions-and-preferences.md` 第 5 节）。打磨分级之后它是 C（31.1、47），照负责人第 1 条进 v0.2，发布说明列为已知限制。负责人若要它在 v0.1，是 P2 的一个小 Step。

### 对主持者说明的核实

- **第 302 条的修法**：读 `page/domain/revert.go:494-548` 与 `page/app/rollback.go:70-125`，形状与 I3、移交的描述一致；改法在计划里做得到。没有造过、没有跑：P3/S2 的 spec 先造这个形状，并造后台恢复与撤销里的同一形状。
- **heavy.sh 的锁**：`docs/v0.1/claude_to_codex/tools/heavy.sh:8` 是 `L=${NW_HEAVY_LOCK:-${TMPDIR:-/tmp}/nervewiki-heavy.lock}`，与说明一致。
- **第四份 413.5 秒**：这是收尾之前那两次 `main`（`07abd0f7`、`9e787bab`，都在 7763 上）的平均。收尾之后的两次第四份是 339、285 秒（平均 312，都不在 7763 上）；第五份 427、423 秒（平均 425，都在 7763 上）（[M11 收尾审查](../M11-upgrade/reviews/M11-closeout-review.md)"门禁"一节的表）。两条都写进第 10 节。
- **`TestRevertingARollback…` 在第八份**：`Makefile:296`、`:328` 都点了它的名。
- **Playwright 不重试**：`e2e/playwright.config.ts` 没有 `retries`。
- **调查之外另查到的**：
  - `platform/httpserver/mcpserver/edge.go:106` 写着 go1.27.1 里 `encoding/json/v2` 在 GOEXPERIMENT 之后，升工具链时要核对（4.3）。
  - `server/tools/go.mod` 的注释把 kin-openapi 等钉在 oapi-codegen 的要求上，所以 `govulncheck` 不进那里，用 `go run …@版本`（4.3）。
  - 事件流已有 `reset` 帧，流数的上限沿用它（4.6.5；13.1 第 30 条）。
- 其余（三份调查的事实与推荐）照做，没有发现做不到的地方。抽查的行号见附录 B。

### 需要负责人确认

没有挡住开工的问题。下面几项各写"什么时候要"：

| # | 事项 | 推荐 | 什么时候要 |
|---|---|---|---|
| 1 | 推 `v0.1.0-rc.1` 与 `v0.1.0` 两个 tag（4.11 第 2 步） | 照 4.11 | 收尾合并、`main` 的 CI 绿之后，推 rc 之前 |
| 2 | 组织允许公开的容器包；组织的 Actions 设置允许工作流申请 `packages: write`、`id-token: write`、`attestations: write`（I1-D19） | 照做 | 推 rc 之前（P5/S2 合并之后就可以） |
| 3 | 给 `v*` 的 tag 加规则集，只有负责人与主持者能推（I1-D19） | 照做 | 推 rc 之前 |
| 4 | 开 Dependabot alerts（npm 一侧的漏洞告警只靠它）与私密的漏洞报告（`SECURITY.md` 指向它）（I1-D19） | 照做 | alerts 越早越好（P1/S2 之后）；漏洞报告在 P5/S3 写 `SECURITY.md` 之前 |
| 5 | 把 GHCR 的包设为公开，**不能撤回**（I1-D19） | rc 核对之后 | 4.11 第 5 步：rc 核对之后、推 `v0.1.0` 之前 |
| 6 | Dependabot 的版本更新 PR 要不要开（移交 M0-P6 第 4 条写明由负责人定） | 不开，只开 alerts，发布前跑 `make outdated`（4.3） | M12 收尾之前（写 v0.2 的移交时） |
| 7 | 有没有 v0.1.0 之前就在跑、要升级到 v0.1.0 的实例（I1-D13） | — | P4/S3 开始之前：没有就不写开发版的附录、不量批次 H |
| 8 | 输入法清单（M4–M8 的 21 步）是否同样在发布之后验收（4.12） | 是：同 2026-10-13 的原话 | 写发布说明之前（P5/S3） |

改了负责人原话或负责人可能想自己定的：M12-12（流数的上限与重置）、M12-21（rc 的重推不再问）、M12-28（MCP 地址进 v0.2），在 M12 的汇报里列出。

## 12. Phase 进度表

| P | Phase 文档 | 状态 | 审查 | 合并 |
|---|---|---|---|---|
| P1 地基（S1、S2） | — | 未开始 | — | — |
| P2 网页的打磨（S1–S6） | — | 未开始 | — | — |
| P3 服务端的正确性与历史（S1–S3） | — | 未开始 | — | — |
| P4 性能与内存实测（S1–S3，条件的 S4 起） | — | 未开始 | — | — |
| P5 部署文档与发布流水线（S1–S3） | — | 未开始 | — | — |
| 收尾与发布 | — | 未开始 | — | — |

## 13. 变更记录

| 日期 | 变更 | 依据 |
|---|---|---|
| 2026-11-14 | 初稿：五个 Phase、17 个 Step，另有条件的修复；不加接口操作与故事；页面树的阈值与三条界线；第 302 条在 M12 修；发布的顺序与对外的步骤；主持者的决定 M12-1–M12-10、M12-12–M12-19、M12-22，设计者的取舍 M12-11、M12-20、M12-21、M12-23–M12-28 | 负责人 2026-11-13 的批准与四个回答；三份只读调查（基点 `5e8af5b5`）；六份 `open` 的移交；主持者的说明 |

---

## 附录 A：打磨的逐项去处

编号是 [M5-polish](handoffs/M5-polish.md) 的条目与 I3 拆出的子项；分类照 I3 第 9 节，例外在 4.5。

### 进 v0.1 的（A 31 项、第 48 条、B 80 项）

| Step | A | B | 一起做的 C、D |
|---|---|---|---|
| P2/S1 焦点与共用组件 | 7 | 6.3、9.1、11.7、12.3、16.1、18.6、19.3、21.14、22.17、23.14、23.16、24.3、26.4、28.4、28.5、35.2、38.1、38.3 | C：23.17、28.2（随 23.16） |
| P2/S2 文字 | — | 6.1、9.5、15.3、16.6、19.1、21.8、21.11、22.11、22.12、22.13、22.16、23.1、23.3、23.4、23.11、23.12、23.13、24.7、26.11、26.13、26.14、35.1、38.6、38.8、40、41.1、41.2、41.3 | H4 的界面一句（移交 M8-history 第 4 条） |
| P2/S3 375 px、`bdi` 与对比度 | — | 12.4、26.9、26.12、26.17、32.1、32.2、32.4、35.8、38.10、38.11、38.12 | D：32.3；M：33.1 的排查 |
| P2/S4 阅读视图 | 6.2、11.1、17.1、17.8、48（C 改判为 A） | 3、8.1、8.2、12.2、29 | C：11.6 的失焦部分；D：14d |
| P2/S5 编辑器、附件、上传、粘贴与导入导出 | 12.8、15.1、15.2、16.3、16.4b、16.11、17.2b、17.4、17.6、17.7、18.1、18.2、19.4、20.3、38.7 | 16.4c、16.10、20.1、20.2、28.1、38.9 | — |
| P2/S6 版本、回收站、历史、目录、后台任务与搜索 | 2、12.12、21.7、22.14、23.10、23.19、24.2、35.3 | 21.2、21.3、21.10、22.2、22.19、24.9、26.2 的前一半、26.5、26.18a、27.2 | D：24.8、24.10、26.15 |
| P3/S1 小的正确性 | 10.1、10.2、25.1（与 25.2） | 30.2、30.4 | C：30.5（与 30.4 定一种约定） |

### 发布之后的人工验收（4.12）

- **M 类**（14 项）：5、8.4、11.8、12.11、13.3、16.12、17.9、18.5、18.8、26.18b、27.3、33.1（P2/S3 先排查）、33.2、39。
- **只从 ARIA 推断的 B**（9 项）：16.8、18.4a、21.15、21.16、22.10、26.1、26.2 的后一半、28.3、49。确认有问题在 v0.1.1 修。

### 已知限制，进 v0.2

- **罕见又大的 A**：24.1（与历史的 H1、导入的 T4）。
- **C 类**（v0.2 的 `M12-polish.md`）：
  - 第 1–12 条：1、3 的高亮闪动、4、6.4、8.3、9.2、9.3、9.4、10.3、11.3、11.4、11.5、11.6 的其余、11.9、12.1、12.2 的开合、12.5、12.6、12.7、12.9、12.10；
  - 第 13–20 条：13.1、13.2、16.2、16.4a、16.5、16.7、16.9、17.2a、17.3、18.3、18.4b、18.4c、18.7、19.2、19.5、19.6、20.4；
  - 第 21–24 条：21.1、21.4、21.5、21.6、21.9、21.12、21.13、22.1、22.3、22.4、22.5、22.6、22.7、22.8、22.9、23.2、23.5、23.6、23.7、23.9、23.15、23.18、24.4、24.5、24.6；
  - 第 25–33 条：26.3、26.6、26.7、26.8、26.10、27.1、30.1、31.1、31.2；
  - 第 34–51 条：34、35.4、35.5、35.6、35.7、36、37、38.2、38.4、38.5、38.13、42、43、44、45、46、47。
- **D 类的其余**：11.2、14a、14b、14c、17.5、22.15、22.18、23.8、26.16、30.3。

I3 标"给主持者"、可升可降的条目（例如 2、6.2、21.4、21.6、22.8、23.15、26.5、35.7、38.2）照 I3 第 9 节的分类不改；P2 的 Step 设计可以照实情提出改判，记成主持者的决定。

## 附录 B：核对过的代码

设计者对着 `main` 的 `5e8af5b5` 核对过下面这些（其余照三份调查，调查各自核对过）：

- `docs/v0.1/claude_to_codex/tools/heavy.sh:8`（锁的默认）。
- `Makefile:12-14`（`VERSION`、ldflags）、`:85-90`（`IMAGE`、`make image` 与 worktree 的检查）、`:263`（第五份）、`:296`、`:328`（第七、第八份点名 `TestRevertingARollback…`）。
- `deploy/Dockerfile:7`、`:9`、`:23`、`:29`、`:41`、`:42`、`:43-46`（三个镜像的摘要、corepack、两个 `ARG`、四个 label，`licenses` 是 `AGPL-3.0-only`）。
- `.github/workflows/ci.yml:3-5`（`on: push` 不带过滤）、`:13-14`（`contents: read`）、`:148-150`（`setup-go` 读 `go.mod`）、`:184`、`:301`（`upload-artifact` v7.0.1）、`:229-233`、`:276-280`（`setup-node` v7.0.0 与 corepack）、`:263`（e2e 的 20 分钟）、`:310-326`（`image` 一份）。
- `e2e/playwright.config.ts:3-4`（`threeBrowsers`），整个文件没有 `retries`；`e2e/stories` 里 109 个文件、259 处 `goto(`。
- `events/app/hub.go:20-35`、`:95-112`（`open` 不数账户）；`web/…/events/handlers.ts:36`（`TREE_INTERVAL_MS = 500`）。
- `page/app/rollback.go:70-125`（`rollbackUnit`、`PlanRollback`、先删的森林）；`page/domain/revert.go:494-548`（`leftDeleted`、`groupIsDeleted`）；`bootstrap/upgrade_rollback_revert_test.go:30`、`:87`。
- `shared/title_key.go:1-28`；`page/domain/search.go:240-256`（`searchFold`）。
- `platform/postgres/check.go:53-66`（建库参数的自检）；`bootstrap/mcp.go:38-42`（大请求体的两个名额、等 2 秒）；`bootstrap/search_bench_test.go:66-69`（没有语料就跳过）。
- `platform/httpserver/mcpserver/edge.go:7`、`:106`（`encoding/json/v2` 与 go1.27.1 的说明）。
- `bootstrap/trash_cost_test.go:539`、`revert_cost_test.go:623`、`:670`（14、14、8 秒）；`bootstrap/background_cost_test.go:936-937`（1.3 秒、3 秒）。
- `server/tools/go.mod`（工具模块与它的注释）。
- `README.md:9`、`:16`、`:181`、`:193`、`:497`、`:522`、`:526`、`:629`。
- `docs/README.md:105`；`docs/v0.1/M11-upgrade/00-M11-design.md:949`；`docs/v0.1/M11-upgrade/reviews/M11-closeout-review.md` 的分份表（第 183–190 行）。
- `v0.1-design.md` 第 7 节"总体设计的修订"引的每段旧文字，各恰好出现一次。
