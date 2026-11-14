# M12 发布：总设计

| 项 | 内容 |
|---|---|
| 里程碑 | M12 发布（`M12-release`） |
| 日期 | 2026-11-14（初稿）；2026-11-15（第一次、第二次修订；定稿） |
| 状态 | 进行中（P1 设计已审查，S1 开工；[审查记录](reviews/M12-design-review.md)；总体设计照第 7 节"总体设计的修订"中标"设计定稿时"的几条改过） |
| 依赖 | M0–M11 全部。直接改动或量的：M0/P6 的镜像与 image-smoke；M1 的续期（e2e 的 `goto`）；M4–M11 的成本守卫与夹具；M5 的事件流（每个凭证与每个账户的流数）；M6 的解析与标签（标题 id、`#//`）；M7 的导入与任务（名称的编码、按人的队列上限、启动时的收拾）；M8 的撤销计划（第 302 条）与日志的读；M9 的 `log`、后台撤销的分段与 MCP 超时的文字；M10 的 `AGENTS` 模板；M11 的回滚 |
| 上级文档 | [v0.1 总体设计](../v0.1-design.md) 第 11、12.2、12.5、12.6、13、14、15 节；[文档约定](../../README.md)；[推进方式](../claude_to_codex/02-workflow.md) |
| 依据 | 负责人 2026-11-13 批准开工与四个回答、2026-11-14 的两个回答（第 11 节）；设计审查 A、B、C 与两轮修订核对（[审查记录](reviews/M12-design-review.md)）；三份只读调查 I1（发布、部署与依赖）、I2（性能与内存）、I3（打磨与历史的分级），基点 `5e8af5b5`；六份 `open` 的移交 |
| 代码位置的写法 | 同 [M11 总设计](../M11-upgrade/00-M11-design.md)：模块写作 `page/app/rollback.go:77`（`server/internal/modules/` 之下）；`platform/…`、`bootstrap/…`、`shared/…` 在 `server/internal/` 之下；前端写作 `web/…/app/confirm-dialog.tsx:177`（`web/apps/web/src/` 之下）；e2e 写作 `e2e/playwright.config.ts:3`；其余写仓库根起的路径。行号都是 `main` 的 `5e8af5b5` 的 |

---

## 1. 目标

M12 把 v0.1 发布出去。总体设计 12.2 的一行是："部署与备份文档、性能与内存实测（包括大笔记本下的页面树与搜索）、版本号与发布流程、发布 v0.1"。六份移交又加了打磨与历史的事项。负责人 2026-11-13 的四个回答与 2026-11-14 的两个回答定了做到什么程度（第 11 节）。

1. **地基**：发布之前把工具链与数据库升到带安全修复的版本（Go 1.27.4、PostgreSQL 18.7、三个基础镜像的摘要、两个 action）；让 e2e 的 `goto` 与 `reload` 不再打断续期，`main` 的 CI 因此稳下来（4.3、4.4）。
2. **打磨，分级处理**（负责人第 1 问）：用户会碰到的错误行为、无障碍问题、误导的文字在 v0.1 修；纯体验的改进写进 v0.2 的移交，发布说明列为已知限制。罕见的 A 类与只从 ARIA 推断的几项照负责人第 5、6 问处理（4.5）。
3. **历史与服务端的正确性**：修好 M11 第 302 条的回滚撤销冲突；日志的读放进一个快照；标题键与标题 id 的两处改动赶在 v0.1.0 之前，那时还没有生产的库；导入导出任务的三处：名称的编码、按人的队列上限、启动时的收拾（4.6）。
4. **实测，超出设计预算才修**（负责人第 2 问）：量到设计写的规模；只修超出已写预算的；页面树的界线在量之前写死；每一行写明在哪台机器上判；其余记成已知限制，进 v0.2（4.7）。
5. **部署与备份文档**：一份照着就能做的运维手册 `deploy/README.md`，带 `deploy/compose.yaml` 样例；安装、升级、备份与恢复有命令、演练过（4.8）。
6. **版本号与发布流水线**：tag 触发的 `release.yml`，构建 amd64 与 arm64 两种镜像，推到 GHCR，带签名、SBOM 与来源证明，建 GitHub Release（负责人第 3 问；4.9、4.10）。
7. **发布**：收尾审查之后问负责人，推 `v0.1.0-rc.1`、校验，再推 `v0.1.0`；人工验收在发布之后，发现的问题出 v0.1.1（负责人第 4 问；4.11、4.12）。

## 2. 范围

**做**：

- **工具链与依赖**（只做与安全相关的，4.3）：
  - Go 的 `toolchain` 到 `go1.27.4`，golang 镜像随之；文档里写着 1.27.1 的五处随之；
  - PostgreSQL 到 `18.7`；
  - node、golang、distroless 三个基础镜像的摘要；
  - `actions/setup-node` 到 v7.1.0，`actions/upload-artifact` 到 v7.0.2；
  - `make vuln` 报出的 Go 模块与 npm 包；
  - 两个守卫：Dockerfile 的 Go 版本（tag 与构建时的断言）等于 `toolchain`；README 的 PostgreSQL 镜像并进已有的一致性守卫；
  - `make vuln`（Go 一侧只扫 `server/`，加 npm 一侧的 `pnpm audit --prod`）与 `make outdated`：两样都只在 runner 上跑；`make vuln` 在一个工作流 `vuln.yml` 里（每天、手动、改了依赖清单的推送），发布的闸门也跑它；npm 一侧挡发布的是 `high` 级。
- **e2e**：`goto` 与 `reload` 之前等在途的续期与读答完的帮手，故事与夹具逐个换上；一条门禁不许直接调这两个方法，豁免只经显式的逃生口、门禁数它；`global-setup.ts` 的网络等待打一行日志（移交 M5-polish 第 50、51 条，4.4）。
- **网页的打磨**：附录 A 列出的 A、B 两类，按组件分六个 Step（4.5）。
- **服务端的正确性与历史**（4.6）：
  - 第 302 条：被这个变更集删了、现在本身是回收站根的节点，在 M8 的撤销计划里是它自己的顶；后台撤销的分段随之；
  - 日志与一个变更集的详情各自的五条读放进一个快照（H5）；
  - 标题键对切罗基文幂等（25.1），标题 id 带上自动链接与原始 HTML 的文字（10.2）；
  - 标签列表不列 `/`（10.1）；只由看不见的字符组成的显示名与客户端名（30.2）；`log` 的 `path` 收 `""` 与 `"/"` 的约定（30.4、30.5）；MCP 调用超时的文字（移交 M4-performance 第 22 条之后的 batch 补记）；
  - 每个凭证与每个账户的事件流数的上限（移交 M5-performance 第 6 条）；
  - 导入导出的任务：名称不是 UTF-8 的条目先读 Info-ZIP 的 Unicode 路径、再按 GBK 回退（T3）；按人的队列上限（T2）；启动时收拾上一个进程取到而没开始的任务（T4）。
- **实测**（4.7）：
  - 进仓库的设施：合成的大笔记本与搜索语料、有预算的几项要的并发驱动、内存采样、浏览器一侧的计时脚本、现有守卫的规模开关；
  - 量：4.7.1 表里有预算的每一行与部署文档要的数；其余能用已有的数推的就推，写明"推算"；
  - 超出预算的修，带缩小规模的守卫。
- **部署文档**（4.8）：`deploy/README.md`（含硬件的下限）、`deploy/compose.yaml` 与它的检查；根 README 的"部署"一节缩成入口；从开发版升级的附录（看负责人的回答，第 11 节）。
- **版本号与发布**（4.9、4.10）：
  - 四份默认值的一致性守卫；MCP 的版本的短测试；镜像补三个 OCI label；
  - `ci.yml` 不在 tag 上跑（`ci-wait.sh` 按工作流取，已在 `b54cd679`）；
  - `release.yml`：闸门、两种架构的构建与 smoke、推送、合成索引、来源证明、SBOM、签名、Release、发布之后的核对；只读的演练作业；
  - 第三方许可证随镜像与 Release 发出；
  - `CHANGELOG.md`（发布说明与已知限制）、`SECURITY.md`。
- **人工验收的入口**（4.12）：读屏软件的清单；一份汇总的验收入口，链到输入法清单、MCP 的清单 A–K 与 M 类各项。
- **以后的移交**：C 类与推后的事项写进 `docs/v0.2/handoffs/`；发布之后的人工验收写进 `docs/v0.1.1/handoffs/`；都在收尾时写（4.13）。

**不做**：

- C 类 87 项与 3、12.2、11.6 的其余（附录 A）、H3（会话里按时间分版本）、H4 的功能（永久删除）、T1（结束的任务行的保留期）、T5（总量配额）、T8（从解析不到的附件链接上传）：进 v0.2（4.5、4.13）。
- H1（撤销按次序值比较）、24.1（结果未知的查找）：罕见，改动大，H1 还要迁移。照负责人 2026-11-14 第 5 问写成已知限制，进 v0.2（4.5）。
- 只从 ARIA 推断"念两遍、念得急"的八项：照负责人 2026-11-14 第 6 问并进发布之后的读屏验收，确认有问题的在 v0.1.1 修（4.5）。
- 页面树的分层加载：只在 1 万页过不了 4.7.2 的界线时才做（4.7.2）。
- 搜索 300 ms 尾巴的改法、按页日志的迁移、共用的参与者界线、MCP 名额的配置项、慢的发送者与 Caddy 早答的服务端改法、`batch` 单独的期限：除非实测超出 4.7.1 的判据，进 v0.2（I2-D3、D6、D7、D12、D13；4.6.4）。
- 导入时选编码的选项；CP437；Shift-JIS、Big5、EUC-KR 的识别（4.6.6）。
- Node 26、功能性的大版本依赖（River 0.51、katex 0.19、`@lezer/markdown` 1.8、mermaid 12、pgx、goose、`x/text`）、Dependabot 的版本更新 PR（4.3）。
- 在本机跑 `govulncheck`、`pnpm audit` 与 `make outdated`（4.3）。
- `latest` 与 `0.1` 这类浮动的标签、独立的二进制附件、宣传 `go install`（4.9、4.10）。
- 本机起一个 registry 演练推送与签名（9.5）。
- 多实例（4.8）；pprof 或 metrics 端点（I2-D14）；新的配置项（5.2）。
- 网页上显示 MCP 地址：负责人 M9 的决定是"等打磨"，打磨分级之后它是 C（31.1、47），进 v0.2。
- 新的接口操作、MCP 工具与 e2e 故事分组。

## 3. 完成标准

总体设计 12.5 的通用标准之外，每条写出由什么证明。

1. **工具链**（P1/S2）：
   - `server/go.mod`、`server/tools/go.mod` 的 `toolchain`，`deploy/Dockerfile` 的 golang 镜像的 tag，与 server 阶段构建时断言的 `go env GOVERSION`，同是 1.27.4。tag 与断言的一行由一个短测试核对；摘要不是 1.27.4 时构建失败。改其中一处，测试或构建失败（反向对照，记在 S2 的记录）。
   - PostgreSQL 镜像在五处（`pgtest/pgtest.go:35`、`deploy/compose.dev.yaml:8`、`e2e/fixtures/db.ts:13`、`deploy/image-smoke.sh:86`、`README.md:629`）同是 `postgres:18.7-trixie`，由 `pgtest/image_test.go` 核对（今天只核对前四处，`pgtest/image_test.go:12-39`）。P5/S3 之后 README 那一处换成 `deploy/README.md`，另加 `deploy/compose.yaml` 的镜像与 initdb 参数。
   - `make vuln`（Go 一侧与 npm 一侧）在 S2 的分支上由 `vuln.yml` 的 `push` 触发跑一次为绿；证据是 S2 记录里的 run 号。之后的运行红了，照 4.3 开旁支或写进 `audit.ignore`；发布的闸门再跑一次（第 9 条）。
2. **e2e 的帮手**（P1/S1）：
   - `e2e/` 里（故事、夹具与以后的 `e2e/perf/`）不再直接调 `page.goto` 与 `page.reload`，由一条门禁守住，有反向对照。豁免只经帮手显式的逃生口；门禁按它的表核对每个文件的逃生口的个数，理由写在表里。帮手所在的那一个模块是唯一直接调用它们的地方，门禁对它例外，不算进逃生口。
   - 帮手有一个对照测试：把续期的答复延后，经逃生口不等时页面落到 `/sign-in`，经帮手时不落（05-lessons 第 6 节："要复现这种竞态，把答复延后，不要靠重复跑"）。
   - P1/S1 合并之后到 M12 收尾，`main` 的 e2e 没有一次因为被跳回登录页而红；收尾记录列出红过的每一次（run 号与原因）与这期间 `main` 的 e2e 的总次数（主持者的决定 334）。
3. **打磨**（P2，服务端的几项在 P3/S1）：
   - 附录 A 的每一项 A、B 各有一个在没修的代码上失败的测试（网页的用 vitest 或 e2e，服务端的用 Go 的测试），或在 Step 的记录里写明为什么只能人工核对；每个 Step 有变异表。
   - 焦点的修复在三种 Playwright 引擎里探查过（05-lessons 第 6 节）；改到的故事照需要进 `threeBrowsers`。
   - 移到发布之后的项都在 `manual/` 的清单里；C 类都在 v0.2 的移交里。
4. **第 302 条**（P3/S2）：
   - `TestRevertingARollbackConflictsUnderANewFolderHoldingBaselineAndNewPages`（`bootstrap/upgrade_rollback_revert_test.go:30`）的 nested 一格（`:87`）翻过来：请求的撤销与后台的撤销各在一本笔记本上成功，树回到回滚之前。测试随之改名，第七、第八份的模式（`Makefile:296`、`:328`）一并改。
   - 两个已有的性质测试扩开：`TestTheRollbacksStepsReplayToTheBaseline`（`page/app/rollback_test.go:560`）在回滚 `Done` 之后撤销回滚的变更集，重放之后等于 agent 留下的状态；`TestTheJobsStepsReplayToStar`（`page/app/revert_job_property_test.go:64`）的生成器加嵌套回收站根的形状，带出现次数的下限。"去掉新规则""`topOf` 走过回收站根"两个变异都让它们失败。
   - 不是回滚的"恢复之后先删里面、再删外面"的变更集有一格确定的测试：照常撤销。
   - M8 原有的撤销测试与性质测试照过。
5. **其余的服务端正确性**（P3/S1、S3）：
   - 25.1：`TitleKey` 幂等的样本含切罗基文；只差切罗基文大小写的两个标题在同一父页下被拒；删掉 `searchFold` 的映射前后，同一组查询的搜索结果相同。
   - 10.2：标题里有自动链接与原始 HTML 时，id 含它们的文字。
   - 10.1：标签列表不列 `/`。
   - 30.2：只由默认可忽略的字符与空白组成的显示名答 `required`，这样的客户端名当作空的；带 ❤️、ZWJ 的 emoji 与波斯文的标题与名字照旧通过；`TestChecksRejectCounterexamples` 不改，照过。
   - 30.4、30.5：`log` 的 `path:""` 答字段错误，`path:"/"` 是整本；`index` 的 `parent:"/"` 照旧是根。
   - H5：日志的读与一次撤销的提交交错时，读到的要么都有、要么都没有（确定的交错测试，13.4 第 4 条）；一个变更集的详情同样。
   - MCP 调用超时：答复的文字说明期限已到、什么也没有写、`batch` 拆小再试，不再只是通用的内部错误。
   - 流数：一个凭证开第 5 条流时，这个凭证最老的那条以新的原因 `superseded` 被重置，别的凭证的流不受影响；账户已有 64 条时，一个已有 4 条的凭证开第 5 条照样顶掉它自己最老的一条、不答 429；只有净增一条的第 65 条答 429 带 `Retry-After`；一个账户 17 个凭证、被重置就立刻重连的客户端，在有限步之内停下（hub 的单元测试与一个整个程序的测试）。
   - T3：中文 Windows 打包、没有 UTF-8 标志的 zip 导入之后名字对，报告写明按 GBK 解读；带 Info-ZIP Unicode 路径的条目取那个名字；有一个非 UTF-8 的名字按 GBK 也解不开的包，整包不回退。
   - T2：一个人排队与运行的任务到上限时，他的下一个答按人的新码，文字是"你自己已有几个任务在排队或运行"那一句（两种语言）；别人的照常开始；全局满了照旧答 503 `server_busy`。
   - T4：上一个进程在 River 取到之后、开始之前停掉的任务，服务启动之后是失败 `interrupted`，名额空出（整个程序的测试）。
6. **实测**（P4）：
   - 4.7.1 表有预算的每一行在 P4 的记录里有：机器、规模、种子、两轮的数、在哪台机器上判（本机、CI 的实数，或乘了哪个倍数）、对照预算的结论（过、超）。B5 的规则一层没过的，记录写明列给主持者与负责人定的两种做法与他们的决定。只量的各项有数，或写明"推算"与它依据的数。
   - 页面树的三条界线在量之前写进 P4 的设计，量之后逐条对照。
   - 每一个超出的已修，带缩小规模的守卫；或经负责人改判。其余写进已知限制与 v0.2 的移交。
7. **部署文档**（P5/S3）：
   - `deploy/README.md` 覆盖第 7 节"移交的落实"里每一条部署项，逐条对照表在 S3 的记录里。
   - `deploy/compose.yaml` 由 `make deploy-check` 核对，CI 的 `image` 一份跑它。
   - 照 `deploy/README.md` 用 `deploy/compose.yaml` 在本机做过三遍：从零装一遍；改镜像的 tag 升一次级（备份之后 `docker compose run --rm migrate`）；备份再恢复一遍。记录写命令与结果。包设为公开之后，用 GHCR 的镜像再装一遍（4.11 第 5 步）。
8. **版本号**（P5/S1、发布）：
   - Makefile、buildinfo 与 Dockerfile 两个 `ARG` 的默认值一致，由短测试守住，有反向对照。
   - `0.1.0` 的镜像里 `nervewiki version` 与 `/api/v0/instance` 都是 `0.1.0`：两种架构上的 image-smoke 核对。
   - MCP 的 `serverInfo.version` 取 `buildinfo.Get().Version`（`bootstrap/mcp.go:90`），一个短测试断言两者相等，有反向对照（P5/S1）。
   - 发布之后 `main` 的默认值是 `0.1.1-dev`，`README.md:43` 与生成的契约随之。
9. **发布流水线**（P5/S1、S2、发布）：
   - `release.yml` 的演练作业只有 `contents: read`，在分支与 `main` 上两种架构都绿；读工作流的检查不许它有写权限，有反向对照。
   - 闸门只认 `ci.yml` 的运行："`ci.yml` 红、`release.yml` 绿"的提交被拒（本机对着假输入的反向对照）。
   - `v0.1.0-rc.1` 与 `v0.1.0`：Release 在；镜像两种架构都能拉；`gh attestation verify` 与 `cosign verify` 都过；SBOM 与许可证在 Release 的附件里。证据是收尾记录的"发布"一节：负责人对两个 tag 的回答与日期、run 号与摘要。
10. **移交与文档**：
    - 六份移交都改为 `done`；v0.2 的移交在 `docs/v0.2/handoffs/`，v0.1.1 的在 `docs/v0.1.1/handoffs/`。
    - `v0.1-design.md` 照第 7 节"总体设计的修订"改过，12.6 的 M12 是"实现完成，待人工验收"。
    - README、`CHANGELOG.md`、`SECURITY.md`、`manual/` 的清单都在。
    - `CHANGELOG.md` 的已知限制一节逐条对照 4.13 列的来源（H1、24.1、Shift-JIS 等的误读与 CP437、`batch` 的期限、P4 只量的、运维相关的、续期途中刷新被登出），对照表在 P5/S3 的记录里；表的最后一列写每一项进了哪一份移交，没有去处的不许有。
11. **CI**：每个 Step 合并之后，两次 `main` 的组合根每一份的 Test 一步平均在 450 秒之下（06-environment 第 4 节）；e2e 在它 20 分钟的时限之内（`.github/workflows/ci.yml:263`）。`release.yml` 的演练与 `vuln.yml` 不在组合根里，不算进各份。

## 4. 关键决定

决定的编号从 304 起（303 是 M11 收尾用掉的，`00-M11-design.md:950`）。初稿的临时号 `M12-n` 与正式号的对照在[审查记录](reviews/M12-design-review.md)。设计审查改过的决定标"（设计审查 X 改）"。

### 4.1 次序：地基先行，实测在代码定下之后

- **P1 先做**：
  - e2e 的 `goto` 挡发布。发布要 `main` 的 CI 为绿，而 Playwright 不重试（`e2e/playwright.config.ts` 没有 `retries`）。M11 收尾之后 `main` 的 run 43339247136 里，LW4 落到了登录页，与 M10 查明的 LW6 同形；是不是续期没有核实（[M11 收尾审查](../M11-upgrade/reviews/M11-closeout-review.md)"门禁"一节；移交 M5-polish 第 51 条）。它先做，后面每个 Step 的 CI 才稳。
  - 工具链与 PostgreSQL 的升级随后。之后的实测、计划测试与成本守卫都在最终的工具链与数据库上跑（I1-D12）。
- **P2、P3、P4/S1、P5/S1 可以并行**：它们改的部件除翻译文件外不重叠（第 7 节"次序与并行"）。重的运行照旧经 `heavy.sh` 串行。
- **页面树的结论先于 P2/S6**：P2/S6 改 `PageTreeStore` 的几项（12.12、27.2、22.14、23.10）。1 万页过不了 4.7.2 的界线时，树要改，它们排在改法之后（I3 第 8 节）。所以 P4/S2 先量页面树。
- **服务端的实测（P4/S3）在 P3 合并之后**：P3 改了撤销的计划（302）、日志的读（H5）、标题键（25.1）与任务的入口（T2），量在改之后才算数。
- **部署文档（P5/S3）在 P4/S3 之后**：它要内存、磁盘、WAL 与硬件下限的数。发布说明的已知限制取自 P2–P4。

### 4.2 P1 之前已提交的两件：heavy.sh 的锁与 `ci-wait.sh`

**heavy.sh 的锁**（`8c247237`）：

- 锁的默认原是 `${TMPDIR:-/tmp}/nervewiki-heavy.lock`（`5e8af5b5` 的 `docs/v0.1/claude_to_codex/tools/heavy.sh:8`）。几个 agent 各设 `TMPDIR` 时，锁就各是各的，重的运行不再串行。M12 的调查期间就发生过（I2 第 8 节第 6 条）。
- 改为固定的 `/tmp/nervewiki-heavy.lock`，仍可由 `NW_HEAVY_LOCK` 改（主持者的决定 309）。
- `06-environment.md` 第 3 节写了两件事：`/tmp` 有用户配额，大的输出与 `TMPDIR` 放 `~/.cache` 下自己的目录；改 `TMPDIR` 不再改变锁。
- 主持者在设计定稿之前单独提交、推上了 `main`：`8c247237`，`docs: heavy.sh's lock stays in /tmp whatever TMPDIR says (M12)`。

**`ci-wait.sh` 按工作流取**（`b54cd679`；设计审查 B 的 I3，修订核对第 1 轮 B 的 M16）：

- 它原按提交取第一个运行（`5e8af5b5` 的 `docs/v0.1/claude_to_codex/tools/ci-wait.sh:13`，`runs?head_sha=$sha&per_page=5` 的 `r[0]`），不看是哪个工作流，`:38` 按这一个运行的结论退出。
- M12 加的两个工作流会在同一个提交上另起运行：P1/S2 起的 `vuln.yml`（每天与手动的运行跑在那时 `main` 的最新提交上，改了依赖清单的推送也跑），P5/S1 起的 `release.yml` 的演练。后建的若是它们，`ci-wait.sh` 等的就是它，`ci.yml` 还在跑或红着也以 0 退出，Test 一步的用时一行也没有。
- 改为按工作流取，默认 `ci.yml`，第二个参数可以改成别的工作流，第一行写明取的是哪个。`06-environment.md` 第 4 节随之。
- 原定在 P5/S1 的第一个提交里改；P1/S2 的 `vuln.yml` 就会碰上，所以主持者在 P1 之前单独提交、推上了 `main`：`b54cd679`，`docs: ci-wait.sh waits for one workflow's run, ci.yml unless told (M12)`。

### 4.3 工具链与依赖：只做与安全相关的更新

照 I1-D10、D12（主持者的决定 310）。

- **Go 1.27.1 → 1.27.4**：1.27.2–1.27.4 修了 `net/http`、`crypto/tls`、`crypto/x509`、`html/template` 等的安全问题，都在服务端运行时的路径上（I1 第 5 节）。
  - 改 `server/go.mod:5`、`server/tools/go.mod:5` 的 `toolchain`，`deploy/Dockerfile:23` 换成 `golang:1.27.4-trixie` 的摘要，`README.md:16` 随之。
  - golang 镜像设了 `GOTOOLCHAIN=local`：只改 `toolchain`、漏改 Dockerfile 时，镜像悄悄用旧的 Go 构建（I1 第 5 节）。`FROM` 同时写 tag 与摘要时，生效的是摘要，只比文字盖不住"tag 改了、摘要没换"。所以 Dockerfile 的 server 阶段加一行构建时的断言 `RUN test "$(go env GOVERSION)" = go1.27.4 || { echo …; exit 1; }`；一个短测试核对 tag、这一行与 README 写的版本都等于两份 `go.mod` 的 `toolchain`（设计审查 B 改）。
  - 核对两处写着 1.27.1 的代码注释：`platform/httpserver/mcpserver/edge.go:106`（`encoding/json/v2` 在默认打开的 GOEXPERIMENT `jsonv2` 之后，M9/P1 要求升级时核对它仍打开），`page/domain/grep.go:149`（引了标准库 `regexp/syntax` 的行号）。
  - 文档里写着 1.27.1 的五处随之（设计审查 B 改）：总体设计 13.1 第 32 条（`v0.1-design.md:1178`，照 `jsonv2` 的核对结果写，第 7 节"总体设计的修订"第 18 条）；`06-environment.md:10`、`:52`，`05-lessons.md:89`，`03-project-background.md:15`。后四处写成"`go.mod` 的 `toolchain` 那个版本"，免得每次升级都要改。
- **PostgreSQL 18.6 → 18.7**：18.7 有一批 CVE 与 GIN 的修复，正对着 `page_search` 的索引（I1-D12）。
  - 改守卫里的四处与 `README.md:629`；`pgtest/image_test.go` 把 README 也核对进去（I1 第 8 节第 6 条）。
  - P5/S3 把 README 那一句挪进 `deploy/README.md` 时，守卫里的这一项随之换成 `deploy/README.md`，并加上 `deploy/compose.yaml` 的镜像与 initdb 参数（设计审查 B 改）。
  - 总体设计 7.1 的例子随之改（第 7 节"总体设计的修订"）。
  - 计划测试（13.4 第 9 条）在 18.7 上跑一遍；计划换了，在 S2 里查清，不放宽测试。
- **三个基础镜像的摘要**：node（Node 24 线到 24.22.3）、golang、distroless（I1 第 5 节）。
- **两个 action**：`actions/setup-node` v7.1.0、`actions/upload-artifact` v7.0.2，按 SHA（I1 第 5 节的表）。
- **漏洞扫描**（主持者的决定 308，设计审查 B 改）：
  - **只在 GitHub 的 runner 上跑**：`govulncheck`、`pnpm audit` 与 `make outdated` 本机都不跑。理由照实写：M12 调查时，本机的权限分类器拒过 `govulncheck`；调查与审查的规矩是"不往外发"，而 `pnpm audit` 与 `make outdated` 同样把依赖清单发给 registry 或 proxy。初稿只禁了 `govulncheck`，理由写成"它把模块清单发给 vuln.go.dev"，与它的文档不符，又让送得更多的 `make outdated` 在本机跑。
  - **`make vuln` 的两侧**：
    - Go 一侧：`go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...`，只扫 `server/`。`server/tools/` 是不随镜像发出的代码生成器，又用 `tool` 指令，`./...` 扫不到工具本身；那里报出的问题既难修、又与发布物无关（设计审查 B 的过度设计）。不放进 `server/tools/go.mod`：那里的 kin-openapi 等跟着 oapi-codegen 的要求（`server/tools/go.mod` 的注释），新的工具依赖会牵动它们。
    - npm 一侧：`pnpm audit --prod`。pnpm 读得懂两个文档的 `pnpm-lock.yaml`。GitHub 的依赖图读不出这种锁文件（`pnpm-lock.yaml:1`、`:158` 各有一个 `---`；GitHub 文档写明 pnpm 12 钉了版本时的这种格式认不出），Dependabot alerts 因此看不见 npm 的公告，不再当作手段。仓库是公开的，锁文件本来就公开。
    - npm 一侧的判据（修订核对第 1 轮 B 的 M15）：`pnpm audit` 默认 `low` 也以 1 退出，又不看我们是否调到了有问题的函数。`make vuln` 带一个级别（例如 `VULN_AUDIT_LEVEL`，默认 `low`）：`vuln.yml` 照 `low` 跑，报出来给人看；发布的闸门传 `high`，`--audit-level=high` 才挡。
    - 修不了（没有补丁，或要升 4.3 不升的功能性大版本）或确认与我们无关的公告，写进 `pnpm-workspace.yaml` 的 `audit.ignore`（GHSA 号），理由写在那个提交与收尾记录里。收尾之后为此加的忽略，算改了产品的依赖配置，照决定 324 问负责人。
  - **不进每次推送的 CI**：Go 的小版本约每月一个，多数带安全修复；`govulncheck` 找到就非 0 退出。放进必绿的 `short` 一份，下一个安全版本一出，每个 Step 都红，连只改网页的 Step 也合并不了。`make vuln` 放在一个单独的工作流 `.github/workflows/vuln.yml` 里：`schedule` 每天一次，加 `workflow_dispatch`，另加一个 `push` 触发，`paths` 只写它自己与依赖清单（`.github/workflows/vuln.yml`、`server/go.mod`、`server/go.sum`、`pnpm-lock.yaml`、`pnpm-workspace.yaml`，以及定义 `make vuln` 的 `Makefile`、`make outdated` 的 `tools/outdated.sh`；修订核对第 2 轮 B 的 N12，P1 设计 2.1 第 13 条），只在分支的推送上（`branches: ["**"]`），tag 不触发。`workflow_dispatch` 与 `schedule` 只对已在默认分支上的工作流生效，引入它的 P1/S2 分支与升依赖的分支靠 `push` 跑（修订核对第 1 轮 B 的 M13）。发布的闸门硬挡（4.10）。
  - **红了先要有人看见**：定时运行的通知发给最后改 cron 那一行的人，也就是推送用的负责人的账户，主持者收不到（修订核对第 1 轮 B 的 N9）。所以每个 Step 合并之前（02-workflow 第 3.6 节的合并步骤），主持者看一眼 `vuln.yml` 最近一次的结论（`gh run list --workflow vuln.yml --branch main --limit 1`）。
  - **红了怎么办**：主持者开旁支 `m12-toolchain-<版本>`，照 P1/S2 的做法升（两份 `go.mod`、Dockerfile 的 tag、摘要与断言、README、守卫；npm 一侧升报出的包，或照上面写进 `audit.ignore`），提交写当时所在的 Phase。收尾合并之后才由闸门发现的，改了产品代码或依赖配置，照决定 324 再问负责人。
  - 公开仓库 60 天没有活动时，定时工作流会被自动停用。发布之后若久无提交，它会悄悄停掉：写进给 v0.1.1 的移交（4.13）。
  - S2 先推一次分支（改了 `vuln.yml` 与依赖清单，`push` 触发它跑），读结果，再升报出的模块。报出的若是 River、`x/text` 这类有连带的模块（新的迁移、换 Unicode 表就要 reindex），在 S2 的记录里写明连带，照做或请主持者定。
- **不升的**（I1-D9、D10、D11）：
  - Node 26：Node 只用于构建与 e2e，运行时镜像里没有；Node 24 维护到 2028-04-30；换 26 要改 corepack 的四处、`engines` 与 `@types/node`。移交 M0-P6 第 4 条的"Node 26 成为 LTS 之后评估"由此了结：评估过，v0.1.x 之后再换。
  - 功能性的大版本：River 0.51（可能带新的迁移，`server/migrations/river_test.go` 会失败）、katex 0.19 与 `@lezer/markdown` 1.8（要重核补丁，13.5 第 5 条）、mermaid 12、pgx、goose、`x/text`（换 Unicode 表就要 reindex，总体设计第 14 节）。
  - golangci-lint 2.15、pnpm 12.18：只用于开发，不涉安全。
- **`make outdated`**（I1-D11）：只读地列出过时的 Go 模块、npm 包、actions 与镜像，做的就是 I1 第 5 节那张表。它在 `vuln.yml` 里一个手动触发、改了依赖清单的推送也跑的作业上（`continue-on-error`，它的失败不让运行变红），每次发布之前跑一次。Dependabot 的版本更新 PR 不开：每个 PR 都触发 19 项的 CI，版本精确固定、补丁要人工核对，自动的 PR 大多会红。要不要开由负责人定（移交 M0-P6 第 4 条写明"由负责人决定"；第 11 节）。

### 4.4 e2e 的帮手：`goto` 与 `reload`

照 I3 第 7 节第 14 条（主持者的决定 311，设计审查 C 改）。

- **成因**：故事在页面的读还在途时换文档，在途的续期被打断。服务端已换了令牌、浏览器没收到，再用旧令牌就算重复使用，会话被撤销，页面落到 `/sign-in`（移交 M5-polish 第 51 条；M1 第 10 节接受了这个后果，`docs/v0.1/M1-auth/00-M1-design.md:193`）。`e2e/stories` 里 109 个文件、259 处 `goto`、10 处 `reload`；夹具里另有一处 `goto`（`e2e/fixtures/wiki-health.ts:157`）。
- **帮手**：
  - `page` 夹具记下每个在途的 `/api/v0/` 请求（`request`、`requestfinished`、`requestfailed`），事件流除外。
  - 帮手在 `goto` 与 `reload` 之前等这些请求答完，其中一定等 `POST /api/v0/auth/refresh`。这两个换掉文档，打断在途的请求。
  - `goBack`、`goForward` 不一律算进来：故事里的 14 处多半是单页应用自己的历史条目，同一个文档里的 popstate，打断不了续期。14 处都在同一个文档里（P1 设计附录），帮手不另给跨文档的后退；以后要它的故事先在帮手里加，照 A15 加一格对照（P1 设计 2.1 第 1 条）。
  - **逃生口**：帮手带一个显式的选项 `{ inFlight: "leave" }`，不等就离开，只给有意在请求途中离开的故事用；P1 里只有 A15 的两格（P1 设计 3.6）。TR2 扣住导入的 POST 之后的后退（`e2e/stories/transfer/tr2-import.spec.ts:241-259`）在同一个文档里，帮手与门禁都不管，用不到逃生口。
  - `pageWatch`（`e2e/fixtures/browser.ts` 的 `watchPage`）认出没有预期的 `/sign-in` 落点，失败时写明"被跳回登录页"：它记下每一次落到 `/sign-in` 与之前那次续期是否答 401，失败时页面的状态第一行写出（P1 设计 3.3）。
  - **第一次续期**（审查 C 未核实的怀疑 U1）：`goto` 返回时，新文档的第一次续期常还没发出（探针：Chromium 15 次里 9 次、WebKit 15 次里 8 次；续期在 `navigator.locks` 授予之后才发，`session/token-manager.ts:289`）。只等在途的请求时没有复现失败（三种引擎各 20 次，0 次落到登录页）。帮手再等这个文档的第一次续期已答、续期的锁空着。A15 的第 4 格不经锁推迟第一次续期，只看在途请求与锁的帮手在 Chromium 与 Firefox 里落到 `/sign-in`；第 6 格在第一次续期之后持住锁，不看锁的帮手在放开之前就换页。WebKit 在请求下一个文档之后旧文档发不出请求，没有这个窗口（P1 设计 3.2、3.6；P1 设计审查 A-I1、C-I1）。
- **门禁**：`e2e/` 里（故事、夹具与以后的 `e2e/perf/`）不许直接调 `goto`、`reload`，只许经帮手。帮手所在的那一个模块是唯一直接调用它们的地方，门禁对它整个例外：不报它的直接调用，也不数它里面的逃生口（修订核对第 1 轮 C 的 N8；P1 设计审查 A-M1、C-M2）。逃生口要显式写，门禁按它的表核对每个文件的逃生口的个数，理由写在表里。门禁是 `tools/e2e-navigation/check.mjs`（`make lint-web`），先在样例上证明它看得见每一种违例，带反向对照（13.4 第 1 条）。
- **对照测试**：把续期的答复延后（`route.fetch()` 之后、`fulfill` 之前扣住，同 `a5-refresh-reuse.spec.ts` 的第二个测试），再 `goto`。经逃生口不等时落到 `/sign-in`，经帮手时不落。对照测试是故事 A15（`e2e/stories/identity/`，三种浏览器）：续期的答复在途时、第一次续期还没发出时，经逃生口离开都落到 `/sign-in`（后一格只在 Chromium 与 Firefox：WebKit 在请求下一个文档之后旧文档发不出请求）；经帮手都不落；另几格各核对帮手等第一次续期、续期的锁与在途的读（P1 设计 3.6）。
- **不改产品**：M1 第 10 节的取舍照旧。用户在续期途中刷新同样会被登出：写进已知限制，并进 v0.2 的移交 `M12-release.md`，留给 v0.2 重看 M1 第 10 节的取舍（设计审查 C 的 N6）。
- **第 50 条**：`e2e/global-setup.ts:14-66` 的网络等待触发时打一行日志，看 CI 的 runner 上它到底触发了没有（I3 第 9 节第 50 条）。

### 4.5 打磨：分级与范围

照负责人第 1 问与 I3 的分级；罕见的 A 类与只从 ARIA 推断的几项照负责人 2026-11-14 第 5、6 问（主持者的决定 312，负责人第 5、6 问与设计审查 C 改）。

- **分类**（I3 第 0 节）：
  - A：用户会碰到的错误行为或数据问题；
  - B：无障碍问题或误导的文字；
  - C：纯体验的改进；
  - D：用户碰不到的（测试、结构、日志）；
  - M：只能人工确认的。
- **照 I3 第 9 节的分类**。备注里"给主持者"的条目不改分类，除了下面几条。依据是负责人第 1 问所选的字面：会碰到的错误行为、无障碍问题、误导的文字在 v0.1 修。
- **移出 v0.1 的**（都经负责人 2026-11-14 回答）：
  - **只从 ARIA 推断"念两遍、念得急"的八项**：16.8、18.4a、21.15、21.16、22.10、26.1、26.2 的后一半、28.3。并进发布之后的读屏验收，确认有问题的在 v0.1.1 修（第 6 问："发布后随读屏验收"）。理由：负责人第 4 问已把读屏放在发布之后；这些没有实测，改了也要靠读屏核对。初稿列了九项；49 移回 P2/S1：它与留在 v0.1 的 24.3 是同一种问题，撤销对话框里一句说明没进 `AlertDialogDescription`，确认框打开时读屏不念（`web/…/pages/notebook/revert-dialog.tsx:331`、`:379`），改法同样是进 `aria-describedby`（设计审查 C 的 M1）。
  - **罕见、改动大的两项 A**：H1、24.1，写成发布说明里的已知限制，进 v0.2（第 5 问："小的修、大的进 v0.2"）。理由照实：H1 要同一间隔连着插约 30 次之后撤销才误报冲突，改动大、要迁移；24.1 要网络结果未知时才可能指到别处的任务，改动大（I3 第 3、9 节）。初稿写"罕见又大"，把 T4 也算进去，I3 判 T4 是"小到中"。
- **移进或留在 v0.1 的**：
  - **T2、T4**（任务，都是 A）：照负责人第 5 问在 v0.1 修，P3/S3（4.6.6，主持者的决定 336）。T2 一个人能占满全局的任务队列，改动小；T4 任务取到之后、开始之前停机，占着名额约 7 小时，改动小到中。初稿把 T2 与 T1、H4 并在一起写成"写明、进 v0.2"，没有承认它是 A（设计审查 C 的 I1）。
  - 第 48 条（横幅出现时正文跳 94 像素）算 A，与 3、8.2、17.1 在同一个 Step，改的是同一个 effect（I3 第 7 节第 13 条）。
  - 23.17 与 28.2（`aria-disabled` 的按钮看着可按）是 C，随 23.16 一起改。不改的话，`ConfirmDialog` 换成 `aria-disabled` 之后，发送中的按钮看着能按，是一个新的误导（I3 第 9 节 23.17 的备注）。
  - H4 的界面一半：回收站与版本列表写明永久删除还不能做、回收站的项在期限之后由系统清除（移交 M8-history 第 4 条要求"在部署文档与界面上写明"；I3 第 5 节第 8 条）。界面文字不写版本号，到 v0.2 不过时；放在版本列表上时不能读成"版本也会清除"。P2/S2 定文字时照此（设计审查 C 的 N5）。
- **几条做法**：
  - 英文单复数：照已有的 `*One` 键，每个要单数的地方加一个键，不引入 `Intl.PluralRules`（I3 第 7 节第 6 条）。
  - 第 7 条退出登录丢文字：做"仍要退出吗"，本标签页的编辑与经频道报告的都算；Web Locks 的登记进 v0.2（I3 第 7 节第 7 条）。
  - 第 40 条：两份 `AGENTS` 模板都改"三个 prompt"的说法，已有笔记本里的 `AGENTS` 不随之改（I3 第 7 节第 12 条）。
  - 33.1（Safari 窄屏的搜索页宽到 704）：在 P2/S3 里用 Playwright 的 webkit 只跑 SE1 的 375 一段排查。复现了就是 B（WCAG 1.4.10），在 S3 里修；没复现，留在发布之后的人工清单（I3 第 7 节第 15 条）。
- **D 类**：只做挡发布或顺手的几项：50、51（P1/S1）、14d（随 P2/S4）、32.3（随 P2/S3）、24.8 与 24.10（随 P2/S6，TB2 偶发的根源）、26.15（随 P2/S6）。其余进 v0.2。
- **总体设计第 13.2 节**：第 12 条"`ConfirmDialog` 发送中仍用 `disabled`，留到 v0.1 之后的打磨"随 P2/S1 改写；第 26 条"无主笔记本接管之后的焦点（M3）还没有照此"与第 1 条"见 M12 的体验移交第 7 项"同样随 P2/S1 照实改写（第 7 节"总体设计的修订"第 11、17 条）。
- **"v0.1 之后"的说法**：13.2 第 12 条与打磨移交里的"v0.1 之后"原意是"M0–M11 之后、在 M12"（I3 第 5 节第 2 条）。M12 的文档一律写"M12（v0.1）"或"v0.2"，不再写"v0.1 之后"。
- **逐项的去处**在附录 A：进 v0.1 的打磨约 113 项（A 31 项、第 48 条、B 81 项），每项写到 Step；任务的 T2、T4 在 P3/S3。

### 4.6 服务端的正确性与历史

#### 4.6.1 第 302 条：回滚的变更集撤销答冲突（H6）

- **字面的改正**（主持者的决定 304）：M11 第 302 条写"v0.1 之内不修"（`00-M11-design.md:949`），本意是"M11 之内不修"。总体设计 5.8 第 6 步写"M12 修"（`v0.1-design.md:580`）。负责人第 1 问已点名 302（"另有历史移交 6 条（含 M11 决定 302 的回滚撤销冲突）"），选的是"分级处理"；它是 A，在 M12 修。00-M11 的第 302 条加一句括注指过来，不改写原文（第 7 节"总体设计的修订"第 15 条）。
- **形状**（移交 M8-history 第 6 条；设计审查 A 在 domain 与整个程序上复现）：
  - 回滚每一遍先在自己的单元、自己的时刻删掉不含基线节点的新建子树（`page/app/rollback.go:117-122`），下一遍才删收着基线节点的新建节点。agent 新建一个文件夹、把原有页移进去、又在里面写新页时，新页先被删成一个回收站根，下一遍才删文件夹。
  - 第二个来源：`PackFreeSubtrees` 对超过 `whole` 的自由子树自下而上地删，下一遍才删它的根（`page/domain/rollback.go:60-69`）。没有任何基线节点也会造出同一形状，而且可以多层：里面的根挂在顶的组的中间节点下。
  - 撤销时，M8 的计划在结构上把里面那页的顶定为文件夹（`topOf`，`page/domain/revert.go:275-326`）。先失败的是 `rootHolds`（`:337-353`）：文件夹那一项回收站的 `TrashPages` 是 1，这个变更集在它下面删的却是 2 页，`RevertGroups`（`:196-206`）不读这个组；`leftDeleted`（`:502`）随之答 `ConflictChanged`。初稿把失败写在 `groupIsDeleted`，不对（设计审查 A 的 M1）。
- **一次恢复只放回一个回收站项**，这条事实定了修法的形状：
  - `RestoreNodes` 要每个节点在同一个 `deleted_at`、只认一个根（`page/adapter/postgres/queries/nodes.sql:258-261`）；
  - `bringBack` 只带一个时刻（`page/app/unit_restore.go:115-118`），行数不对答 `ErrTrashChanged`（`:149-150`）；
  - 调度按顶逐个放回，`restore()` 取的是那个顶自己的组与时刻（`page/domain/revert_schedule.go:328`、`:431-436`）。
  - 所以初稿的"把里面的根并进顶的组、先顶的组再里面的组"做不成一次放回，只能是两个放回的步。
- **修法**（主持者的决定 304，设计审查 A 改）：**一个被这个变更集删了、现在本身是回收站根的节点，是它自己的顶。**
  - 改在 `readReverted`/`topOf`：往上走到这样的节点就停。`RevertGroups`、`scheduled`、`CheckRevertPart`、`CheckRevertSteps`、`RevertActionFacts` 传入 `Nodes`。
  - `groupIsDeleted`、`leftDeleted` 与调度都不改。调度自己先外后里：`canRestore` 要父节点活着。
  - 后台撤销的 `revertSteps`/`needed`（`page/app/jobs.go:578-585`）随之改。它的注释写明的前提是"a restore under a member of another group is a nested trash item, which L refuses"，修法让 L 接受了嵌套的回收站项，前提不再成立。`needed` 也看段内后面的放回的父节点。审查 A 没有造出出错的交错，前提已不成立，照改（设计审查 A 的 M3）。
  - 不改 M11 的"一个任务一个变更集"与任务答复的 `changeset_id`，不改契约。
  - 审查 A 的原型（`split-tops.diff`，约 10 行）在 nested 形状上跑通：PlanRevert 给出 `[restore Made, restore Inner, place Guide]`；整个程序上请求的撤销答 200，后台的撤销 succeeded，笔记本等于 agent 留下的样子；page 模块的测试全过。原型只作参照，P3/S2 的 spec 照它核对，不是定稿。
- **认出的形状**，照实写。初稿写"只认这一种"，列了三个条件；它们实际上就是"本身是回收站根、被这个变更集删的节点"（设计审查 A 的 M2）。认出的有三种：
  1. 回滚第一遍对计划那一遍：新文件夹收着基线页与新页；
  2. `PackFreeSubtrees` 的自下而上，可以多层；
  3. 不是回滚的：变更集 X 一次删了一个文件夹与它下面的页（一个回收站项）；之后有人恢复了这一项，先删那页，再删文件夹。撤销 X 今天答冲突，修了之后照常撤销。按状态比较的原则这是对的：今天"恢复之后整个再删"就照常撤销。spec 写明这一点，有测试钉住。
- **测试**：
  - 性质测试走现成的路（设计审查 A 的 M2、M3）：
    - `TestTheRollbacksStepsReplayToTheBaseline`（`page/app/rollback_test.go:560`）的 `whole` 本来就是 1–4，前两种形状自然出现。在回滚 `Done` 之后撤销回滚的变更集、重放，对照 agent 留下的状态。
    - `TestTheJobsStepsReplayToStar`（`page/app/revert_job_property_test.go:64`）的生成器加嵌套回收站根的形状（含挂在组的中间节点下的），给它出现的次数加一个下限：数形状真的出现的次数，不是规则触发的次数（05-lessons 第 1 节）。
  - 第三种形状一格确定的测试。
  - 变异：去掉新规则；`topOf` 走过回收站根；`needed` 不看段内后面的放回。初稿的"放宽到任何回收站根"与新规则等价，会活下来，删掉；"`RevertGroups` 不读里面的组"随修法不再适用，删掉。
  - `TestRevertingARollbackConflictsUnderANewFolderHoldingBaselineAndNewPages` 的 nested 一格翻过来。今天那一格在同一个变更集上先同步撤销、再后台撤销（`bootstrap/upgrade_rollback_revert_test.go:99-110`），只因两次都冲突才行得通；修好之后第二次撤的是已经撤过的变更集。改为两本笔记本，每条路各回滚一本（审查 A 的 `bootstrap_zz_m12review_test.go` 的做法）。测试照新的结论改名，`BOOTSTRAP_SEVENTH`（`Makefile:296`）与 `BOOTSTRAP_EIGHTH`（`:328`）的模式同时改；漏了的话新名字以 `TestR` 开头，落进补集的第四份（设计审查 A 的 N4）。
- **不去造的**：初稿要 P3/S2 的 spec 去造"后台恢复与后台撤销里的同一形状"。审查 A 已核对造不出，从 spec 里拿掉（设计审查 A 的过度设计 1）。理由：后台恢复的任务只放回、不删；后台撤销的一次删除是一个步、不跨批（`page/app/jobs.go:494-499`），`tops(s)` 不会先删"上级是本变更集新建且还活着"的节点（`page/domain/revert_schedule.go:370-384`）；MCP 的 `batch` 把"后删的上级盖住先删的子树"并成一次删除（`page/domain/batch_plan.go`）。只有回滚造得出，就是上面前两种形状。
- **放弃要不要输入名称**：第 302 条让放弃照旧不要求输入名称。修好之后理由更足，不改。
- **P3/S2 先写 spec**：计划是所有撤销都走的路，改动的边界要先定清。

#### 4.6.2 日志的读放进一个快照（H5）

- 日志的读不开事务、不加锁（`page/app/list_changesets.go:51` 的注释）。现在是五条读：`ListChangesets`（`:192`），加 `page/app/history.go:165-180` 的 `ChangesetFacts`、`FirstChanges`、`FirstIngestSources`、`DisplayNames`（I3 第 3 节 H5）。读跨过一次撤销的提交时，可能列出"已撤销"却没有那条撤销。
- 修法（I3 的推荐）：`ListChangesets.page` 包一层 `WithinSnapshot`（`platform/postgres/tx.go:137-144`，搜索已在用，`page/app/search.go:222`）；`HistoryReads` 加 `Snapshots`。MCP 的 `log {path}` 也走 `page`，一并得到。
- 一个变更集的详情（`GetChangeset` 的 `detail`，`page/app/history.go:83-93`）同样调 `logged`，五条读不在快照里，会有同一种不一致。一并包上（设计审查 A 的 N2）。
- 审查 A 核对过：`WithinSnapshot` 不许嵌套；`page` 只由 `Execute` 与 `AtPath` 调，都不在事务里；LLM Wiki 的虚拟 `log.md` 直接调 store（`page/virtual_files.go:108`），不经 `page`。
- 代价：每次读多占一个只读快照的连接。P4/S3 的日志一项照新的读量。

#### 4.6.3 标题键与标题 id：赶在 v0.1.0 之前（25.1、10.2）

两处都改已有的键或 id。发布之前没有生产的库，改动只碰开发库；之后再改，就要每个部署都 reindex（I3 第 7 节第 4、5 条；主持者的决定 313）。

- **25.1**：`shared.TitleKey` 对切罗基文不幂等（`shared/title_key.go:18-28`）：`cases.Fold()` 把切罗基文的大写折成小写、小写折成大写，`TitleKey(TitleKey(x))` 不等于 `TitleKey(x)`。审查 A 复现了：`TitleKey("Ꭰ")` 得 `"ꭰ"`，`TitleKey("ꭰ")` 得 `"Ꭰ"`。
  - 三处同样的折叠一起改：`shared/title_key.go`、`shared/title_key_context.go:29`、`linking/domain/index.go:80,97`（I3 第 5 节第 6 条：移交只写了第一处；审查 A 核对过非测试代码里恰好这三处）。
  - 改为在折叠之后把切罗基文的小写换成大写，同 `searchFold` 的映射；然后删掉 `searchFold` 的映射（`page/domain/search.go:245-256`）。删掉前后同一组查询的搜索结果相同，由测试比对。
  - 不升分词的版本，也不升 `Extractor`：改了之后 `searchFold` 的映射不再起作用，分出的词不变；升版本只会让 reindex 白白重建全部搜索的行（I3 第 5 节第 7 条；README:534：30 万页约 26 分钟）。
  - 网页的 `lib/title-key.ts` 把两种大小写都键成小写，等价类与服务端一致，不必改（审查 A 核对）。
  - 13.1 第 28 条（标题键）随 P3/S1 改。发布说明写一句：v0.1.0 之前的试用库若有切罗基文的标题，跑一次 `nervewiki reindex`。
- **10.2**：标题的 id 丢了自动链接与原始 HTML 的文字（`platform/markdown/parse.go:83-103` 的 `PlainText` 跳过它们，`:174-177` 的 `AnchorID` 用全文）。改为带上它们的文字。
  - 不升提取规则的版本（13.3 第 6 条），不要 reindex。链接的锚点按文字存在 `Link.Anchor`（`linking/domain/facts.go`）；标题的 id 只在渲染时算，`PlainText`、`HeadingID` 在非测试代码里只有 `platform/markdown/parse.go:69` 一处调用；没有渲染的缓存（设计审查 A 的 N1）。
  - 发布说明写一句：这类标题的锚点 id 变了。

#### 4.6.4 小项

- **10.1**：标签 `/`（写作 `#//`）列得出、叫不到。最小的改法：`listTags` 不列 `/`（`linking/adapter/postgres/queries/reads.sql:87-94`，按 `tag_key` 分组，加一个 `<> '/'`），补全随之不列（I3 第 9 节 10.1）。`TagPages` 的前缀比较对 `/` 本来就走不到（审查 A 核对）。
- **30.2**：`shared/name.go:36-38` 的 `Unshowable` 只去掉 Cc、Zl、Zp 与 Bidi_Control。由看不见的字符组成的客户端名与显示名可以冒充空白（移交 M5-polish 第 30 条第 2 点）。改法（设计审查 A 改）：
  - `Unshowable` 不动。它不只管名字：标题（`shared/title.go:35`、`:71`、`:87`）、客户端名（`shared/client_name.go:30`）、库的 CHECK（`server/migrations/sql/00044_page_client_checks.sql:10-15`，由 `migrations/schema_test.go` 的 `TestChecksRejectCounterexamples` 逐码位核对）与网页的 `lib/upload-name.ts:17-19` 都是同一个集合。初稿要把它扩到默认可忽略的字符，会拒带 ❤️（U+FE0F）、ZWJ 的 emoji 与 ZWNJ 的波斯文的新标题与名字，与 `shared/name.go:29-35` 写明的取舍相反，还要迁移两张表的 CHECK。
  - 改为对整个名字的判断，同移交的原意（"没有图形字符的名称算空"）：在 `CheckName` 与 `CleanClientName` 之后，去掉默认可忽略的字符与空白之后什么也不剩，显示名答 `required`，客户端名当作空的（`""`）。只管新的输入。
  - 标题、库的 CHECK 与迁移都不变。
  - 标签块 U+E0000–U+E007F 要不要一并去掉，是 P3/S1 实施时的小决定，记在 S1 的记录里。
- **30.4、30.5**：MCP 的 `log {path:""}` 与 `{path:"/"}` 答"path is required"（`page/app/list_changesets.go:132-134`），文字误导（30.4，B）。`index` 的筛选给 `""` 答 `too_short`（`linking/app/list_index.go:196-201`），工具说明写着 `"" is refused` 与 `"/"` 是根（`linking/adapter/mcp/tools.go:61-62`）（30.5，C）。约定（主持者的决定 314，设计审查 A 改）：
  - `index` 不动：它的 `parent:"/"` 是根（`list_index.go:205-211`），写在工具说明里；`list` 同样以 `""` 与 `"/"` 表示根（`page/adapter/mcp/tools.go:170`）。初稿让 `"/"` 一律答字段错误，会让 agent 列根下页的现有写法 `index {parent:"/"}` 改答 422，是公开的 MCP 行为的回退。
  - 只改 `log` 的 `path`：省略是整本；`""` 答字段错误，文字写"省略它就是整本"（同 `index` 的 `""`）；`"/"` 当作整本（同 `list` 与 `index.parent` 的根）。
  - 细节在 P3/S1 的设计里。
- **MCP 调用超时的文字**（移交 M4-performance 第 22 条之后的补记 `:490`；主持者的决定 335，设计审查 C 的 I2）：
  - 一次 MCP 调用超过 `server.request_timeout` 时答 `internal_error`，文字是通用的"the call ran out of time"（`platform/httpserver/mcpserver/errors.go:77`、`:101`）。最大合法的 `batch` 在慢的 4 核上就会这样：事务整个回滚，不留半截。只说内部错误，agent 不知道该拆小再试，属误导的文字（B）。
  - P3/S1 改文字：说明这次调用超过了请求的期限、什么也没有写，`batch` 拆成几次再试。码照旧是 `internal_error`；要不要改码，由 P3/S1 的设计看契约定。
  - 不给 `batch` 单独的期限：负责人 2026-11-06 的话是不无限优化。写进已知限制，进 v0.2。
  - 硬件的下限写进部署文档（4.8）。

#### 4.6.5 每个凭证与每个账户的事件流数（移交 M5-performance 第 6 条）

- hub 不按账户设上限（`events/app/hub.go:26-31`；`open` 在 `:100-109`，不数）。每条流占一个连接、一个 goroutine 与 64 格缓冲。网页一个浏览器只用一条。
- 加上限（I2-D9；主持者的决定 315，设计审查 A 改）：
  - **按凭证计数**：`shared.Actor` 的 `SessionID` 或 `APITokenID`（`shared/actor.go:15-23`）。每个凭证至多 4 条；第 5 条打开时，重置**同一凭证**最老的那条。被重置的客户端照常重连。
  - **先后**（修订核对第 1 轮 A 的 N10）：先按凭证判。凭证已有 4 条时，新开的顶掉它自己最老的一条，账户的总数不变，不看账户的上限；只有凭证不到 4 条、要净增一条时，才看账户的 64 条。否则账户满了时，睡眠的笔记本电脑留下半开连接、新开的标签页会被拒，这正是选"重置"而不选 429 的原因。
  - **账户一级的硬上限** 64 条：超出时新开的答 429，带 `Retry-After`：一个凭证已被限在 4 条，这一层挡的是一个账户用很多会话或令牌开一批（移交的原意是挡一个账户开太多）。码与文字（沿用 `rate_limited` 或另加）在 P3/S1 的设计里定；网页的 `events/hub.ts` 对它怎样处理，一并核对。
  - 两个上限都是**常量**，不做配置项（设计审查 A 的过度设计 2）。
  - 被重置的那一条记一行 `debug`，或限频的 `info`：带账户 id、凭证的种类与条数，不带令牌（13.1 第 10 条）。
- **为什么按凭证**（设计审查 A 的 I1）：初稿按账户数，超出重置最老的。契约要客户端在任何 reset 之后立刻重连、整体刷新（`events/domain/frame.go:10-11`、`api/modules/events.yaml:54-55`），网页照此不退避（`events/hub.ts:217-219`）。一个账户有 17 个守契约的客户端时（17 个浏览器或配置文件，或照契约写的 PAT 脚本），它们会无限地轮流互顶；每一圈一次认证、几条读（`events/app/open_stream.go:46-60`）、一次全页刷新与一行日志。限流按凭证计（`platform/httpserver/limit.go:24-29`），挡不住。按凭证计数之后：
  - 同一浏览器睡眠之后留下的半开连接是同一凭证的，新开的标签页顶掉它，初稿不选 429 的理由照样满足；
  - 不同设备、不同令牌之间不互顶；
  - 同一凭证上 5 个以上守契约的客户端仍会轮流互顶。它们共用这个凭证的限流，每一圈都受它约束；这是一个凭证开一批的人自己的事，不碍别人。
- **reset 的原因**（修订核对第 1 轮 A 的 N9）：reset 帧的 `reason` 是必填的枚举（`api/modules/events.yaml:239-243`：access、notebooks_deleted、expired、unauthenticated、reconnected、overflow），哪一个都不合"被同一凭证新开的流顶掉"，拿 `overflow` 凑是误导。加一个值 `superseded`，说明照旧写"立刻重连"。网页不看原因就重连（`events/hub.ts:217-219`），所以只改契约与 `make gen`，网页不用改。
- 不选"新的 reset 原因加客户端退避"那一整套：要改契约、`make gen` 与网页的重连；按凭证计数已够。只加一个原因的值不在此列。
- **测试**：
  - hub 的单元测试：同一凭证的第 5 条以 `superseded` 重置最老的；别的凭证、别的账户不受影响；账户的第 65 条被拒；账户已有 64 条时，一个已有 4 条的凭证开第 5 条照样顶掉自己最老的一条、不被拒；一个账户 17 个凭证、被重置就立刻重连的客户端，在有限步之内停下（初稿的写法下不停）。
  - 一个经 `/api/v0/events` 的整个程序的测试：同一凭证开 5 条，最老的那条收到 `reset`。名字照第 7 节"分份"点名进第七或第八份。
- `events.yaml` 与 README 的事件流一节写明两个上限、它们的先后、429 与新的原因 `superseded`。13.1 第 30 条随之改。

#### 4.6.6 导入导出的任务：名称的编码、按人的队列上限、启动时的收拾（T3、T2、T4）

**T3 名称不是 UTF-8 的导入条目**（主持者的决定 316，设计审查 A 改）

- 现在跳过并写进报告：`transfer/domain/entry.go:97-101` 答 `name_not_utf8`；`transfer/adapter/archive/archive.go:305` 照原样取 `f.Name`。推断：旧的 Windows 资源管理器以 OEM 码页打包，中文 Windows 上中文名的页与附件会全部被跳过（I3 第 4 节 T3）。
- 修法，两步：
  1. 先读 Info-ZIP 的 Unicode Path 扩展字段 0x7075。它带原名的 CRC 与 UTF-8 的名字，是确定的。`archive/zip` 不解析它，原始字节在 `f.Extra` 里。CRC 对得上就取它的名字。
  2. 其余的按 GBK 回退，条件三条：
     - 没有 UTF-8 的标志：读 `f.Flags&0x800 == 0`，不用 `f.NonUTF8`。名字不是合法的 UTF-8 时，`archive/zip` 不管标志就把 `NonUTF8` 设为真，用它就盖不住"有标志、却不是 UTF-8"的坏包（设计审查 A 的 N3）。
     - 不是合法的 UTF-8。
     - 按 GBK 解出的结果里没有 U+FFFD。`golang.org/x/text` 的 GBK 解码器从不报错，非法字节写成 U+FFFD，所以"能解开"只能这样定义。

     一个包里只要有一个非 UTF-8 的名字这样解不开，整个包都不回退，照旧跳过。
  - 报告写明哪些名字是按 0x7075 或 GBK 解读的。
- **已知限制**：日文的 Shift-JIS 名字按 GBK 也完整解开，没有 U+FFFD（审查 A 实测："テスト資料.md"成了"僥僗僩帒椏.md"）；Big5、EUC-KR 与 GBK 的双字节区大面积重叠，推断也是这样。这些包今天被清楚地跳过，修了之后会被建成乱码名的页，报告里另列。写进已知限制（4.13）。
- **不做**：导入时选编码的选项；CP437（任何字节串都是合法的 CP437，要另定判别），进 v0.2。
- 外来格式的读取照 13.4 第 11 条：解读只改原本不是合法 UTF-8 的名字，有差分的测试；预读与标准库的对照照旧。`golang.org/x/text` 已是依赖。
- 报告的新原因带两种语言的文字，与服务端一起加（13.2 第 4、11 条）。

**T2 按人的队列上限**（主持者的决定 336；负责人 2026-11-14 第 5 问）

- 现在：`room`（`transfer/app/start.go:212-227`）只比全局的 `CountActive` 加正在上传的导入，至多 `transfer.max_queued`（默认 20），超出答 503 `server_busy`（`ErrQueueFull`，`Retry-After` 5 分钟，`transfer/domain/errors.go:49-54`）。同一人同一本只有一个导出，一本笔记本只有一个写入的任务，跨笔记本不限：一个人可以占满全局的队列（I3 第 4 节 T2）。
- 改法：`room` 另数这个人开始的排队与运行中的任务（`created_by_id`）与他正在上传的导入，到一个常量（例如 5）就拒。常量，不做配置项。
- **拒绝用一个新的码**（修订核对第 1 轮 A 的 M9、C 的 M10）：例如 `transfer.too_many_jobs`，有自己的两种语言的文字，说的是"你自己已有几个任务在排队或运行，等一个做完，或取消一个"。
  - 第一次修订让它沿用 503 `server_busy`、契约不加码。可网页按码出文字：导入对话框把 `server_busy` 显示成"Too many jobs are waiting on the server. Try again in a few minutes."（`pages/notebook/import-dialog.tsx:220`、`en.ts:905`），别处是"The server is busy. Try again in a moment."（`en.ts:1202`）。到了自己上限的人读到的是"服务器忙"，别人的任务却照常能开始，他自己的任务可能还要跑几个小时，`Retry-After` 却是 5 分钟。这是负责人第 1 问要在 v0.1 修的误导的文字，而且是 M12 自己新造的。
  - 状态码推荐 429：这不是服务器忙，是这个人到了自己的份额。P3/S3 的设计照契约的 `x-problem-codes`、权限与码的矩阵、码的次序的约定核对之后定，写明理由。
  - 开任务的四个操作（导出、导入、后台的恢复与撤销，都在 `api/modules/transfer.yaml`）的 `x-problem-codes` 与说明随之；码与原因由 `problem-messages.test.ts` 读契约核对（13.2 第 11 条）。
  - 网页在开任务的各处照新码显示。MCP 的工具不开任务（各模块的 `adapter/mcp` 不调 transfer），不涉及。
  - 全局的 503 `server_busy`（`ErrQueueFull`）照旧。
- **升级的回滚不受按人的上限拒绝**（修订核对第 2 轮 A 的 N14）：回滚的开始也走 `room`（`transfer/app/rollbacks.go:32`、`start_background.go:175-182`、`:200`），`abandonUpgrade` 与驳回因此今天就可能答 `server_busy`。按人的检查若不另作处理，一个人有 5 个任务时连放弃升级都被拒，笔记本停在冻结里，与负责人"任何管理员都能随时放弃、整体回滚"相悖。所以 `room` 对回滚的任务不做按人的检查。回滚算不算进别人的计数、计数与队列的锁的次序，在 P3/S3 的设计里定；若定为也拒，`wiki.yaml` 的 `abandonUpgrade`、驳回与网页的升级页都要列进去。
- 测试：一个人到上限时他的下一个答新码，别人的照常开始；全局满了照旧答 `server_busy`；网页的文字由 vitest 整句比对。去掉按人计数、按人的拒绝答成通用的 `server_busy`，两个变异都让它们失败。

**T4 启动时收拾上一个进程取到而没开始的任务**（主持者的决定 336；负责人 2026-11-14 第 5 问）

- 现在：服务启动时 `AtStart`（`transfer/app/maintain.go:84-95`）只把 `running` 的行记成失败。进程恰在 River 取到任务、任务开始之前停掉时，行仍是排队，River 把它的任务记成 running，直到救援（`transfer.job_timeout` 加 1 小时，默认 7 小时）。其间它占着队列的一个名额，这本笔记本开始不了新的导入、恢复与撤销（`README.md:193`）。
- 改法：v0.1 只有一个实例（4.8）；`AtStart` 在 River 开始取任务之前跑，这时 River 记为 running 的导入导出任务都是上一个进程留下的。`AtStart` 把这些任务对应的、仍在排队的行也记成失败 `interrupted`，名额立刻空出。
- River 那一边（修订核对第 1 轮 A 的 N12）：平台已有 `Job.DiscardRunning`（`platform/jobs/jobs.go:33-39`、`:134-148`），在 River 启动之前把某一种上一个进程留下的 `running` 任务记成 discarded，搜索的任务在用（`page/adapter/river/search.go:88`）。这是 River 那一边最自然的做法。
- 要注意次序：discard 排在同一个任务自己的 `Start` 之前（`jobs.go:101-106`）。transfer 若照抄，`AtStart` 就看不到"River 记为 running"的那些了。所以要么 `AtStart` 自己先读、再 discard，要么按"discarded 且错误是 left running by a stopped server"来认。P3/S3 的设计选一种。
- 测试：一个整个程序的测试造出"River 取到、没开始"的状态，重启之后行是失败、名额空出。名字照"分份"点名进第七或第八份。
- `README.md:193` 的那一句随之改。

### 4.7 性能与内存实测

照负责人第 2 问（主持者的决定 317，设计审查 A 改）："量到总体设计写的规模；只修超出设计已写预算的（例如页面树超标就改分层加载），其余记成已知限制进 v0.2"。负责人 2026-11-06 另说过："不要把这次当做性能问题继续无限优化了，调整测试用例量级或者时限即可"。

#### 4.7.1 预算与超出的判据

写下了预算的只有下表几行（I2 第 3 节）。没有预算的只量、记下，能推算的推算。

| 编号 | 预算 | 出处 | 量的规模 | 超出的判据 | 判在 | 超出时 |
|---|---|---|---|---|---|---|
| B1 | 搜索：每类查询第一页与"更多"的 p99 ≤ 300 ms；体积 ≤ 正文 3 倍；任务 ≥ 100 页/秒；30 万页从空建 ≤ 1 小时；一页的保存 p99 多 ≤ 2 ms | `M8-history-search/05-P5-search.md:890-902` | 30 万页，两种分布（200 本、5,000 本） | 300 ms 的尾巴与约 5 倍的体积是已接受的超出（`05-P5-search.md:894`、`:899`、`:187`）。只有超出 M8 已记的最大值才算回归：无统计 719 ms，建索引与积压时约 1.2 秒，5,000 本 827 ms（I2-D3）；其余几项照原预算 | 本机的数，对照 M8 记的数看回归 | 回归要修；没回归，维持已知限制 |
| B2 | 解析预算 8 MiB，最坏约 2.4 GB 堆；取不到等 `parse_max_wait` 2 秒后 503 | `server/configs/config.yaml:95-98`，`v0.1-design.md:1300` | k = 2、4、8、16 个并发的 5 MiB 阅读与保存，普通与病态两种正文 | Go 的堆峰值超过 2.4 GB 加空闲时的基线；或没等满 `parse_max_wait` 就 503 | 本机的数（内存不随 CPU 变，不乘） | 修 |
| B3 | 大请求体整个服务两个名额、每凭证一个、等 2 秒；两个合起来约 1 GB | `bootstrap/mcp.go:40-41`，`README.md:627` | 两个名额加排队的，几种形状 | 内存超过约 1 GB 加基线 | 本机的数（不乘） | 修 |
| B4 | 每个请求 15 秒（`server.request_timeout`） | `server/configs/config.yaml:17` | 正常形状：导入上限的 6 万个节点（`config.yaml:153`）、每页几十条链接 | 正常形状下删除、移动、改名到了期限（I2-D6） | 有 CI 上的实数时由它定；没有时本机 × 4.6（推断） | 修（照用户碰到的错误）；病态形状（20 万个节点的子树、400 页 × 2,000 条链接）记成已知限制 |
| B5 | 满负载下一个单元 ≤ 5 秒（`MaxUnitNodes` 1 万、`MaxUnitHistory` 40 万、`MaxRevertSteps` 1,500、`MaxJobItems` 6 万） | `v0.1-design.md:1171`；`page/domain/trash.go:57`、`:66`，`revert.go:58`，`jobs.go:29` | 各界线处 | 只用 M8 写下的规则：参照机（本机）上、满负载、两轮的较大中位数超过 5 秒。5 秒是 15 秒期限的三分之一，余量已含慢的主机与解析（`M8-history-search/02-P2-trash.md:470`），不再乘。最低配置上的硬期限另核：CI 守卫在 EPYC 7763 上的实数加 `parse_max_wait`（2 秒）超过 15 秒；硬期限只对请求一路的守卫核（界线处的恢复、撤销一次删除、撤销很多最上层），`MaxJobItems` 是后台单元、没有请求期限，只用规则一层（修订核对第 2 轮 A 的 N15） | 规则在本机；硬期限用 7763 上的实数（`test-go-guards` 带 `-v` 的那几行），不乘 | 不自动降常量：照移交 M4-performance 第 14 条的两种做法（降常量，或写进部署要求）列给主持者与负责人定（4.7.5） |
| B6 | lint 名额与期限 `min(10 秒, request_timeout/2)`，生产 7.5 秒 | `bootstrap/wire.go:263-264`，`M10-llm-wiki/00-M10-design.md:295` | 10 万页（设计规模）；30 万页只量 | 10 万页上每条规则与摘要过 7.5 秒 | 有 CI 上的实数时由它定；没有时本机 × 2.33（推断） | 修 |
| B7 | 升级的发起与批准在请求期限的 3/4 之内，生产 11.25 秒 | `bootstrap/upgrade_start_cost_test.go:31-40`（M11 第 232 条） | 10 万页；30 万页只量 | 10 万页上过 11.25 秒 | 有 CI 上的实数时由它定（守卫在 7763 上记下的）；没有时本机 × 2.33（推断） | 修 |
| B8 | 导入导出一个任务 ≤ `transfer.job_timeout` 6 小时 | `config.yaml:144` | 导入的上限：5 万个条目、512 MiB 的 zip、4 GiB 解压（`config.yaml:151-157`）；几 GB 的导出 | 过 6 小时 | 本机 × 4.6（推断：移交与 I2 里没有导入导出的比；6 小时之下乘多少都不改结论） | 修 |
| B9 | 编辑器的段落、标题每次解析至多约 80 ms | `v0.1-design.md:1227` | Chromium 里的最坏形状 | 过 80 ms | 本机的 Chromium（在用户的机器上跑，不乘） | 修 |
| B10 | 日志读的守卫 200 ms；`index` 的守卫 3.5 秒 | `v0.1-design.md:1275`；移交 M4-performance 第 18、20 条 | `index` 在 10 万页；按页的日志 10⁵、10⁶ 个变更集只量 | 两个都不是产品的预算，是 CI 守卫的界（`index` 的是 `linking/adapter/postgres/index_cost_test.go:37` 的 `indexCallBound`，13.4 第 8 条的口径）。只量；`index` 乘了之后过 15 秒的请求期限才算超出 | `index`：有 CI 上的实数时由它定，没有时本机 × 2.33（推断），对 15 秒 | `index` 过 3.5 秒照 13.4 第 8 条调守卫；过期限照 B4 修。按页的日志约 13–25 万个变更集时过 200 ms，记成已知限制，迁移进 v0.2（I2-D7） |
| B11 | lint 十万页摘要 TotalAlloc 的"初始 250 MiB" | `v0.1-design.md:1275` | — | — | — | 不是产品的预算：它被守卫的 1,571 MiB 取代，358 MiB 记下，不修（I2-D8） |
| — | 页面树 | 第 14 节"上万页"（`v0.1-design.md:1316`），没有数 | 见 4.7.2 | 见 4.7.2 | 本机的 Chromium | 见 4.7.2 |

- **判在哪台机器上**（设计审查 A 的 M7，修订核对第 1 轮 A 的 I4）：同一个数，换个判法结论相反：本机 7 秒的改名，在慢 4.6 倍的主机上是 32 秒，已经过了 15 秒，是用户碰到的 500。所以逐行写明：
  - "本机的数"：内存、浏览器、只看回归的 B1，与 B5 的规则。
  - 有硬期限的 B4、B6、B7、B8、B10 的 `index`，与 B5 的硬期限：**有 CI（EPYC 7763）上守卫记下的实数时，由实数定结论；没有时才乘倍数**，写明"推断"。实数要与预算同一规模：规模不同时仍是推断。B7 的守卫量的是 5 万页（`bootstrap/upgrade_start_cost_test.go:41-44`），注释里 CI 上 10 万页的数是乘出来的，所以 B7 在 10 万页上只能乘 2.33、写"推断"；B6 有 10 万页 lint 摘要的守卫，同规模（修订核对第 2 轮 A 的 N13）。
  - 倍数是移交里记下的 CI 对当时开发机之比的最大值：lint、`index` 与升级记的是 1.16–2.33 倍，取 2.33；回收站、撤销与后台记的是 2.1–4.6 倍，取 4.6（I2 第 1.1 节）。它的分母是当时更快的开发机（M5 Max），其中还有 CI 的摆动；拿它乘现在更慢的本机，会多算一层，所以只在没有实数时用。
  - 部署文档的最低配置是 EPYC 7763 的 4 核一类，那一档的数用它自己的实测（4.8）。
- **B5 只用 M8 写下的规则判**（修订核对第 1 轮 A 的 I4）：第一次修订加了"本机 × 4.6 ≤ 15 秒"一层，那是在设计已写的预算之上另立一个更严的，余量算了两遍，与负责人第 2 问"只修超出设计已写预算的"不合；它又比 5 秒严，规则一层成了摆设，"超出时"的做法也对不上。去掉。界线处的单元在 7763 上有守卫的实数（例如 `TestARevertAtTheUnitsBoundTakesItsTime` 记过 12.3 秒，移交 M4-performance 第 15 条），最低配置上的硬期限用它判。
- B4、B5 是硬界与定界线的规则，不是体验的目标：过了 15 秒是 500 或 503，什么也不写（I2 第 3 节）。

**只量、不判的**（移交的条目，归类照 I2 第 5 节）：M4-performance 第 1、5–8、10–13、15 的日志、16 的整个任务、18 的多数分项、19 的 30 万页、20、21 的 30 万页，与第 22 条之后的补记（第 7 节"移交的落实"）；M5-performance 第 3 条；M7-transfer 第 4、6 条；M8-history 第 2 条（版本的增长）；13.4 第 9、12 条的旧版本下的通用计划。它们进已知限制、部署文档的数与 v0.2 的移交。

**量的范围缩小**（设计审查 A 的过度设计 3）：先做有预算的 B1–B10 与部署文档要的数（内存、磁盘、WAL、硬件下限）。只量的各项，能用 I2 第 7 节已有的数推的就推，记录写明"推算"与依据的数。推不出、又不判任何预算的（事件流同时重连、按凭证的并发上传、版本的增长），不写专门的驱动或生成器：版本的增长照每次保存的版本行的大小推算；另两项写明不量与理由。

#### 4.7.2 页面树的界线（量之前写死）

主持者的决定 305：第 14 节的"上万页"定为 1 万页。下面的界线是设计者提的（主持者的决定 329，设计审查 A 改）；P4/S2 的设计照抄，量之前不再改。

- **规模与形状**：1 万页，三种形状：
  - 平：1 万页都在根下；
  - 深：10 个根、各 10 个文件夹、各 100 页，另有一个 1,000 个孩子的文件夹；
  - 带附件：深的形状另加一成附件（1,000 个）。
- **环境**：本机，Playwright 自带的 Chromium，生产构建（`make build`），默认视口。不降 CPU，不加 CDP 的网络降速：走回环。
- **三条界线**：
  - **T1 冷打开**：从 `goto` 笔记本首页（已登录、HTTP 缓存清空）到树的答复已到、并且左栏视口里的行已画出，≤ 1 秒。终点用 `PerformanceObserver` 的 element timing，或视口内行数的 `waitForFunction`。
  - **T2 别人改一次树**：另一个账户经 API 新建一页，名字按次序排在最前，落在视口里。从答复到这个标签页画出新的一行 ≤ 1.5 秒（含 refresher 的 500 ms，`web/…/events/handlers.ts:36`）；其间主线程最长的一个长任务 ≤ 200 ms。新页放不进视口时，量"store 已更新加视口重画"。
  - **T3 展开**：展开那个 1,000 个孩子的文件夹，到视口里的孩子都画出 ≤ 500 ms。
- **为什么是这些终点**（设计审查 A 的 M5）：初稿的终点是"根下的行全部进入 DOM""新的一行画出"。"平"的形状没过时，候选的改法是只渲染看得见的行；改了之后，那两个终点永远到不了（新页按次序在一万行的末尾），同一条界线就证明不了改好了。视口里的终点在改法前后都量得了。T3 照同样的道理改为视口里的孩子。
- **为什么 T1 是 1 秒**：照 I2-D1 的推荐。量在回环上，网络不在被量的东西里；初稿放宽到 1.5 秒，理由是"给网络留余地"，与量法不合，而且盖不住真实网络上不压缩的 2.7 MiB（20 Mbps 上单传输约 1.1 秒）。主持者取不加网络降速、用 1 秒在回环上判这一种（设计审查 A 改）。
- **只记的**：线上的字节（代理开压缩与不开各一次）与 JS 堆（CDP 的 `JSHeapUsedSize`）两样；5 万、10 万页同样只记这两样。CPU 降速与持续改树不改任何结论，不量（设计审查 A 的过度设计 4）。
- **怎样判**：每条界线在每种形状上判。两轮，每轮 5 次取中位数，两轮的较大中位数对照界线。
- **结论**：
  - 1 万页三种形状都过：保留整棵树。5 万、10 万页的数记成已知限制，分层加载进 v0.2。
  - 有一条没过：在 v0.1 改，先写设计。改法看没过的是什么：数据一侧（读、比较、索引）多半是分层加载；只有"平"的形状在渲染上没过，多半是只渲染看得见的行（虚拟列表或分批）。改法定了之后，P2/S6 照它排；改完照同样的终点再量。
- **代价最低的缓解**：部署文档的两份代理样例给 JSON 开压缩（I2 第 2.5 节）。它不改结论，只写进文档。
- 依据：I2 第 1.2 节的小样里，数据一侧在 1 万页约 20 ms，渲染没有量（I2 第 2.3 节）；I2-D1 的推荐。

#### 4.7.3 方法：机器、规模、种子、两轮

- **机器**（I2-D2；主持者的决定 318，设计审查 A 改）：以本机为参照（aarch64，8 vCPU，30 GiB，Parallels 虚拟机；I2 第 1.1 节）。CI 的 EPYC 7763 作"慢的主机"的代表：它上面有守卫的实数时用实数，没有时按移交里记的两者之比推算（4.7.1 的"判在"一列）。部署文档写"以下数字在 X 上量得"，最低配置是 7763 的那一档。没有真的生产机，不另租。
- **之前**：`free -h`（交换已满，噪声会大）、`df -h /`（搜索一批约 20 GB，先修剪 Go 的构建缓存，06-environment 第 3 节）；其间不跑别的构建与测试；全部经 `heavy.sh`。
- **被量的**：生产构建的 `nervewiki serve`（`NWIKI_ENV=prod`，默认配置，另写的除外），PostgreSQL 18.7 的容器，建库参数同测试（builtin `C.UTF-8`、LZ4）。
- **种子**：每批一个固定的种子，写进记录；生成器同一个种子出同样的数据。
- **两轮**：每批两轮，各自重启服务进程与 PostgreSQL 的容器。时延报两轮的 p50、p99，对照预算取较差的一轮；时长取两轮的较大中位数；内存取两轮的较大峰值。
- **内存**（I2-D14）：外部每 100 ms 读进程的 `VmRSS`、`VmHWM`，与容器的 cgroup `memory.current`（取最大）；容器每轮重启时也读 `memory.peak`。`memory.peak` 从容器起来就一直累积，清零要往里写、要 root（设计审查 A 的 N6）。Go 的堆用 `GODEBUG=gctrace=1`。产品里不加 pprof 或 metrics 端点。
- **浏览器**：Playwright 与 CDP（`Performance.getMetrics`、`PerformanceObserver` 的 `longtask` 与 element timing）。
- **批次**（I2 第 6 节）：A 页面树与浏览器各项、B 搜索、C lint 与 `index` 与升级、D 回收站与撤销与后台、E 并发与内存、F GB 级的导入导出、G 版本的增长（推算）、H 迁移在大库上的停机。缩小之后的机器时间少于初稿估的 10–16 小时，P4/S3 的设计重估。H 只在负责人说有 v0.1.0 之前的实例要升级时才量（4.8）。
- **10 万页的树的重读**（设计审查 A 的 Q1）：批次 E 量一项，只记。10 万页的 LLM Wiki，agent 每秒建一页、60 秒，k = 1、4、8 个开着这本笔记本的标签页；量服务进程的 CPU 与出口字节，写进部署文档。每个标签页每 0.5 秒整棵重读一次（I2 第 2.3 节），碍着的是同一实例上的别人，不只是那个标签页慢。
- **CI 不跑**（I2-D5）：CI 不加 10 万、30 万页的守卫。只有修了东西的 Step 照负责人 2026-11-06 的话加缩小规模的守卫。

#### 4.7.4 设施进仓库

照 I2-D4（主持者的决定 319，设计审查 A 改）。可复现、可审查；"Codex 的证据不在 git 里"的教训。只写有预算的几项与部署文档要的设施。

- **Go 的生成器与基准**：在组合根写 `_test.go`，没有环境变量就跳过，同 `BenchmarkSearch`（`bootstrap/search_bench_test.go:66-69`）：
  - 合成的大笔记本：1 万、5 万、10 万、30 万个节点；链接、附件、历史可调；
  - 合成的搜索语料：写成 `BenchmarkSearch` 读的三个 TSV，200 本与 5,000 本两种分布；词表取仓库里的 Markdown（中文的设计文档与英文的代码注释），按 Zipf 抽样，不下载语料（`04-owner-decisions-and-preferences.md` 第 4 节）；
  - GB 级的 vault（批次 F 要）。版本的增长不写生成器（4.7.1）。
- **规模开关**：现有的守卫（升级、lint 等）加一个只在手跑时用的环境变量（例如 `NW_SCALE_PAGES`），默认仍是今天的常量（`upgrade_start_cost_test.go:41-44` 等）。
- **外部的驱动与采样**：放在 `server/tools/perf/`，只经 HTTP 打一个在跑的 `serve`。只做有预算的几项要的：k 个 5 MiB 的阅读与保存（B2）、MCP 的大请求体（B3）、N 个编辑者自动保存加树写（B4、B5 的满负载与部署文档的内存）、k 个标签页的树重读（4.7.3）。事件流同时重连与并发上传不写驱动（4.7.1）。采样的脚本同目录。
- **浏览器的计时脚本**：`e2e/perf/` 与它自己的 Playwright 配置，不进 `make e2e`；tsc、knip、lint 照常看它们。`knip.jsonc` 今天没有 e2e 的入口配置，P4/S2 给 `e2e/perf/` 加入口（审查 C 未核实的怀疑 U4）。
- 全部合并之后，CI 只多编译，不多跑。

#### 4.7.5 修与不修

- 超出 4.7.1 的判据的，在 v0.1 修，各开一个条件的 Step（P4/S4 起），先写设计，带缩小规模的守卫。负责人可以改判为已知限制。
- B5 的规则一层在本机没过时，不自动降常量：照移交 M4-performance 第 14 条的两种做法，降 `MaxUnitNodes`、`MaxUnitHistory`、`MaxRevertSteps` 等常量，或写进部署要求，列给主持者与负责人定；"负责人可以改判"用在这一格。本机比 M5 Max 慢，界线处的撤销单次记过约 6.2 秒（移交第 15 条，哪台机器没写明），没过的可能不小（修订核对第 1 轮 A 的 I4）。
- 守卫的规模与时限照负责人 2026-11-06 的话调，不为它优化。`index` 的守卫照 B10 调，不修。
- 只量的，写进 P4 的记录、已知限制与 v0.2 的移交。
- 已经知道的超出项（搜索的尾巴、日志的 13–25 万个变更集、`DeleteNode` 与单个 `move` 的病态形状、lint 的 TotalAlloc）照 4.7.1 的处置，不重开。

### 4.8 部署与备份文档

照 I1-D13、D15–D18（主持者的决定 320，设计审查 B、C 改）。

- **`deploy/README.md`** 作运维手册，目录：
  1. **要求**：
     - 硬件分两档，默认配置与小机器（`page.parse_budget_bytes` 5 MiB 加 `GOMEMLIMIT`），数取自 P4。最低配置是 EPYC 7763 的 4 核一类，那一档的数用 CI 守卫在那里记下的实数（4.7.1）；写明最大合法的 `batch`、回收站与撤销在慢的 4 核上离 15 秒的期限不远（移交 M4-performance 第 22 条之后的补记 `:490`、`:492`）。
     - PostgreSQL 18.7 或更新：18.7 的 GIN 修复正对着 `page_search`，还有一批 CVE（设计审查 B 的 N7）。以 builtin provider 的 `C.UTF-8` 建库、带 LZ4（`platform/postgres/check.go:53-66` 不满足就拒绝启动），托管的 PostgreSQL 要能这样建。
     - 磁盘：正文约 5 倍给搜索、版本的增长、导出与导入的临时文件、WAL。
     - v0.1 只支持单实例。
  2. **安装**：拉镜像并校验签名；建库；两个角色与 `deploy/runtime-grants.sql`；签名私钥；配置（`NWIKI_ENV=prod`、`server.public_url`、`server.trusted_proxies`）；`migrate up`；第一个管理员（`nervewiki users`，链到 README 的命令）；`serve`；`deploy/compose.yaml` 样例。
  3. **反向代理与 HTTPS**：nginx 与 Caddy 的整站样例，从根 README 移过来（`README.md:554-616`）；JSON 开压缩；几个 GB 的导出下载的超时与缓冲（移交 M7-transfer 第 3 条）；大的 MCP 请求体与正文写较多时推荐 nginx，经 Caddy 时早答的拒绝成 502 的代价写明（I1-D15、I2-D13；服务端的改法进 v0.2）。
  4. **备份与恢复**：命令（`pg_dump -Fc`、`pg_dumpall --roles-only`、先库后 `blobs/`）；恢复到 builtin 的库；恢复之后先 `ANALYZE` 再开放服务；私钥与配置也要备份；一次演练的步骤。
  5. **升级与回退**：通用的升级（拉新镜像并校验、`docker stop -t 40`、备份、`migrate up`、补授权、有说明时 `reindex`、启动、看 `/readyz` 与版本号）；`nervewiki reindex` 在 30 万页约 26 分钟、约 290 GB 的 WAL，之后表约是原来的 1.7 倍；回退靠恢复备份，`migrate down` 不保证数据；PostgreSQL 的小版本（18.7 的 GIN 修复：怀疑损坏时 REINDEX；18.7 说明里 TOAST 索引的 REINDEX 提示）与大版本（`pg_upgrade` 或 dump/restore、builtin locale、复核"一个事务里几条 `NOTIFY` 只取一次全局锁"，移交 M5-performance 第 5 条）。
  6. **运行与监控**：探针；日志；磁盘与 507；停机的次序与 `docker stop -t 40`；`server.shutdown_timeout` 20 秒短于一次 `/mcp` 调用（至多约 45 秒），停机时还在传大请求体的调用被切断、整个回滚，要它们做完就把两者一起调长（移交 M7-transfer 第 6 条）；连接数；`GOMEMLIMIT`；连接池代理（PgBouncer）只能用会话模式（移交 M4-performance 第 10 条）；校时：下载地址的到期按客户端的时钟比较（移交 M7-transfer 第 4 条）。
  7. **容量**：P4 的数（内存、磁盘、WAL、版本的增长的推算、10 万页的树被几个标签页重读时的 CPU 与出口）；搜索的索引约是正文的 5 倍。
  8. **运维相关的已知限制**：单实例；没有总量配额，由管理员监控磁盘；结束的任务行不清理；没有永久删除与它的变通（I3 第 3 节 H4）；`batch` 没有单独的期限，最大合法的 `batch` 在慢的机器上可能超过请求的期限、整个回滚（4.6.4）。初稿的"队列按人不设上限"（T2）与"River 取到之后停掉的任务占名额约 7 小时"（T4）在 v0.1 修了，不再列（4.6.6）。
  - 附录"从 v0.1.0 之前的开发版升级"：只在负责人说有这样的实例时写（第 11 节"需要负责人确认"），照它实际所在的迁移号核对；那时 I2 的批次 H 才量。没有的话，根 README 的六节开发版升级说明（`README.md:526-543`）删掉，它们在 git 的历史里。删之前先把 `README.md:534` 里对新装也成立的数（`reindex` 的时长、WAL 与表的膨胀，索引约是正文的 5 倍）挪进上面的"升级"与"容量"（设计审查 B 的 N6；移交 M4-performance 第 17 条要 WAL 的实测进部署文档）。
- **单实例**（I1-D18）：写明 v0.1 只支持单实例与原因：附件在本地磁盘；限流、大请求体的名额、正在上传的导入只在进程内计数；T4 的启动收拾也假定只有一个实例（4.6.6）。多实例之间的时钟一条（移交 M5-performance 第 2 条）因此不适用。
- **根 README**：
  - "部署"一节（`README.md:497-632`）缩成几行，链到 `deploy/README.md`；
  - 示例从本地的 `nervewiki:0.1.0` 改为 GHCR 的地址与标签；
  - "项目处于 v0.1 开发阶段"（`README.md:9`）随发布改；
  - M10 的两段压缩写法（`README.md:499` 等）改成人读的句子或移走，"M12 实机复核"的说法照 P4 的结论改成已知限制（I1 第 8 节第 5 条）。
- **`deploy/compose.yaml`**（设计审查 B 改）：
  - PostgreSQL（`postgres:18.7-trixie`，builtin locale 的 `POSTGRES_INITDB_ARGS`）、`serve`、卷、`stop_grace_period: 40s`。
  - `migrate` 放在 `profiles: [migrate]` 里，`docker compose up` 不启动它；备份之后显式执行 `docker compose run --rm migrate`。常见的写法是让 `serve` 依赖 `migrate: service_completed_successfully`：改了 tag 再 `up -d`，就在备份之前迁移了，分两个角色时也不补授权，正是 4.10 不打浮动标签想挡的"拉了就起"。
  - 写明样例用一个角色还是两个；两个时 `deploy/runtime-grants.sql` 在哪一步执行。
  - `make deploy-check` 跑 `docker compose -f deploy/compose.yaml config -q`，CI 的 `image` 一份跑它（13.5 第 1 条：新门禁同时进 Makefile 与 CI）。它只查语法，上面那一条由演练核对。
  - 它的 PostgreSQL 镜像与 initdb 参数进 `pgtest/image_test.go` 的守卫（4.3）。
- **演练**：P5/S3 照 `deploy/README.md`，用 `deploy/compose.yaml` 本身在本机做三遍：从零装一遍；改镜像的 tag 升一次级（备份、`run --rm migrate`、启动）；备份再恢复一遍。用本地构建的镜像（`make image` 要在普通的克隆里跑，`Makefile:89`）。包设为公开之后，用 GHCR 的镜像再装一遍（4.11 第 5 步）。

### 4.9 版本号

照 I1-D8（主持者的决定 321，设计审查 B 改）。

- **唯一的注入点**是 ldflags 写 `buildinfo.version`。默认值 `0.1.0-dev` 有四份：`Makefile:13`、`platform/buildinfo/buildinfo.go:9`、`deploy/Dockerfile:29`、`:42`，没有守卫（I1 第 1 节）。
- **守卫**：一个短测试核对四份一致，带反向对照（P5/S1）。不去掉 Dockerfile 的 `ARG` 默认值：直接 `docker build` 的人仍得到一个能用的镜像。
- **MCP 的版本**：`serverInfo.version` 取 `buildinfo.Get().Version`（`bootstrap/mcp.go:90`）。image-smoke 只核对 `nervewiki version`（`deploy/image-smoke.sh:76-78`）与 `/api/v0/instance`（`:131-136`），不碰 `/mcp`；加一个短测试断言 MCP 的 `serverInfo.Version` 等于 `buildinfo.Get().Version`，来源相同，image-smoke 照旧（P5/S1；设计审查 B 的 M9、C 的 M2）。
- **打 tag 时不改版本号**：发布的版本由 tag 经 ldflags 注入（4.10）。
- **发布之后**，`main` 的四份默认值改为 `0.1.1-dev`（负责人第 4 问：验收发现的问题出 v0.1.1）。随之改的：`api/modules/instance.yaml:80` 的示例、`README.md:43` 写的默认值、`instance/domain/info.go:15` 的注释；`api/dist/openapi.yaml:3293` 与 `instance/adapter/http/gen/server.gen.go:63` 经 `make gen` 随之。提交的范围写 `build, api`（设计审查 B 的 N2）。
- **Go 的模块**：`server/go.mod` 是子目录的模块，仓库根的 `v0.1.0` 不给它定版本；`go install` 出来的二进制没有内嵌前端（`webui/dist` 只提交了 `.gitkeep`）。不打 `server/v0.1.0`，发布说明不宣传 `go install`（I1 第 1 节）。
- **接口的版本**：`api/openapi.yaml` 的 `info.version: v0` 是接口的版本，与产品版本无关，不改。

### 4.10 发布流水线

照 I1 第 4 节与 D1–D7、D14（主持者的决定 322，设计审查 B 改）。

- **镜像名与标签**（I1-D1；设计审查 B 的 Q1，主持者定）：`ghcr.io/open-nerve/nervewiki`（仓库名有大写，镜像名必须小写）。只打确切的版本号：`v0.1.0` 打 `0.1.0`，rc 打 `0.1.0-rc.1`。**不打 `0.1`，也不打 `latest`**：升级要先停机、备份、`migrate up`，prod 不自动迁移；浮动的标签会诱使人拉了直接重启，新的二进制碰上没迁移的库，`/readyz` 一直 503。补丁版本同样可能带迁移（v0.1.1 很可能带验收的修复），所以 `0.1` 与 `latest` 一样。部署文档要求生产写确切的版本号，照升级的步骤换版本。
- **触发**（I1-D2）：
  - `release.yml`：`on: push: tags: ['v[0-9]+.[0-9]+.[0-9]+*']` 走发布；分支与 `main` 的推送只走演练（见下）。两种都不加路径条件，写在同一个 `on.push` 里。
  - `ci.yml` 改为 `on.push.branches: ['**']`，tag 不再触发（今天 `on: push` 不带过滤，`.github/workflows/ci.yml:3-5`，推 tag 会多跑一遍完整的 19 项）。
  - 不加 GitHub 的 environment 审批：负责人已要求推 tag 之前问他一次。
- **`ci-wait.sh` 按工作流取**：P1 之前已改好（`b54cd679`，4.2）。读 `release.yml` 的演练时第二个参数写它。
- **闸门**（第一个作业）：
  - tag 名是 semver，`VERSION=${GITHUB_REF_NAME#v}`；
  - tag 指的提交是 `origin/main` 的祖先；
  - 这个 SHA 在 `main` 上 push 的 `ci.yml` 的运行已成功：只查 `ci.yml`（`/actions/workflows/ci.yml/runs?head_sha=$SHA&branch=main&event=push`），看最后一次尝试的结论；`release.yml` 的演练绿了不算（经 `gh api`，要 `actions: read`）；
  - `make vuln` 为绿（npm 一侧照 `--audit-level=high`，4.3）；
  - `CHANGELOG.md` 里有这个版本的一节（rc 取去掉 `-rc.N` 之后那个版本的一节），在构建之前就核对。
  - 闸门写成一个脚本（`deploy/release-gate.sh`），可以在本机与分支上对着假的输入跑。
- **两种架构**（I1-D3）：矩阵 `ubuntu-24.04` 与 `ubuntu-24.04-arm`，各自原生构建。Dockerfile 不必改成交叉编译；smoke 在真的 arm64 上跑；公开仓库的 arm64 runner 免费。每个架构：
  1. 检出，`persist-credentials: false`；断言 `git status --porcelain` 为空：`image-smoke.sh:17-20` 照工作区算期望的 `modified`，脏的工作区也能过，发布的二进制要一定是 `modified=false`（设计审查 B 的 N5）；
  2. `docker/setup-buildx-action`、`docker/login-action`；
  3. `docker/build-push-action`，`context: .`（**路径上下文**：它默认的 Git 上下文会忽略之前对文件的改动，也不带 `.git`，要设 `BUILDKIT_CONTEXT_KEEP_GIT_DIR` 才保留；VCS 信息因此缺失、`image-smoke.sh` 会失败；在演练里核实），按摘要推送、不打 tag，`provenance: false`、`sbom: false`（用 GitHub 的证明，免得两套）；
  4. 以 `deploy/image-smoke.sh ghcr.io/open-nerve/nervewiki@<digest> $VERSION` 拉回刚推的摘要来 smoke。smoke 过的就是推上去的；失败时 GHCR 里只留一个没有 tag 的摘要。
- **合并**：`docker buildx imagetools create` 用两个摘要建出多架构的索引，先只打一个不是版本号的 tag（`sha-<提交>`）。给人用的版本号最后打（见"次序与重跑"）。
- **来源证明与 SBOM**（I1-D4、D5）：`actions/attest`，`subject-name: ghcr.io/open-nerve/nervewiki`、`subject-digest: <索引的摘要>`、`push-to-registry: true`。
  - 镜像的 SBOM 由 syft（`anchore/sbom-action`）从镜像生成，覆盖二进制里的 Go 模块。
  - 前端的生产依赖另出一份：打包进二进制的 npm 包在镜像里看不出来（vite 的产物里没有 `package.json`）。用什么工具，P5/S2 的设计里先做个小实验。
  - 两份都作 attestation，也作 Release 的附件。
- **签名**（I1-D4）：`sigstore/cosign-installer` 加 `cosign sign --yes ghcr.io/open-nerve/nervewiki@<digest>`，keyless。负责人原话列了"签名、SBOM 与来源证明"三样，这样做一一对应。
- **第三方许可证**（I1-D14）：镜像里放一个 `/licenses`（Go 模块与前端生产依赖的许可证文本），同一份作 Release 的附件。理由（推断，非法律意见）：MIT、BSD、Apache-2.0 都要求分发时附上版权声明；SBOM 只列标识，不含文本。怎样收集在 P5/S2 的设计里定，只读本机的模块缓存与 `node_modules`，不往外发。
- **Release**（I1-D6）：用 runner 自带的 `gh`。Release 没有就建（`gh release create "$GITHUB_REF_NAME" --verify-tag --notes-file <文件>`），有就改（`gh release edit`）；`gh release edit` 不换附件，附件用 `gh release upload --clobber` 传，重跑时换掉旧的（修订核对第 1 轮 B 的 N8）。rc 加 `--prerelease`。
  - 说明由脚本 `deploy/release-notes.sh` 拼：取 `CHANGELOG.md` 里这个版本的一节；rc 取去掉 `-rc.N` 之后那个版本的一节，前面加一句"预发布"；后面接工作流生成的一段：索引的摘要与两条校验命令。脚本像闸门脚本一样在本机对着假的输入跑，缺这一节就拒（设计审查 B 的 M6）。
  - 附件：两份 SBOM、许可证。
  - 不附独立的二进制：部署文档只覆盖容器；附二进制要另做签名、证明与运行说明。
  - `v0.1.0` 不标 pre-release。
  - 不用第三方的 release action。
- **次序与重跑**（设计审查 B 的 M7）：
  1. 每个架构构建、smoke、按摘要推送；
  2. 合成索引，打 `sha-<提交>`；
  3. 按索引的摘要做来源证明、SBOM 与签名；
  4. 用单个来源 `imagetools create` 复制出 `0.1.0`（只给一个来源时原样复制，摘要不变）；
  5. 建或改 Release。
  - 这样 `0.1.0` 出现时已经签好、证明好。包已设为公开时，不会有一段公开却没签名的 `0.1.0`。
  - 一时的失败（Sigstore 或 GHCR 一时出错）在同一次运行上"Re-run failed jobs"，不重推 tag。每个作业都能重跑：`imagetools` 得到同一个摘要，证明与签名多一份无害，Release 没有就建、有就改。只有产品的问题才出下一个 rc 或 v0.1.1（4.11）。
- **发布之后的核对**：最后一个作业是矩阵，在 `ubuntu-24.04` 与 `ubuntu-24.04-arm` 上各按 tag 拉索引，跑 `gh attestation verify oci://ghcr.io/open-nerve/nervewiki:<tag> -R open-nerve/NerveWiki` 与 `cosign verify …`（`--certificate-identity` 指 `release.yml` 在这个 tag 上，`--certificate-oidc-issuer=https://token.actions.githubusercontent.com`），再 smoke 一次。初稿让本机核对 arm64：rc 还是私有包时，本机拉取要负责人的令牌，本机也没装 cosign（设计审查 B 的 M8）。本机只在包设为公开之后不登录地拉一次（4.11）。
- **演练**（I1-D7；设计审查 B 的 M4、M5，修订核对第 1 轮 B 的 M14）：
  - 单独的作业，只有 `contents: read`，用 `if` 只在不是 tag 的推送上跑。`jobs.<id>.permissions` 只能写字面值，不能随事件变；与 tag 共用一个作业定义的话，分支推送也拿到能写 GHCR 的令牌，与移交 M0-P6 第 1 条"写权限只在发布的触发条件下授予"相反。
  - 两种原生 runner 各自 `push: false`、`load: true` 构建，对本地的镜像跑 `image-smoke.sh`；核实路径上下文带上了 `.git`、VCS 信息在。
  - 在所有分支与 `main` 的每次推送上都跑，不加路径条件：`ci.yml` 的 `image` 一份只构建 amd64（`ci.yml:310-326`），Dockerfile 的 web 阶段在目标架构上 `pnpm install` 与构建，有按架构选的原生可选依赖（`deploy/Dockerfile:16-18`）。P2–P4 改 `web/`、`server/`、锁文件时，只在 arm64 上坏的构建在合并时就看得到，不等到 rc.1 才发现、又因改了产品代码要再问负责人。
  - 第一次修订写的"`main` 每次跑、分支只在改了工作流或 `deploy/` 时跑"，一个 `on.push` 写不出来：同时有 `branches` 与 `paths` 时两个都满足才跑，`paths` 对 `main` 与分支一样有效（GitHub 的 workflow syntax）。取最简单的一种（修订核对第 1 轮 B 的 M14）。
  - 代价：每次推送（分支与 `main`）多两个并行的作业，推断各 10–15 分钟，与 `ci.yml` 并行，不进组合根；公开仓库不收钱（第 10 节）。
  - P5/S1 加一个读 `release.yml` 的检查：演练作业不许有任何 write，带反向对照。
- **权限**：只用 `GITHUB_TOKEN`，不要别的机密。顶层 `permissions: {}`，各作业按需开：

  | 作业 | 触发 | 权限 |
  |---|---|---|
  | 演练（两种架构的构建与 smoke，不推送） | 分支与 `main` | `contents: read` |
  | 闸门 | tag | `contents: read`、`actions: read` |
  | 每个架构的构建、smoke 与推送 | tag | `contents: read`、`packages: write` |
  | 合并、证明、签名、复制出版本号的 tag | tag | `contents: read`、`packages: write`、`id-token: write`、`attestations: write`、`artifact-metadata: write` |
  | 建或改 Release | tag | `contents: write` |
  | 发布之后的核对（两种架构） | tag | `contents: read`、`packages: read`、`attestations: read` |

- **新加的 action**，按 SHA 引用、注释写 tag（13.5 第 2 条；版本与 SHA 见 I1 第 4 节的表与 `survey-i1/actions-new-shas.txt`）：`docker/setup-buildx-action` v4.4.1、`docker/build-push-action` v7.4.0、`docker/login-action` v4.8.0、`actions/attest` v4.2.2、`sigstore/cosign-installer` v4.1.3、`anchore/sbom-action` v0.24.3、`actions/download-artifact` v8.0.2。实施时再核一次最新版。初稿列的 `docker/metadata-action` 没有哪一步用它（label 由构建参数传，tag 由 `imagetools` 打），删掉（设计审查 B 的过度设计）。第三方的几个要负责人在"允许的 actions"里放行（第 11 节）。
- **actionlint**：进 `make lint`，核对三份工作流。至少 1.7.10：`artifact-metadata` 权限到 1.7.10 才认，`ubuntu-24.04-arm` 到 1.7.7。同 `govulncheck`，用 `go run …@版本`，不进 `server/tools/go.mod`（设计审查 B 的 N4）。
- **镜像的 label**：补 `org.opencontainers.image.revision`、`created`、`description`（今天只有 title、version、source、licenses，`deploy/Dockerfile:43-46`）。由构建参数传入，`make image` 与 `release.yml` 都传。
- **`CHANGELOG.md`**（主持者的决定 323）：仓库根，一个版本一节，中文（同 README）。v0.1.0 一节写：做了什么（按 M 一句）、已知限制（4.13）、人工验收在发布之后（4.12）、两处改了已有键与 id 的事（4.6.3）。日期写在 `v0.1.0` 的 tag 指的那个提交里：rc 来回几天时，收尾时写下的日期会不对（设计审查 B 的 M6）。
- **`SECURITY.md`**：怎样报告漏洞（经 GitHub 的私密漏洞报告，要负责人在仓库设置里开，第 11 节）、支持的版本（0.1.x）。

### 4.11 发布的顺序与对外的步骤

主持者的决定 306、324、332。

| 步 | 做什么 | 谁 | 对外？要负责人点头？ |
|---|---|---|---|
| 1 | M12 收尾审查、修复、合并，`main` 的 CI 绿 | 主持者 | 否（推 `main` 是日常） |
| 2 | 问负责人：推 `v0.1.0-rc.1` 与 `v0.1.0` 两个 tag，一起问（I1-D7）；同时请他做第 11 节列的 GitHub 设置里"推 rc 之前"的几项。他的回答与日期记进收尾记录 | 主持者问 | **要点头** |
| 3 | 推 `v0.1.0-rc.1`：GHCR 里有了镜像（私有）、Sigstore 的公开透明日志里有了签名的记录、一个 pre-release 的 Release | 主持者 | **对外**（第 2 步已点头） |
| 4 | 核对 rc：发布之后的核对作业（amd64 与 arm64 的矩阵）为绿；Release 的说明与附件看一遍 | 主持者 | 否 |
| 5 | 负责人把 GHCR 的包设为公开（**不能撤回**）；之后本机不登录拉一次 rc，照 `deploy/README.md` 用它从零装一遍 | 负责人；主持者核对 | **对外**，负责人自己做 |
| 6 | 在 `m12-release` 上写 `CHANGELOG.md` 的日期，合并，`main` 的 CI 绿；推 `v0.1.0`，同第 4 步核对，本机不登录再拉一次 | 主持者 | **对外**（第 2 步已点头） |
| 7 | 在 `m12-release` 上把 `main` 的默认值改为 `0.1.1-dev`，合并，`main` 的 CI 绿 | 主持者 | 否 |
| 8 | 收尾记录写"发布"一节（负责人对两个 tag 的回答与日期、run 号、摘要、校验的输出、`m12-release` 上的每个改动）；12.6 写"实现完成，待人工验收"；推送；停下汇报 | 主持者 | 否 |

- **第 2 步的点头盖住什么**（主持者的决定 324）：rc 有问题时，修了再推 `rc.2`、`rc.3`。只改发布流水线、部署文档或发布说明的，同一次点头盖住；改了产品代码的，再问一次。收尾合并之后由闸门的 `make vuln` 发现、要升 Go 或依赖的，算改了产品代码；为此往 `audit.ignore` 加的忽略，算改了产品的依赖配置；都再问（4.3）。
- **发布期的改动走 `m12-release` 分支**（主持者的决定 332，设计审查 C 的 M6）：收尾合并之后为发布做的改动（rc 的修复、`CHANGELOG.md` 的日期、`0.1.1-dev`）都在分支 `m12-release` 上做。主持者读 diff、看分支的 CI，`git merge --no-ff` 合并进 `main`，`main` 的 CI 绿之后才推 tag。每一轮从 `main` 开分支，合并之后删。收尾记录的"发布"一节逐个列出。不直接提交到 `main`：`docs/README.md` 的 Git 一节要代码在分支上开发、门禁全绿再合并，这些改动又不在任何收尾审查之内。
- **一时的失败**：在同一次运行上"Re-run failed jobs"，不重推 tag，不算重推（4.10）。
- **rc 不过时**：不删 tag、不删 Release，标明作废，修了推下一个 rc。GHCR 里留着的 rc 镜像不影响 `0.1.0`。
- **`v0.1.0` 推了之后发现产品的问题**：不重推同一个 tag，出 `v0.1.1`。
- **公开的时机**：包第一次推送时默认私有（GitHub 文档）。在 rc 核对过之后、`v0.1.0` 之前设为公开：这样匿名拉取在 rc 上就核对了，`v0.1.0` 的说明里的命令一发出就能用。

### 4.12 人工验收在发布之后

照负责人第 4 问（没有选推荐的那项）。

- **先发 v0.1.0**，之后做人工验收：
  - MCP 真实客户端的清单 A–I、J、K（`docs/v0.1/M9-mcp/manual/P5-mcp-clients-checklist.md`，K 是 M11 加的）；
  - 读屏软件：清单在 P5/S3 新写（`manual/screen-reader-checklist.md`），收 4.5 移过去的八项（负责人 2026-11-14 第 6 问）、M 类里与读屏有关的项与第 39 条；
  - M 类的其余各项（附录 A）。
- **输入法清单**（`docs/v0.1/M4-pages/manual/P6-ime-checklist.md`，M4–M8 的 21 步）：负责人第 4 问没有点名它。主持者的理解是它同样在发布之后（负责人 2026-10-13："IME 清单,我最后 统一验收就可以了"）；列在第 11 节请负责人确认。
- **汇总的入口**：`docs/v0.1/M12-release/manual/README.md` 链到上面每一份，写怎样记结果。
- **发现的问题**出 v0.1.1。v0.1.1 的文档放 `docs/v0.1.1/`，与 `docs/v0.1/` 并列；v0.1.1 的工作由负责人另行批准开工，M12 只写它的移交（主持者的决定 326，设计审查 C 的 Q2；第 7 节"写给后面的 M"）。
- **状态**：M12 收尾之后 12.6 写"实现完成，待人工验收"；M4–M11 的状态不变；验收通过之后一起改为"已完成"。
- **发布说明写明**：人工验收在发布之后；还没有在真实的读屏软件、真实的输入法与真实的 MCP 客户端上逐项核对过。

### 4.13 已知限制与以后的移交

- **已知限制**写进 `CHANGELOG.md` 的 v0.1.0 一节，按主题写十来句，不逐条列（I3 第 2 节）。P5/S3 的记录里有一张逐条对照的表（第 3 节第 10 条）。来源：
  - P2 推后的 C 类（编辑时没有大纲、没有标签总览、相对时间不刷新、健康度不自动刷新、网页上不显示 MCP 地址等）；
  - 4.5 的 H1、24.1（负责人 2026-11-14 第 5 问）；
  - 4.6.6 的 Shift-JIS、Big5、EUC-KR 会被误读，与 CP437；
  - 4.6.4 的 `batch` 没有单独的期限；
  - P4 只量的与超出而经负责人改判的；
  - 运维相关的（4.8 的第 8 项）；
  - e2e 揭出的那一条：续期途中刷新会被登出（4.4）。
- **v0.2 的移交**：M12 收尾时在 `docs/v0.2/handoffs/` 写（`status: open`、`from: M12`、`to: v0.2`），按主题分：
  - `M12-polish.md`：C 类 87 项与 3、12.2、11.6 的其余（附录 A）、D 类其余、Web Locks 的登记（第 7 条）；
  - `M12-performance.md`：只量的各项与它们的数、已知的改法（页面树的分层加载若没做、搜索尾巴的三种改法、按页日志的迁移、共用的参与者界线、MCP 名额的配置项、慢的发送者与 Caddy 早答的服务端改法、附件列表的重读、`batch` 单独的期限）；
  - `M12-history.md`：H1、H3、H4 的功能、版本的保留策略（带 P4 推算的增长）；
  - `M12-transfer.md`：24.1（网页的任务列表在网络结果未知时可能指到别处的任务，负责人 2026-11-14 第 5 问）；T1、T5、T8；CP437 与别的编码的识别、导入的编码选项；
  - `M12-release.md`：Node 26、功能性的大版本依赖、Dependabot（看负责人的决定）、多实例、独立的二进制；续期途中刷新会被登出（重看 M1 第 10 节的取舍，设计审查 C 的 N6）。
- **v0.1.1 的移交**：`docs/v0.1.1/handoffs/M12-after-release.md`：发布之后的人工验收（4.12 的各份清单）与读屏验收要确认的八项；`vuln.yml` 的两件（定时运行的通知发给最后改 cron 的人；公开仓库 60 天没有活动时定时工作流会被停用，4.3）。
- **逐项核对去处**（修订核对第 1 轮 C 的 M9）：上面"已知限制"的每一项都在某一份移交里，P5/S3 的对照表写明是哪一份：H1 在 `M12-history.md`；24.1、T1、T5、T8、CP437 与别的编码在 `M12-transfer.md`；`batch` 的期限与 P4 只量的在 `M12-performance.md`；续期途中刷新被登出、单实例在 `M12-release.md`；C 类在 `M12-polish.md`；H4 的功能在 `M12-history.md`。T2、T4 在 v0.1 修，不进移交。
- `docs/README.md` 的目录结构加 `v0.1.1/` 与 `v0.2/handoffs/` 两行。v0.2 的总体设计开工时先处理它们。

## 5. 接口

M12 不加 REST 操作、MCP 工具与 prompt，也不加配置项。改的是几处行为、几个 Makefile 目标与工作流、发布物。

### 5.1 REST 与 MCP 的行为

| 处 | 改动 | Step |
|---|---|---|
| 节点的标题键（`shared.TitleKey`，三处） | 切罗基文的小写在折叠之后换成大写；只差切罗基文大小写的两个标题从此冲突（`page.name_taken` 照旧） | P3/S1 |
| 阅读视图的标题 id | 含自动链接与原始 HTML 的文字；这类标题的锚点 id 变了 | P3/S1 |
| 标签列表（linking 的 `listTags`） | 不再列 `/`；`api/modules/linking.yaml` 的说明随之 | P3/S1 |
| 显示名与客户端名 | 去掉默认可忽略的字符与空白之后什么也不剩的新的显示名答 `required`，这样的客户端名当作空的；标题与库的 CHECK 不变 | P3/S1 |
| MCP 的 `log` | `path` 省略或 `"/"` 是整本；`""` 答字段错误，文字写"省略它就是整本"；`index` 不变（`parent:"/"` 仍是根）；`api/mcp/tools.json` 的说明随之 | P3/S1 |
| MCP 的工具调用超过请求的期限 | 文字说明期限已到、什么也没有写、`batch` 拆小再试；码照旧是 `internal_error`（或照 P3/S1 的设计） | P3/S1 |
| 日志与变更集的详情（`listChangesets`、`getChangeset`、MCP 的 `log`） | 各自的五条读在一个只读快照里；答复的形状不变 | P3/S1 |
| 事件流（`GET /api/v0/events`） | 一个凭证至多 4 条，超出时以 `reset`（新的原因 `superseded`）结束同一凭证最老的那条；先按凭证判，只有净增一条时才看账户的上限：一个账户至多 64 条，超出答 429 带 `Retry-After`；`api/modules/events.yaml` 的枚举与说明随之 | P3/S1 |
| 撤销（`revertChangeset`、`startRevert`） | 被这个变更集删了、本身是回收站根的节点自成一顶：回滚的变更集与"恢复之后先删里面、再删外面"的变更集照常撤销，不再答 `page.revert_conflict` | P3/S2 |
| 导入的报告 | 按 Info-ZIP 的 Unicode 路径或 GBK 解读的名字另列，带新的原因；两种语言的文字随之 | P3/S3 |
| 开始任务（导出、导入、后台的恢复与撤销） | 一个人排队与运行中的任务到上限（常量，例如 5）时答一个新的码（例如 `transfer.too_many_jobs`，状态码推荐 429，P3/S3 定），两种语言的文字说"你自己已有几个任务在排队或运行"；全局的 503 `server_busy` 照旧；`api/modules/transfer.yaml` 的 `x-problem-codes` 与说明随之 | P3/S3 |
| 服务启动 | 上一个进程在 River 取到之后、开始之前停掉的任务记成失败 `interrupted`，名额空出；README 的那一句随之 | P3/S3 |
| `GET /api/v0/instance` 的 `version` 与 MCP 的 `serverInfo.version` | 发布的镜像是 `0.1.0`；`main` 之后是 `0.1.1-dev` | P5、发布 |

改说明的每一处都 `make gen`、`make gen-check`（05-lessons 第 8 节）。

### 5.2 配置

不加配置项。初稿的 `events.max_streams_per_account` 改为常量（设计审查 A 的过度设计 2）；事件流的两个上限与按人的队列上限都是代码里的常量，写在 README 与契约的说明里。

### 5.3 Makefile、工作流与工具

| 目标 | 做什么 | 在哪里跑 | Step |
|---|---|---|---|
| `make vuln` | Go 一侧 `govulncheck`，只扫 `server/`；npm 一侧 `pnpm audit --prod`，级别可传（默认 `low`）；版本固定 | `vuln.yml`（每天、`workflow_dispatch`、改了它自己或依赖清单的推送）；发布的闸门（npm 照 `high`）；**本机不跑** | P1/S2 |
| `make outdated` | 只读地列出过时的 Go 模块、npm 包、actions 与镜像 | `vuln.yml` 里一个手动触发、改了依赖清单的推送也跑的作业（`continue-on-error`，失败不让运行变红），发布之前手动触发一次；**本机不跑** | P1/S2 |
| `make deploy-check` | `docker compose -f deploy/compose.yaml config -q` | CI 的 `image` 一份 | P5/S3 |
| `make lint` 加 actionlint | 核对三份工作流；至少 1.7.10，`go run …@版本` | 本机与 CI | P5/S1 |
| `release.yml` 的权限检查 | 读工作流，演练作业不许有 write，带反向对照 | `make lint` 的一步 | P5/S1 |
| 性能的目标（名字在 P4 的设计里定） | 跑 4.7 的生成器、驱动与浏览器脚本 | 只在本机手跑 | P4/S1、S2 |

- **工作流**：`ci.yml` 只在分支上跑（4.10）；`release.yml` 是新的，tag 走发布、分支与 `main` 走只读的演练（4.10）；`vuln.yml` 是新的，每天、手动与改了它自己或依赖清单的推送时跑 `make vuln`，手动触发与改了依赖清单的推送时另跑 `make outdated`（4.3）。
- **`ci-wait.sh`**（`docs/v0.1/claude_to_codex/tools/`）：按工作流取运行，默认 `ci.yml`，第二个参数可改（已在 `b54cd679`，4.2）。

### 5.4 发布物

- **镜像**：`ghcr.io/open-nerve/nervewiki:0.1.0`，`linux/amd64` 与 `linux/arm64` 的索引；只有确切的版本号，没有 `0.1` 与 `latest`；label 加 `revision`、`created`、`description`。
- **证明与签名**：GitHub 的来源证明与 SBOM（推进 GHCR）、cosign 的 keyless 签名。校验：
  - `gh attestation verify oci://ghcr.io/open-nerve/nervewiki:0.1.0 -R open-nerve/NerveWiki`
  - `cosign verify ghcr.io/open-nerve/nervewiki:0.1.0 --certificate-identity=https://github.com/open-nerve/NerveWiki/.github/workflows/release.yml@refs/tags/v0.1.0 --certificate-oidc-issuer=https://token.actions.githubusercontent.com`
- **Release**：说明取 `CHANGELOG.md` 的一节，加摘要与两条校验命令；附件是镜像的 SBOM、前端的 SBOM、第三方许可证。
- **仓库里的文件**：`deploy/README.md`、`deploy/compose.yaml`、`deploy/release-gate.sh`、`deploy/release-notes.sh`、`CHANGELOG.md`、`SECURITY.md`、`docs/v0.1/M12-release/manual/` 的两份。

## 6. 从 Nerve 借鉴

无。M12 不读兄弟项目 Nerve（`CLAUDE.md` 的规矩）。发布流水线、部署文档与实测都照本项目自己的：M0/P6 的镜像与 image-smoke、M4–M11 的成本守卫与夹具、README 已有的部署一节。

## 7. Phase 划分

五个 Phase、17 个 Step，另有条件的修复 Step，最后是收尾与发布。

| P | 名称 | Step | 交付 | 验证 |
|---|---|---|---|---|
| — | 设计定稿 | 一个直接进 `main` 的文档提交（heavy.sh 的锁 `8c247237`、`ci-wait.sh` `b54cd679` 已在 `main`） | 00-M12、审查记录、定稿时的总体设计修订、六份移交的"处理进展"（写到 Step） | 修订的复核 |
| P1 | 地基 | S1、S2 | e2e 的 `goto` 与 `reload` 帮手；工具链与安全更新、`vuln.yml` | 帮手的对照测试与门禁；两个守卫；`vuln.yml` 在分支上由 `push` 触发为绿 |
| P2 | 网页的打磨 | S1–S6 | 附录 A 的 A、B 两类，按组件分 | 每项一个会在没修的代码上失败的测试；变异表 |
| P3 | 服务端的正确性与历史 | S1–S3 | 4.6 的各项 | 第 3 节第 4、5 条 |
| P4 | 性能与内存实测 | S1–S3，另有 S4 起的条件修复 | 设施；浏览器与服务端的实测记录；超出的修复 | 4.7.1 有预算的每一行有结论、写明判在哪台机器上；超出的有缩小规模的守卫 |
| P5 | 部署文档与发布流水线 | S1–S3 | 版本号与流水线的构建；推送、签名、证明与 Release；部署文档与发布说明 | 演练两种架构为绿、演练作业只读；闸门与说明脚本的反向对照；用 `compose.yaml` 安装、升级、备份恢复的演练 |
| — | 收尾与发布 | — | 收尾审查；问负责人；`m12-release` 上的发布期改动；rc 与 `v0.1.0`；`0.1.1-dev`；记录 | 第 3 节第 9、10 条 |

### 设计定稿（P1 之前）

1. **heavy.sh 的锁**（4.2）：主持者已单独提交、推上 `main`：`8c247237`。
2. **`ci-wait.sh` 按工作流取**（4.2）：主持者已单独提交、推上 `main`：`b54cd679`。
3. **设计定稿的提交**（主持者的决定 333，设计审查 C 的 Q1）：一个文档提交，写 `docs: the M12 design, its review, and each hand-off item's place in M12 (M12)`，含：
   - `docs/v0.1/M12-release/00-M12-design.md` 与 `reviews/M12-design-review.md`；
   - 六份移交各加"处理进展"（02-workflow 第 1 节第 1 步），写本节"移交的落实"的去处，写到 Step，状态仍是 `open`；
   - 本节"总体设计的修订"标"设计定稿时"的几条（第 1–3、14、16 条）。

   初稿建议的"移交的处理进展在初稿之前单独提交"不要了：处理进展要写到 Step，Step 在审查之后还会变（这次审查就改了几行的去处）。

### P1 地基

| Step | 内容 | 改的部件 | 单独合并为绿 | 组合根与 CI 的用时 |
|---|---|---|---|---|
| S1 e2e 的帮手 | 4.4：夹具里的在途记录与帮手（`goto`、`reload`、显式的逃生口）；`pageWatch` 认出 `/sign-in`；109 个故事文件与 `e2e/fixtures/wiki-health.ts:157` 换上帮手；门禁（含 `e2e/fixtures/`，数逃生口）与它的反向对照；延后续期的对照测试；等这个文档的第一次续期与续期的锁；`global-setup.ts` 的一行日志（第 50 条）；总体设计 13.4 第 3 条 | e2e、tools、build（`Makefile`、`knip.jsonc`）、docs | 能：只改 e2e，CI 的 e2e 一份证明；逃生口只有 A15 的两格 | 组合根不变。e2e 作业预计多 0–30 秒（推断：多数 `goto` 之前没有在途的读；有任务时每秒轮询的设置页可能要多等几次，没有量；今天 6:52–9:23，时限 20 分钟） |
| S2 工具链与安全更新 | 4.3：Go 1.27.4（两份 `go.mod`；Dockerfile 的 tag、摘要与构建时的断言；README；文档里的五处：总体设计 13.1 第 32 条、06-environment 两处、05-lessons、03-project-background）；PostgreSQL 18.7（五处、守卫加 README）；三个镜像的摘要；两个 action；`make vuln`（Go 只扫 `server/`，npm 的 `pnpm audit --prod`）、新的 `vuln.yml` 与它报出的模块；`make outdated`；Dockerfile 的 Go 版本的守卫；两处注释的核对；总体设计 7.1 与 13.5 第 2 条 | server、deploy、e2e、CI、Makefile、README、docs | 能：一个分支、一次完整的 CI，`vuln.yml` 在同一个分支上由 `push` 触发跑一次；PostgreSQL 的五处由守卫一起改，漏一处就红 | 组合根各份照新的工具链与数据库重量：合并之后两次 `main`，过 450 秒照 06-environment 第 4 节挪份。`ci.yml` 不多跑 `make vuln`。`image` 一份要拉新的基础镜像 |

- S1 先做：S2 要一次干净的完整 CI，不该被 `goto` 的偶发打红。
- **每个 Step 合并之前**（02-workflow 第 3.6 节）：主持者看一眼 `vuln.yml` 最近一次的结论（`gh run list --workflow vuln.yml --branch main --limit 1`）；定时运行的通知不发给主持者（4.3）。
- **`vuln.yml` 在 M12 期间红了**（4.3）：npm 一侧修不了或与我们无关的写进 `audit.ignore`，其余主持者开旁支 `m12-toolchain-<版本>`，照 S2 的做法升，提交写当时所在的 Phase，照常审查与合并。收尾合并之后由闸门发现的，照决定 324 再问负责人。

### P2 网页的打磨

每个 Step 的逐项清单在附录 A。每个 Step 自带它加的文字的两种语言（13.2 第 4 条）、假服务端的字段与 knip 的干净。

| Step | 内容 | 改的部件 | 单独合并为绿 | 组合根与 CI 的用时 |
|---|---|---|---|---|
| S1 焦点与共用组件 | `ConfirmDialog` 发送中改用 `aria-disabled`，被拒之后焦点还给确认按钮或说明（17 个文件、25 处 `<ConfirmDialog`）；`useForm` 只有横幅的拒绝时焦点给横幅（`web/…/components/form-field.tsx:89-97`）；`button.tsx` 加 `aria-disabled:` 的样式（23.17、28.2）；到达的焦点（12.3、23.14、26.4、28.4、28.5）；其余的焦点项；撤销对话框的两句说明进 `aria-describedby`（24.3、49）；第 7 条"仍要退出吗"；总体设计 13.2 第 12、26、1 条改写 | web、e2e、docs | 能 | 组合根不变；vitest 多几十个测试；e2e 改到的故事照需要进 `threeBrowsers`，推断多 1 分钟以内 |
| S2 文字 | 单复数（35.1、41.x，每个单数一个 `*One` 键）；根下的叫法（22.11、23.3）；撤销的用词（23.1、23.4、23.11、24.7）；其余误导的文字；第 40 条两份 `AGENTS` 模板；H4 的界面一句（不写版本号） | web、server（`wiki/domain/template-*-agents.md`） | 能 | 组合根不变（模板的测试在 wiki 模块里）；vitest 多一些 |
| S3 375 px、`bdi` 与对比度 | flex 项补 `min-w-0` 或 `[overflow-wrap:anywhere]`（26.17、32.x、35.8、38.10–38.12）；从右到左的名字进 `bdi`（26.12、32.4）；焦点环与补全说明、暗色高亮的对比度（12.4、26.9）；32.3 MC3 的 375 检查；33.1 在 webkit 上排查 | web、e2e | 能 | 组合根不变；e2e 加几处 375 px 的检查，33.1 的排查不进套件 |
| S4 阅读视图（**先写 spec**） | `reading-view.tsx:133-232` 那个 layout effect 的重写：更新不整段重写 `innerHTML`、不失焦（3、8.2、12.2、29）；每小时的整篇替换与全屏退出（17.1、17.8）；`editable` 与 `role` 不进那个 effect（48，改法见移交第 48 条）；后退前进到锚点（8.1）；mermaid 的 id（11.1）；失去角色后的勾选（6.2）；换语言时的失焦（11.6）；按职责拆开（14d） | web、e2e | 能；要对照 M6 以来依赖 `editable` 的增强逐一核对（移交第 48 条） | 组合根不变；e2e 加阅读视图的焦点与滚动检查，三种引擎（05-lessons 第 6 节） |
| S5 编辑器、附件、上传、粘贴与导入导出 | 附件一节、阅读视图里的附件、编辑器的上传与粘贴、导入对话框与任务列表的 A、B（附录 A）；上传的超时取多长在 S5 的设计里定（16.3） | web、e2e | 能；可按"附件""编辑器""导入导出"分三组提交 | 组合根不变 |
| S6 版本、回收站、历史、目录、后台任务与搜索的网页 | 22.14 与 23.10 的"在途的写进 store"；12.12 的 `Math.max(...)` 与 `push(...)` 的参数上限（`web/…/stores/page-tree.ts:116`、`:124`）；27.2；删除与新建之后的导航（2）；其余 A、B；24.8、24.10（TB2 的偶发）、26.15 | web、e2e | 能；**排在 P4/S2 的页面树结论之后**；可按版本与回收站、历史与目录、搜索分组 | 组合根不变 |

`en.ts`、`zh-CN.ts` 几乎每个 Step 都改：按序合并，后合并的先 `git merge main` 解决冲突（02-workflow 第 3.6 节）。

### P3 服务端的正确性与历史

| Step | 内容 | 改的部件 | 单独合并为绿 | 组合根与 CI 的用时 |
|---|---|---|---|---|
| S1 小的正确性 | 25.1（三处折叠，删 `searchFold` 的映射）、10.2、10.1、30.2（整个名字的判断）、`log` 的 `path` 约定（30.4、30.5）（4.6.3、4.6.4）；MCP 调用超时的文字（4.6.4）；H5 的快照，日志与变更集的详情（4.6.2）；每个凭证与账户的流数（4.6.5）；13.1 第 28、30 条 | server、api（说明）、web（若有新的文字；事件流的 429） | 能 | 单元与适配器的测试为主；流数的整个程序的测试点名进第七或第八份，名字避开第二份的模式（"分份"），推断多 5 秒以内 |
| S2 第 302 条（**先写 spec**） | 4.6.1：`readReverted`/`topOf` 把本身是回收站根、被这个变更集删的节点当作它自己的顶，几处传 `Nodes`；后台撤销的 `revertSteps`/`needed`；两个性质测试扩开；第三种形状的一格；nested 一格翻过来（两本笔记本，第七、第八份的模式一并改）；总体设计 5.8 第 6 步与 13.1 第 2 条 | server（page 的 domain 与 app） | 能；nested 一格在同一个提交里翻过来 | 两个性质测试在 page 的 app。`make test-go-race` 对组合根之外的包不带 `-short`（`Makefile:179`），代价在 `race` 一份：各多至多约 15 秒，以同类为参照（`TestTheJobsStepsReplayToStar` 竞态下 13.99 秒、`TestTheRollbacksStepsReplayToTheBaseline` 1.85 秒）；`-short` 的次数照 `TestTheJobsStepsReplayToStar` 的 2,000 对 400（设计审查 A 的 M8）。组合根只改第八份已有的测试，推断不变 |
| S3 导入导出的任务 | 4.6.6：名称的编码（先读 0x7075，再 GBK 回退，整包的判断）、报告的新原因与两种语言的文字、差分的测试；按人的队列上限（T2）；启动时收拾上一个进程取到而没开始的任务（T4）；`README.md:193` 那一句 | server（transfer）、api、web | 能 | 适配器与单元的测试；T4 的整个程序的测试点名进第七或第八份 |

P3 的三个 Step 与 P2 除翻译文件外不重叠，可以并行；翻译文件照上面按序合并。S1 与 S2 都改 page 模块，S2 在 S1 之后开，或在合并时 `git merge main`。S3 只改 transfer，与 S1、S2 不重叠。

### P4 性能与内存实测

| Step | 内容 | 改的部件 | 单独合并为绿 | 组合根与 CI 的用时 |
|---|---|---|---|---|
| S1 设施（Go） | 4.7.4：合成的大笔记本、搜索语料（三个 TSV）、GB 级的 vault；现有守卫的规模开关；`server/tools/perf/` 的驱动（有预算的几项要的）与采样 | server（`_test.go` 与 `server/tools/perf/`）、Makefile | 能：没有环境变量就跳过，CI 只多编译；lint 要过 | 组合根多编译，不多跑；推断各份不变 |
| S2 浏览器侧的设施与实测（**页面树先**） | `e2e/perf/` 的脚本与配置，`knip.jsonc` 给它加入口；先量页面树（4.7.2，量之前把界线写进 P4 的设计），结论先报主持者；再量 M4-performance 第 7、9、11、12 条与 M7-transfer 第 4 条；记录 | e2e（perf）、knip、docs | 能：脚本不进 `make e2e`；记录只改文档 | 不变 |
| S3 服务端实测（P3 合并之后） | 4.7.3 的批次 B–G（H 看负责人的回答）；有预算的每一行对照 4.7.1，写明判在哪台机器上；部署文档要的数（含硬件下限、10 万页的树被几个标签页重读）；只量的推算；已知限制与 v0.2 的清单；总体设计第 14 节与 13.4 第 8、9 条写着 M12 的各句照结论改 | docs | 能：只改文档 | 不变；本机的机器时间少于初稿估的 10–16 小时（推断，P4/S3 的设计重估），独占 heavy 的锁 |
| S4 起：条件的修复 | 只在超出 4.7.1 或 4.7.2 的判据时开，各自先写设计、带缩小规模的守卫。可能的：页面树的改法、参与者的共用界线（B4）、附件列表的重读（I2-D10 判为 A 时）、搜索的回归、B5 的规则一层没过时经主持者与负责人定下的做法（降常量，或写进部署要求） | 看改法 | 每个都要能 | 守卫照 13.4 第 8 条进成本或守卫的一份 |

S2、S3 的"实测"Step 照常有分支、审查与合并：审查者核对方法、种子与脚本，抽一两项在自己的副本上重跑，不重跑全部。S2 与 S3 可以写在同一时段，跑要串行（heavy 的锁）。

### P5 部署文档与发布流水线

| Step | 内容 | 改的部件 | 单独合并为绿 | 组合根与 CI 的用时 |
|---|---|---|---|---|
| S1 版本号与流水线的构建部分 | 四份默认值的守卫与 MCP 版本的短测试（4.9）；Dockerfile 的三个 label 与 `make image` 传它们；`ci.yml` 只在分支上跑；`release.yml` 的闸门（`deploy/release-gate.sh`，只认 `ci.yml`，带"`ci.yml` 红、`release.yml` 绿"被拒的反向对照，核对 `CHANGELOG.md` 的一节）、说明脚本（`deploy/release-notes.sh`）、两种架构的构建与 smoke；只读的演练作业（每次推送）与读 `release.yml` 的权限检查；actionlint 进 `make lint` | deploy、CI、Makefile、server（守卫在 buildinfo 与 bootstrap 的测试） | 能：演练作业在分支上证明两种架构为绿；不推送 | 组合根不变；每次推送（分支与 `main`）多两个并行的演练作业（推断各 10–15 分钟，与 `ci.yml` 并行） |
| S2 推送、签名、证明、SBOM、许可证与 Release | 合成索引（先打 `sha-<提交>`）；`actions/attest`；cosign；镜像与前端两份 SBOM；`/licenses`；最后复制出版本号的 tag；Release 没有就建、有就改；发布之后的核对作业（两种架构的矩阵） | CI、deploy | 合并时为绿（分支上不触发这部分）。这条路径**只有推 tag 才证明得了**，所以要 4.11 的 rc | 不变 |
| S3 部署文档与发布说明 | `deploy/README.md`（含硬件下限、从 `README.md:534` 挪来的数）；`deploy/compose.yaml`（`migrate` 在 profile 里）与 `make deploy-check`；PostgreSQL 镜像的守卫把 README 那一项换成 `deploy/README.md`、加 `compose.yaml`；根 README 缩写与改地址；第 7 节"移交的落实"的每条部署项；开发版的附录（看负责人的回答）；`CHANGELOG.md` 的 v0.1.0 一节（已知限制取自 P2–P4，逐条对照 4.13）；`SECURITY.md`；`manual/` 的读屏清单与汇总入口；用 `compose.yaml` 安装、升级、备份恢复的演练；总体设计第 11 节 | docs、deploy、README、CI、server（PostgreSQL 镜像的守卫） | 能：只改文档、一个检查与一个守卫 | `image` 一份多 `make deploy-check`，几秒 |

P5/S1 在 P1 之后就能开，与 P2–P4 并行。S3 等 P4/S3 的数。

### 收尾与发布

照 02-workflow 第 4 节与 4.11：

1. 三位审查者并行：A 服务端（跨 Phase 的一致性、302 的修法与撤销的其余路径、任务的入口与启动）；B 网页与 e2e（打磨的回归、三种引擎的焦点）；C 完成标准、文档、移交的落实与发布流水线（对照第 3 节逐条，对照代码核对每份移交）。
2. 修复在分支 `m12-closeout`，提交写 `(M12 closeout)`；核对一轮一轮做到干净；合并，`main` 的 CI 绿。
3. 处理移交：六份改为 `done`；写 `docs/v0.2/handoffs/` 的五份与 `docs/v0.1.1/handoffs/` 的一份（4.13）。
4. 4.11 的第 2–7 步；发布期的改动在 `m12-release` 分支上（主持者的决定 332）。
5. 收尾记录 `reviews/M12-closeout-review.md`，加"发布"一节：负责人对两个 tag 的回答与日期；`m12-release` 上的每个改动；run 号、摘要与校验的输出；P1/S1 之后 `main` 的 e2e 红过的每一次与总次数（主持者的决定 334）。12.6 写"实现完成，待人工验收"；推送；停下，给负责人写中文汇报（02-workflow 第 6 节）。

### 次序与并行

逐条列出依赖（初稿的图有两处歧义，设计审查 C 的 N4）：

- P1/S1 在设计定稿之后开；P1/S2 在 P1/S1 合并之后开。
- P1/S2 合并之后，可以开 P2/S1–S5、P3/S1、P4/S1、P5/S1。
- P2/S1–S5 彼此按序合并（翻译文件）。
- P2/S6 等 P4/S2 的页面树结论；结论要改树时，等改法的 Step。
- P3/S2 在 P3/S1 之后开，或在合并时 `git merge main`；P3/S3 不等 S1、S2。
- P4/S2 在 P4/S1 之后；P4/S3 在 P3 全部合并、P4/S2 之后；P4/S4 起的条件修复在它们的结论之后。
- P5/S2 在 P5/S1 之后；P5/S3 在 P4/S3（与它引出的条件修复）、P5/S2 之后。
- 收尾在 P2–P5 全部合并之后；发布在收尾合并之后。

其余：

- 同时在实施的 Step 不多于两个：本机 8 核，重的运行串行，两个 Step 的门禁与审查已经排队（06-environment 第 3 节）。
- P4/S3 的实测要独占机器：安排在没有别的 Step 在实施与审查的时段，例如夜里。
- 页面树的结论若要改树，改法的 Step 排在 P2/S6 之前，P2/S6 随之等。

### 分份

- 组合根八份。收尾之后两次 `main`（`05dac827` 的 run 43339247136、`a057b6ad` 的 run 43344369124）的平均：325、326.5、402.5、312、425、326、257、267 秒（[M11 收尾审查](../M11-upgrade/reviews/M11-closeout-review.md)"守卫与分份的数"一节）。
  - 第五份两次都在 EPYC 7763 上（427、423），平均 425 秒，余量 25 秒。
  - 第四份这两次都不在 7763 上（339、285），平均 312 秒；再前两次（`07abd0f7`、`9e787bab`）都在 7763 上，平均 413.5 秒。
  - 第七、第八份的 257、267 秒是两次都不在 7763 上的数；在 7763 上是 315、318 秒（`9e787bab` 那一次）。
- M12 新的整个程序的测试**逐个点名进第七或第八份的模式**（`Makefile:296`、`:328`），照收尾时 `TestRevertingARollback…` 的先例（主持者的决定 328）。
  - 第四份是补集，第五份收 `TestThe(MCP|Lint|WikiTools)`、`TestRebuild` 等（`Makefile:263`），新测试的名字要避开它们。
  - 第二份是 `-run SECOND -skip FIFTH`（`Makefile:342`），不跳过第七份：名字以 `E`、`Source`、`TheMCP[C-Z]`、`TheContents` 开头的新测试点名进第七份之后，会在第二份再跑一遍。事件流现有的整个程序测试叫 `TestEvery…`（`bootstrap/events*_test.go`），P3/S1 的流数测试不照这个习惯起名。避不开时，第二份的 `-skip` 加第七份的模式，同 M11 收尾给第四份加的（设计审查 A 的 N5）。
- P1/S2 合并之后两次 `main` 重看各份；过 450 秒照 06-environment 第 4 节先挪给邻份，挪不下再分一份，不提高 `-timeout`。

### 移交的落实

六份移交在设计定稿时各写"处理进展"，写到 Step（主持者的决定 333），做完的 Step 各加一行，M12 收尾时改为 `done`。

**[M0-P6-image-release](handoffs/M0-P6-image-release.md)**

| 项 | 落实 |
|---|---|
| 1 推送 | `release.yml` 按摘要推送、合成索引、最后打 `0.1.0`；`packages: write` 只在 tag 触发的作业里，演练作业只有 `contents: read`（4.10），P5/S1、S2 |
| 2 多架构 | 两种原生 runner 各自构建、各跑一次 image-smoke；演练在每次推送上，P5/S1；推送 P5/S2；不交叉编译（I1-D3） |
| 3 签名、SBOM、来源证明、校验说明 | `actions/attest` 的来源证明与 SBOM、cosign keyless、前端的 SBOM，P5/S2；发布之后的核对作业在两种架构上跑两条校验命令，P5/S2；`deploy/README.md` 与 Release 的说明写两条校验命令，P5/S3、P5/S2 |
| 4 依赖、工具链与基础镜像 | 安全相关的更新、`make vuln`（Go 与 npm 两侧，`vuln.yml` 每天）与 `make outdated`，P1/S2；GitHub 的依赖图读不出 pnpm 12 的锁文件，Dependabot alerts 不当作 npm 一侧的手段（4.3）；Node 26 评估过，v0.1.0 仍用 24（4.3）；Dependabot 的版本更新 PR 由负责人定（第 11 节） |

**[M4-performance](handoffs/M4-performance.md)**（"只量"的都写进 P4 的记录、已知限制与 v0.2 的移交；能用已有的数推的就推，写明"推算"，4.7.1）

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
| 17 搜索 | P4/S3 批次 B，照 B1 与已接受的超出判（I2-D3）；WAL 的数写进部署文档（4.8） |
| 18 MCP | P4/S3 批次 C、E，照 I2 第 5.1.1 节的分项；`index` 照 B10 只量、调守卫；Caddy 早答写进部署文档（I1-D15）；名额的配置项不做（I2-D12） |
| 19 M10 的 LLM Wiki | P4/S3 批次 C，照 B6 判；TotalAlloc 照 B11（I2-D8）；迁移的停机只在批次 H |
| 20 两个守卫的 CI 记录 | P4/S3 的记录写 CI 的余量；`index` 的守卫照 B10 |
| 21 M11 的升级 | P4/S3 批次 C，照 B7 判 |
| 22 分份与 `apitest.Load` | M12 不主动做；某一份两次 `main` 的平均到约 430 秒时再做（I2-D11） |
| 补记 `:480–:486`（M10/P2/S2 的来源成本与迁移补充） | P4/S3 批次 C、E 只量：持续并发的来源首写负载与失败率、真实的附件文件。升级的停机窗口只在批次 H 的条件下量；没有 v0.1.0 之前的实例时省掉，记录写明（设计审查 C 的 I2） |
| 补记 `:488`（M10/P2 的后续接受补记，"open 保持"） | 同上一行；收尾时在移交里注明去处 |
| 补记 `:490`（最大合法 `batch` 的容量边界，"M12 要定"三件） | 硬件下限写进部署文档的"要求"，P5/S3，数取 P4；超时对用户的提示改文字，P3/S1（4.6.4，主持者的决定 335）；`batch` 单独的期限不做，进已知限制与 v0.2 |
| 补记 `:492`（守卫改 14 秒的根因：慢的 4 核离 15 秒不远） | 部署文档的硬件一节写明，P5/S3；B5 照 4.7.1：规则在本机判，最低配置的硬期限用 7763 上的实数加 `parse_max_wait` 判 |
| 补记 `:494`（M10/P4/S2 的实测记录） | P4/S3 只量：生产形状的并发负载、MVCC 的旧版本；大实例的迁移停机同 `:480` 一行 |

**[M5-performance](handoffs/M5-performance.md)**

| 项 | 落实 |
|---|---|
| 1 多实例 | 部署文档写明单实例与原因（I1-D18），P5/S3 |
| 2 多实例之间的时钟 | 单实例下不适用，部署文档写明（I1-D18），P5/S3 |
| 3 重连与整体刷新 | 不写重连的驱动（4.7.4）：P4/S3 照 I2 已有的数推算，写明"推算" |
| 4 自动保存与树写 | 并入 M4-performance 第 1 项 |
| 5 PostgreSQL 大版本 | 部署文档的大版本升级一节写复核这一条，P5/S3 |
| 6 一个账户的流数 | 4.6.5，P3/S1：每个凭证至多 4 条、重置同一凭证最老的；每个账户至多 64 条、答 429 |

**[M5-polish](handoffs/M5-polish.md)**

| 项 | 落实 |
|---|---|
| A、B 两类（第 1–49 条的子项） | 附录 A 的去处：P2/S1–S6，服务端的几项在 P3/S1 |
| 第 25 条（标题键）、10.1、10.2、30.2、30.4、30.5 | P3/S1（4.6.3、4.6.4） |
| 第 48 条 | 算 A，P2/S4（4.5） |
| 第 49 条 | 与 24.3 同一个改法，P2/S1（4.5） |
| 第 50、51 条（e2e） | P1/S1（4.4） |
| C 类 87 项与 3、12.2、11.6 的其余 | v0.2 的移交 `M12-polish.md`；发布说明的已知限制按主题写（4.13） |
| D 类 | 4.5 列出的几项随各 Step；其余进 v0.2 |
| M 类 14 项与读屏推断的八项 | 发布之后的人工验收（4.12），读屏推断的八项照负责人 2026-11-14 第 6 问，确认有问题的在 v0.1.1 修；33.1 先在 P2/S3 排查 |

**[M7-transfer](handoffs/M7-transfer.md)**

| 项 | 落实 |
|---|---|
| 1 结束的任务行（T1） | 部署文档与发布说明写明不清理，v0.2 定保留期 |
| 2 几个 GB 的导出 | P4/S3 批次 F，照 B8 判 |
| 3 部署文档 | P5/S3：点名 `exports/`、`imports/`；导出下载经代理的超时与缓冲；其余 README 已有（I1 第 7 节），挪进 `deploy/README.md` |
| 4 轮询；下载地址按客户端的时钟 | 轮询 P4/S2 只量；校时写进部署文档，P5/S3 |
| 5 按人的队列上限（T2） | P3/S3 修：按人的常量上限，答按人的新码与它自己的文字（4.6.6，主持者的决定 336；负责人 2026-11-14 第 5 问） |
| 5 几个 GB 的导入 | P4/S3 批次 F，照 B8 判 |
| 5 部署文档 | P5/S3（README 已有大半） |
| 5 名称不是 UTF-8（T3） | 先读 0x7075、再 GBK 回退，P3/S3；Shift-JIS、Big5、EUC-KR 的误读列进已知限制；CP437 与编码选项进 v0.2 |
| 5 River 取到之后停掉（T4） | P3/S3 修：启动时收拾（4.6.6，主持者的决定 336；负责人 2026-11-14 第 5 问）；`README.md:193` 随之改 |
| 6 总量配额（T5） | 写明由管理员监控磁盘，v0.2 |
| 6 按凭证的并发上传 | 不写驱动，写明不量与理由（4.7.1）；上限进 v0.2 |
| 6 附件很多时节点树的大小 | P4/S2 的页面树带附件的形状 |
| 6 从解析不到的附件链接上传（T8） | v0.2 |
| 6 部署文档 | P5/S3，含"停机时 `/mcp` 的调用在 20 秒处被切断"一句 |

**[M8-history](handoffs/M8-history.md)**

| 项 | 落实 |
|---|---|
| 1 撤销的次序比较（H1） | 已知限制，v0.2（负责人 2026-11-14 第 5 问） |
| 2 版本的保留策略（H2） | P4/S3 批次 G，照每次保存的版本行的大小推算增长；部署文档的容量，P5/S3；v0.2 设计保留策略 |
| 3 会话里按时间分版本（H3） | v0.2 |
| 4 永久删除（H4） | 界面的一句 P2/S2；部署文档与发布说明写明与变通 P5/S3；功能进 v0.2 |
| 5 日志的读（H5） | 4.6.2，P3/S1；变更集的详情一并 |
| 6 回滚的变更集的撤销（H6） | 4.6.1，P3/S2 |

**总体设计第 14 节写着 M12 的各行**：搜索 → P4/S3 批次 B；渲染与提取的内存、解析预算先到先得、`FOR SHARE` 与 MultiXact → 批次 E；lint 与 `index.md` → 批次 C；同一笔记本的保存串行与改名到期限 → 批次 E 与 D；附件与导入导出的配额与负载 → 批次 F、E 与部署文档；页面树 → 4.7.2；版本的增长 → 批次 G（推算）；升级的冻结时长 → 批次 C；`NOTIFY` 的全局锁 → 部署文档。每一行在 P4/S3 或 P5/S3 的文档提交里照结论改写（"总体设计的修订"第 9 条）。13.4 第 8、9 条里写着 M12 的几句同样（第 9 条）。

### 写给后面的 M（v0.1.1 与 v0.2）

- **v0.1.1**：发布之后的人工验收（4.12）发现的问题；读屏验收确认的八项。文档放 `docs/v0.1.1/`，与 `docs/v0.1/` 并列，`docs/README.md` 的目录结构随之加一行。v0.1.1 的工作由负责人另行批准开工，那时照一个小的 M 做：00 号文档、Step、审查与提交说明的写法照 M12。M12 只写它的移交 `docs/v0.1.1/handoffs/M12-after-release.md`（主持者的决定 326，设计审查 C 的 Q2）。
- **v0.2**：`docs/v0.2/handoffs/` 的五份（4.13）。v0.2 的总体设计开工时先处理它们。

### 总体设计的修订

定稿之后用脚本逐条替换，断言每段旧文字在文件里恰好出现一次（02-workflow 第 1 节第 8 步）。下面写出的每段旧文字都已这样核对过（`5e8af5b5`）。标"随某 Step 改"的，在那个 Step 的文档提交里改。

1. **12.2** 的 M12 一行，设计定稿时：`发布 v0.1 | 轻 |` → `发布 v0.1 | 中（五个 Phase、17 个 Step：地基（两个）、网页的打磨（六个）、服务端的正确性与历史（三个）、性能与内存实测（三个，另有条件的修复）、部署文档与发布流水线（三个）；不加接口操作与故事分组） |`。
2. **12.3**，设计定稿时：在 `- M11 依赖 M10（模式契约、守卫的框架、lint 的显式范围、模式变化的订阅者、来源的哈希）、M8（撤销的计划、后台任务与分批，用于回滚）、M9（MCP 的注册表、\`batch\`、多轮的调度）。` 之后加一行 `- M12 依赖 M0–M11 全部（发布的是它们）；改动前面 M 的地方逐项写在 [M12 总设计](M12-release/00-M12-design.md)第 8 节。`
3. **12.6**：设计定稿时 `| M12 | 发布 | 未开始 | — |` → `| M12 | 发布 | 进行中 | [00-M12-design.md](M12-release/00-M12-design.md) |`；收尾之后照 4.12 是"实现完成，待人工验收"。
4. **7.1**，随 P1/S2 改：`例如 \`postgres:18.6-trixie\`` → `例如 \`postgres:18.7-trixie\``。
5. **13.5 第 2 条**，随 P1/S2 改：`Node 本身按大版本（\`.node-version\`）。` → `Node 本身按大版本（\`.node-version\`）。Dockerfile 的 golang 镜像的版本（tag 与 server 阶段构建时断言的 \`go env GOVERSION\`）与 README 写给贡献者的版本等于两份 \`go.mod\` 的 \`toolchain\`（golang 镜像设了 \`GOTOOLCHAIN=local\`，\`FROM\` 里生效的是摘要）；测试、开发、部署样例与 smoke 的 PostgreSQL 镜像相同，\`pgtest/image_test.go\` 列出要核对的各处；都由测试核对（M12）。`
6. **13.5** 加第 6 条，随 P5/S1、S2 改：`6. **版本号与发布**（M12）：版本号只由 ldflags 注入，Makefile、buildinfo 与 Dockerfile 的默认值相同，由测试核对；发布之后改为下一个补丁版本加 \`-dev\`。发布由推 \`v*\` 的 tag 触发 \`release.yml\`，闸门只放过 \`main\` 上 \`ci.yml\` 已绿、\`make vuln\` 为绿的提交；依赖的漏洞扫描另在每天的 \`vuln.yml\` 里跑，只在 runner 上。镜像两种架构，带签名、SBOM 与来源证明，只打确切的版本号，不打 \`latest\` 与浮动的"大版本.小版本"。推 tag 之前问负责人；收尾之后为发布做的改动在 \`m12-release\` 这样的分支上，合并之后才推 tag。`
7. **第 11 节**，随 P5/S3 改：
   - 第一段，从 `**M10/P3/S1升级补充**` 到 `不以暖样本承诺生产停机。`（各恰好出现一次）整段换成 `部署、备份、升级与回退的步骤在 [deploy/README.md](../../deploy/README.md)（M12）；这里只列要点。`
   - 最后一段，从 `- **M10/P3来源表示升级（决定62）**` 到 `§3.0、§8.1a）。`（各恰好出现一次）整段删掉；开发版的升级若要写，在 `deploy/README.md` 的附录。
   - `**单实例**：v0.1 官方只支持单实例（附件在本地磁盘）。` → `**单实例**：v0.1 只支持单实例：附件在本地磁盘，限流、大请求体的名额与正在上传的导入只在进程内计数（M12）。`
   - `不可写时拒绝启动（M7）。` → `不可写时拒绝启动（M7）。M12 起发布到 \`ghcr.io/open-nerve/nervewiki\`：amd64 与 arm64，标签只是确切的版本号，不打 \`latest\`；带签名、SBOM 与来源证明，校验的命令在 deploy/README。`
   - `恢复时先恢复数据库，再放回附件目录。` → `恢复时先恢复数据库，再放回附件目录；恢复之后先 \`ANALYZE\` 再开放服务，命令与演练见 deploy/README（M12）。`
8. **5.8 第 6 步与 13.1 第 2 条**，随 P3/S2 改：
   - `回滚自己的变更集在"先删的子树落在收着基线节点的新建节点下"时也答冲突（M11 第 302 条，M12 修）。` → `回滚自己的变更集照常可以撤销（M11 第 302 条的冲突由 M12/P3/S2 修好，[M12 总设计](M12-release/00-M12-design.md) 4.6.1）。`
   - 13.1 第 2 条里从 `回滚的变更集由几遍、几个单元在不同时刻写下` 起的那一句（到 `第 6 条）。` 为止）换成照 P3/S2 实情写的一句：被这个变更集删了、现在本身是回收站根的节点，在撤销计划里是它自己的顶；后台撤销的分段随之。
9. **第 14 节**，随 P4/S3、P5/S3 改：写着 M12 的十一行（搜索、渲染的内存、`FOR SHARE`、解析预算先到先得、lint 与 `index.md`、`NOTIFY`、保存串行与改名、附件与导入导出、页面树、版本的增长、升级的冻结时长）照实测的结论改写："M12 实测"与"M12 压测"改为量得的数、结论与已知限制。旧文字的锚点：`M12 压测`、`M12 实测；不达标时改为分层加载`、`v0.1 全部保留；M12 实测后在 v0.2 设计保留策略`、`配额与负载留给 M12（M7 写的移交）`、`10 万页与 30 万页由 M12 实测`，各恰好出现一次；其余几行在 P4/S3 的文档提交里照同样的办法先核对。13.4 第 8 条里的 `进 M12 的性能移交（00-M11 第 232 条）`、`检查的十万页在 M12` 与第 9 条里的 `旧generic返回2000却扫52000的失败和未成功实验仍移交M12`（各恰好出现一次）一并照结论改成量得的数或已知限制（设计审查 C 的 M7）。
10. **13.1 第 28 条（标题键）与第 30 条（实时推送）**，随 P3/S1 改：标题键加切罗基文那一步；实时推送加每个凭证的上限与重置、每个账户的硬上限与 429。
11. **13.2 第 12 条**，随 P2/S1 改：从 `` `ConfirmDialog` 发送中仍用 `disabled` `` 起、到它之后的第一个 `第 23 条）。` 为止的那一句（起头恰好出现一次；中间是指向 `M12-release/handoffs/M5-polish.md` 的链接）→ `` `ConfirmDialog` 发送中用 `aria-disabled`，被拒之后焦点还给确认按钮或说明；只有横幅的拒绝时焦点给横幅（M12/P2/S1）。``
12. **13.4 第 3 条（端到端测试）**，随 P1/S1 改：加一句（P1 设计第 8 节第 5 条）："M12/P1/S1：e2e 换文档只经 `e2e/fixtures/navigation.ts` 的 `goto` 与 `reload`：它们先等页面没有在途的接口请求、这个文档的第一次续期已答、续期的锁空着，再换；有意在请求途中离开的写显式的逃生口 `{ inFlight: "leave" }`。门禁 `tools/e2e-navigation/check.mjs`（`make lint-web`）不许 `e2e/` 里别处直接调 `page.goto`、`page.reload`，按表核对每个文件的逃生口；同一个文档里的后退与前进不经帮手。A15 演示续期途中离开会被登出（M1 第 10 节），并核对帮手等的每一样；只有页面版本，接口那一面是 A5 (API)；在三种浏览器里跑，第一次续期发出之前离开的那一格只在 Chromium 与 Firefox（WebKit 在请求下一个文档之后旧文档发不出请求）（M12/P1）。"
13. **1.2** 的末句，收尾时：`以下几项明确留给 v0.2：同步 CLI` 起的那一句末尾，加 `；M12 的已知限制与推后的事项在 [v0.2 的移交](../v0.2/handoffs/)`。
14. **第 15 节**：设计定稿时加一行，写这些修订与依据；之后每次修订各加一行。
15. **00-M11 第 302 条**（不是总体设计，同一个提交里改）：`负责人若要放弃也输入名称，在 M12 改。` → `负责人若要放弃也输入名称，在 M12 改。（M12 更正：这里的"v0.1 之内不修"本意是"M11 之内不修"；照负责人 2026-11-13 第 1 问（它点名了 302）在 M12 修，[00-M12](../M12-release/00-M12-design.md) 4.6.1。）`
16. **`docs/README.md`**：设计定稿时，Git 一节 `- M11 同 M10（…）` 之后加 `- M12 同 M11（[M12 总设计](v0.1/M12-release/00-M12-design.md)第 7 节），另有发布的 \`(M12 release)\`。`；收尾时目录结构加 `v0.1.1/` 与 `v0.2/handoffs/` 两行。
17. **13.2 第 26 条与第 1 条**，随 P2/S1 改（设计审查 C 的 M7）：第 26 条从 `无主笔记本接管之后的焦点（M3）还没有照此` 起的那一句（恰好出现一次）照 22.17 修好之后的实情改写，不再写"v0.1 之后"；第 1 条里 `M12 的体验移交第 7 项`（恰好出现一次）照第 7 条"仍要退出吗"的结论改写。
18. **13.1 第 32 条**，随 P1/S2 改（设计审查 B 的 M1）：从 `go1.27.1 里它在默认打开的 GOEXPERIMENT` 起的那一句（恰好出现一次）照 1.27.4 里 `jsonv2` 的核对结果写。

**文档与代码不符之处**（M12 里改文档，不改代码，除非另写）：

- 5.8 第 6 步写"M12 修"、00-M11 第 302 条写"v0.1 之内不修"：第 15 条与 4.6.1。
- 13.2 第 12 条、第 26 条与打磨移交的"v0.1 之后"：4.5，第 11、17 条。
- 总体设计 13.1 第 32 条、`06-environment.md`、`05-lessons.md`、`03-project-background.md` 写着 Go 1.27.1：P1/S2（4.3，第 18 条）。
- `ci-wait.sh` 是 06-environment 第 4 节规定的读 CI 的工具，它原取这个提交的第一个运行、不分工作流：已在 `b54cd679` 改好（4.2）。
- 移交 M4-performance 第 14、15 条的守卫时限写 12 秒，代码是 14、14、8 秒（`bootstrap/trash_cost_test.go:539`、`revert_cost_test.go:623`、`:670`；M10 第 153 条）；第 16 条写 1 秒、2.1 秒，代码是 1.3 秒、3 秒（`bootstrap/background_cost_test.go:936-937`；M11 第 249 条）；第 15–18 条的份名是旧的（I2 第 8 节第 1–3 条）。P4 照代码量，移交收尾时注明。
- 移交 M8-history 第 5 条写"两条语句"，现在是五条读（I3 第 5 节第 4 条）。
- 移交 M5-polish 第 25 条写"分词的版本随之提升"：不需要（4.6.3）。
- 移交 M5-polish 已修或部分修了的：12.7 的一半、24.10 的注释、22.18 的大半、21.12 的一半、第 3 条的公式与图、`NativeSelect` 的宽度（I3 第 5 节第 5 条）。附录 A 只列剩下的。
- `Makefile:12` 的注释"默认值与 buildinfo 中的相同"：Dockerfile 另有两份，P5/S1 的守卫之后注释照实改。
- `README.md:629` 写 `postgres:18.6-trixie`，不在守卫里：P1/S2；P5/S3 换成 `deploy/README.md` 与 `deploy/compose.yaml`。
- `README.md:499`、`:532`、`:539` 与总体设计第 11 节的首尾两段写着"M12 实机复核"：照 P4 的结论改成已知限制（第 7 条）。

### 提交说明

照 M11（`docs/README.md:105`），加发布的一种：

- **代码**：`<范围>: <说明> (M12/P<n>/S<m>)`，范围是部件（`server`、`api`、`web`、`e2e`、`deploy`、`build`、`docs`，多个用逗号），例如 `e2e: wait for the reads in flight before a story's goto (M12/P1/S1)`。
- **文档**：直接进 `main` 的写 `docs: <说明> (M12/P<n>)`，M 级的写 `(M12)`，例如设计定稿 `docs: the M12 design, its review, and each hand-off item's place in M12 (M12)`；Step 的分支上的每个提交，文档的也是，写 `(M12/P<n>/S<m>)`。
- **审查的修复**：Step 的分支上、合并之前的写到 Step；合并之后的 Phase 的写到 Phase；修复的核对写第几轮，`(M12/P<n>/S<m> fix check <k>)`；收尾 `(M12 closeout)`。
- **合并**：一个 Step 一次，`Merge branch 'm12-p<n>-s<m>': <说明> (M12/P<n>/S<m>)`，正文经 `git merge --no-ff -F <文件>`。`Merge main into <分支>: <一句说明>` 不带后缀。
- **旁支**：分支 `m12-<slug>`，写它所属的 Step 或 Phase；修的是更早的 M 的故事时写 `(M<k> story, found in M12)`。`vuln.yml` 红了的旁支是 `m12-toolchain-<版本>`，写当时所在的 Phase（4.3）。
- **发布**：收尾合并之后、为发布做的提交在分支 `m12-release` 上（主持者的决定 332），写 `(M12 release)`，例如 `build, api: main's version after v0.1.0 is 0.1.1-dev (M12 release)`；合并写 `Merge branch 'm12-release': <说明> (M12 release)`，正文经 `git merge --no-ff -F <文件>`。
- **tag**：带注释的 tag，`git tag -a v0.1.0-rc.1 -m 'Nerve Wiki v0.1.0-rc.1'`；不签 GPG（镜像由 Sigstore 签）。
- 分支名 `m12-p<n>-s<m>`。每个提交以空行加 `Co-Authored-By:` 一行结尾。

## 8. 本 M 建立与注册的扩展点

M12 不建立新的扩展点，也不注册（12.4 里没有 M12）。

改前面 M 的代码（12.1 第 6 条的例外，逐项写明；都不改扩展点的签名）：

- **page（M8、M9）**：
  - 撤销计划的 `readReverted`/`topOf`（`page/domain/revert.go:275-326`）把被这个变更集删了、本身是回收站根的节点当作它自己的顶；`RevertGroups`、`scheduled`、`CheckRevertPart`、`CheckRevertSteps`、`RevertActionFacts` 传入 `Nodes`；`groupIsDeleted`、`leftDeleted` 与调度不改（M8，P3/S2）。
  - 后台撤销的 `revertSteps`/`needed`（`page/app/jobs.go:578-585`），`needed` 也看段内后面的放回（M9/P3，P3/S2）。
  - 日志与变更集详情的读放进快照，`HistoryReads` 加 `Snapshots`（P3/S1）；MCP `log` 的路径约定（P3/S1）；`searchFold` 的切罗基文映射删掉（P3/S1）。
- **shared（M4、M9）**：`TitleKey`、`TitleKeyContext` 的切罗基文一步；`CheckName` 与 `CleanClientName` 之后整个名字的判断，`Unshowable` 不动（P3/S1）。
- **linking（M6、M9）**：`IndexKeyer` 的切罗基文一步；`listTags` 不列 `/`（P3/S1）。`index` 的约定不变。
- **platform/markdown（M4、M6）**：`PlainText` 带上自动链接与原始 HTML 的文字（P3/S1）。
- **platform/httpserver/mcpserver（M9）**：调用超时的文字（`errors.go:77`，P3/S1）。
- **events（M5）**：hub 按凭证数流、以新的原因 `superseded` 重置同一凭证最老的那条，账户的硬上限答 429（P3/S1）；`events.yaml` 的 reset 原因加一个值；网页的 `events/hub.ts` 对 429 的处理若要改，同一个 Step。
- **transfer（M7、M9）**：导入的条目名先读 0x7075、再 GBK 回退与报告的新原因；`room` 按人计数、答按人的新码；`AtStart` 收拾 River 取到而没开始的任务，River 那一边照 `Job.DiscardRunning` 的先例（P3/S3）；网页开任务的各处照新码显示（P3/S3）。
- **wiki（M10）**：两份 `AGENTS` 模板（`wiki/domain/template-*-agents.md`）的"三个 prompt"的说法（P2/S2）。
- **网页（M4–M11）**：附录 A 的各文件（P2）。
- **e2e（M0 起）**：夹具（含 `wiki-health.ts`）与 109 个故事文件（P1/S1）；打磨的断言（P2）；`e2e/perf/`（P4/S2）。
- **构建与 CI（M0）**：`ci.yml` 的触发与两个 action、`make deploy-check`；新的 `release.yml` 与 `vuln.yml`；Dockerfile 的摘要、构建时的 Go 版本断言与 label；Makefile 的新目标（P1/S2、P5）。`ci-wait.sh` 已在 `b54cd679` 改好，不在 Step 里。

## 9. 测试策略

### 9.1 打磨：每一类的测试写法

每一项先写一个在没修的代码上失败的测试，再修（13.4 第 1 条）。

- **焦点**：
  - jsdom 做得到的（焦点落在哪个元素、被拒之后回到哪里）用 vitest；断言 `document.activeElement` 是那个元素，不只是"不是 `body`"。
  - jsdom 做不到的（Chromium 在下一次渲染时才做 HTML 的焦点修正，Safari 不让点过的链接获得焦点，05-lessons 第 6 节）进 e2e，在三种引擎里探查；改到的故事照需要加进 `threeBrowsers`（`e2e/playwright.config.ts:3-4`）。
  - 一处改了焦点的规则会波及别处：对照空闲退出、未读的内容、portal 里的对话框逐一核对（05-lessons 第 6 节）。
  - 变异：`aria-disabled` 改回 `disabled`；去掉被拒之后的还焦点；去掉 `watchReader`（13.2 第 26 条）。
- **说明进不进确认框的描述**（24.3、49）：vitest 断言那一句在 `aria-describedby` 指的元素里。
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
- **只从 ARIA 推断的八项**：不写自动测试，进读屏的人工清单（4.12）。
- **阅读视图（P2/S4）**：更新之后，选区、展开的 callout、焦点、正在播放的媒体都还在；17.1 的整点替换由 vitest 的假定时器推进触发，不真的等一小时。

### 9.2 服务端

- **第 302 条**（P3/S2）：
  - 两个现成的性质测试扩开（4.6.1）：回滚的那一个在 `Done` 之后撤销回滚的变更集，重放对照 agent 留下的状态；后台撤销的那一个的生成器加嵌套回收站根的形状。期望由各自的模型算出，不读被测的计划（05-lessons 第 1 节）。
  - 下限：数"嵌套的回收站根"真的出现的次数，不是规则触发的次数；换几组种子（05-lessons 第 1 节）。
  - 确定的一格：不是回滚的"恢复之后先删里面、再删外面"的变更集照常撤销。
  - 组合根：nested 一格翻过来，两本笔记本，请求的撤销与后台的撤销各一本。
  - 用时：`race` 一份里各多至多约 15 秒（第 7 节 P3 的表）。
- **H5**（P3/S1）：确定的交错（13.4 第 4 条）：持住撤销要的行，让日志的第一条读先读，再提交撤销，再让后几条读；断言读到的一致。变更集的详情同样一格。去掉 `WithinSnapshot` 的变异让它们失败。
- **标题键**（P3/S1）：`title_key_test.go` 的幂等样本加切罗基文；同一父页下只差切罗基文大小写的两页被拒；搜索的结果不变（删掉 `searchFold` 的映射前后，同一组查询比对）。
- **名字与 `log` 的路径**（P3/S1）：
  - 只由 U+200B、U+2060、U+3164 组成的显示名答 `required`，客户端名当作空的；带 ❤️、家庭 emoji、ZWNJ 的波斯文的标题与名字照旧通过；`TestChecksRejectCounterexamples` 不改。
  - `log` 的 `path` 三种：省略、`""`、`"/"`；`index` 的 `parent:"/"` 照旧是根。
- **MCP 调用超时**（P3/S1）：让一次调用的 ctx 到期，答复的文字是新的那句，码照旧。
- **流数**（P3/S1）：hub 的单元测试（4.6.5 的几格，含"17 条活流在有限步之内停下"与"账户满了时凭证自己顶掉自己的、不被拒"）；一个整个程序的测试经 `/api/v0/events`：同一凭证开 5 条，最老的那条收到原因是 `superseded` 的 `reset`。按账户而不按凭证数、上限交成很大、先看账户再看凭证，几个变异都让它们失败。
- **名称的编码**（P3/S3）：
  - 测试里生成几种 zip：带 0x7075 的条目（CRC 对得上取它的名字，对不上不取）；没有 UTF-8 标志、名字是 GBK 字节的；有 UTF-8 标志却不是合法 UTF-8 的（照旧跳过，读 `Flags`、不读 `NonUTF8`）；一个名字按 GBK 解出 U+FFFD 的（整包不回退）。
  - 合法的 UTF-8 名字不变；差分：解读只改原本不是合法 UTF-8 的名字（13.4 第 11 条）。
- **T2、T4**（P3/S3）：一个人到上限时他的下一个答按人的新码、别人的照常开始，全局满了照旧答 503；网页的文字由 vitest 整句比对；整个程序的测试造出"River 取到、没开始"的状态，重启之后行是失败、名额空出。

### 9.3 e2e

- P1/S1 的帮手有一个对照测试（4.4），经逃生口复现、经帮手不落。
- 门禁有反向对照：在一个故事里写一个直接的 `page.goto`、在帮手之外的夹具里写一个直接的 `page.reload`，门禁都失败；帮手所在的模块直接调用它们，门禁不报、不计数；逃生口多一处、少一处，或出现在表外的文件里，门禁都失败。
- M12 不加故事分组。打磨的断言加进那一页已有的故事；没有的，在它的分组里加一个故事（照 10.1，每个故事一个文件）。

### 9.4 实测

方法在 4.7.3。每一项的记录写：

- 机器（`nproc`、内存、当时的 `free`、`df`），PostgreSQL 与 Go 的版本，提交；
- 规模与形状、种子、命令；
- 两轮的数（p50、p99、峰值、时长），取哪一轮；
- 判在哪台机器上（本机、CI 的实数，或乘了哪个倍数并写"推断"），对照的预算与结论（过、超）；
- 只量的写数，或写"推算"与依据的数；
- 超出的去处（修的 Step，或负责人的改判）。

### 9.5 发布流水线怎样在不推送的情况下验证

推送、签名、证明与 Release 只有真的 tag 才走得到（I1-D7）。之前分两层核对，rc 是第三层：

1. **静态**：
   - actionlint 核对三份工作流（`make lint`）。
   - 读 `release.yml` 的检查：演练作业没有 write。反向对照：给它加 `packages: write`，检查失败。
   - 闸门脚本 `deploy/release-gate.sh` 在本机对着假的输入跑。tag 名不是 semver、提交不在 `main` 上、提交的 `ci.yml` 不绿、"`ci.yml` 红而 `release.yml` 绿"、`CHANGELOG.md` 没有这一节、`make vuln` 红（npm 一侧照 `high`：一个 `moderate` 的公告不挡、一个 `high` 的挡），各被拒或放过一次（反向对照）。
   - 说明脚本 `deploy/release-notes.sh` 对着假的 `CHANGELOG.md` 跑：rc 取到去掉 `-rc.N` 的那一节、前面有"预发布"；缺这一节就拒。
2. **演练**（P5/S1）：单独的只读作业，两种原生 runner 各自 `push: false`、`load: true` 构建，对本地的镜像跑 `image-smoke.sh`；核实路径上下文带上了 `.git`、VCS 信息在。分支与 `main` 的每次推送都跑。
3. **rc**：推 `v0.1.0-rc.1`，走完整条路，照 4.11 第 4、5 步核对。keyless 签名、`actions/attest`、GHCR、Release、复制出版本号的 tag 与重跑，只在这一层第一次真的跑。只改流水线的 rc.N 不再问负责人（决定 324），这一层因此便宜。

初稿的另一层（本机起 `registry:2`、cosign 用临时的 key、syft）删掉（设计审查 B 的 M10 与过度设计）。它与 GHCR 的差别正落在要验证的地方：本机的 buildx 只有 `docker` 驱动与 `linux/arm64`，合成的索引只有一种架构；`registry:2` 没有 OCI 1.1 的 referrers 接口，cosign v3 经它存签名；cosign v3 不进透明日志的签名，校验要另带 `--insecure-ignore-tlog=true`；还要装 syft、cosign。

### 9.6 变异（每个 Phase 设计列全，这里是要列进去的）

- P1/S1：帮手什么也不等（A15 第 2、4、5、6 格抓）；帮手不等这个文档的第一次续期（第 4 格，Chromium 与 Firefox 落到 `/sign-in`、WebKit 报旧文档没发续期）；帮手不看续期的锁（第 6 格）；帮手不等在途的读（第 5 格）；`reload` 不先等（第 2 格）；门禁放过直接的 `goto` 或 `reload`；门禁不看 `e2e/fixtures/`；门禁不核对逃生口的表；门禁把帮手模块里的逃生口文字也算进去（P1 设计 7.2）。
- P1/S2：Dockerfile 的 Go 版本的 tag 与 `toolchain` 不同（守卫抓）；摘要换成别的版本的（构建时的断言抓）；README 的 PostgreSQL 版本不同（守卫抓）。
- P2：9.1 列的几种，每个 Step 照它的项补全。
- P3/S1：`TitleKey` 不做切罗基文一步；三处只改一处；`PlainText` 仍跳过自动链接；`listTags` 列 `/`；日志或变更集详情的读不在快照里；流数按账户而不按凭证数、重置最新的那条、账户的硬上限不拒、先看账户再看凭证；整个名字的判断不认只由 U+200B、U+2060、U+3164 组成的名字；`index` 的 `parent:"/"` 被拒；`log` 的 `""` 被当作整本；超时的文字照旧。
- P3/S2：去掉新规则；`topOf` 走过回收站根；`needed` 不看段内后面的放回。
- P3/S3：不看 UTF-8 标志就按 GBK；读 `NonUTF8` 而不读 `Flags`；合法的 UTF-8 名字也被解读；结果里有 U+FFFD 也回退；一个名字解不开时其余的照样回退；0x7075 的 CRC 不核；报告不写；`room` 不按人数；按人的拒绝答成通用的 `server_busy`；`AtStart` 不收拾排队的行。
- P5/S1：四份默认值有一份不同（守卫抓）；MCP 的版本不取 buildinfo（短测试抓）；闸门的各条各去掉一条；闸门不分工作流；演练作业加了 write（权限检查抓）。

### 9.7 门禁与 CI

- 每个 Step 照 02-workflow 第 3.1 节：`make lint gen-check knip build`、`make test`、`make test-go-short`、`make e2e`，都经 `heavy.sh`；每个提交单独为绿。
- `make image`、`make image-smoke` 不能在 worktree 里跑（06-environment 第 2 节）：P1/S2、P5 要它们时在一个普通的克隆里跑。
- 每个 Step 合并之后两次 `main` 的分份照第 7 节"分份"。
- 读 CI 用 `ci-wait.sh`，它按工作流取，默认等 `ci.yml`（`b54cd679`）；读 `vuln.yml` 或演练时，第二个参数写那个工作流。
- 每个 Step 合并之前看一眼 `vuln.yml` 最近一次的结论（第 7 节 P1 的说明）。

## 10. 风险

| 风险 | 应对 |
|---|---|
| **发布前的升级**：PostgreSQL 18.7 换了某条语句的计划；Go 1.27.4 的运行时改了竞态下的时间；新的 Debian 摘要带来别的变化 | P1 最先做，之后的每个 Step、计划测试与实测都在新的版本上跑；计划测试（13.4 第 9 条）在 18.7 上跑一遍，换了的在 P1/S2 里查清，不放宽；同一个 trixie，glibc 的大版本不变；`jsonv2` 的实验开关由 `mcpserver` 的测试核对（4.3） |
| **`vuln.yml` 在 M12 期间变红**：Go 的小版本约每月一个，多数带安全修复；npm 一侧 `low` 也报、不看可达性；定时运行的通知不发给主持者 | 不在每次推送的 CI 里，Step 照常合并；每个 Step 合并之前看一眼它最近一次的结论；旁支 `m12-toolchain-<版本>` 升，npm 一侧修不了的写进 `audit.ignore`，闸门只挡 `high`（4.3）；发布的闸门硬挡；收尾之后才发现的再问负责人 |
| **第五份与第四份的余量**：收尾之后两次（`05dac827`、`a057b6ad`）第五份都在 EPYC 7763 上，平均 425 秒，余量 25 秒；第四份这两次不在 7763 上，平均 312 秒，再前两次在 7763 上平均 413.5 秒 | 新的整个程序的测试点名进第七、第八份（收尾之后两次 257、267 秒，在 7763 上 315、318 秒），名字避开第二、第五份的模式；第四、第五份不加；P1/S2 合并之后两次 `main` 重看；过线先挪份，不提高 `-timeout` |
| **e2e 的偶发挡发布**：Playwright 不重试，`main` 的 CI 红了就不能打 tag | P1/S1 最先做；门禁数逃生口；`ERR_NETWORK_CHANGED` 加一行日志看它在 CI 上触发了没有；其余的偶发照 M11 第 277 条的做法只重跑 e2e，并在记录里写明 |
| **演练作业的 CI 用时**：每次推送（分支与 `main`）多两个各约 10–15 分钟的作业（推断） | 与 `ci.yml` 并行，不进组合根；`ci-wait.sh` 按工作流取，不被它挡住也不被它冒充 |
| **推 tag 之后才走得到的路径**：keyless 签名、`actions/attest`、GHCR 的推送与权限、Release、复制版本号的 tag；`build-push-action` 的上下文 | 9.5 的三层；rc 先走一遍；一时的失败在同一次运行上重跑，不重推 tag（4.10）；允许的第三方 action 与组织的 Actions 设置由负责人在推 rc 之前核对（第 11 节） |
| **GHCR 设为公开不能撤回** | 只在 rc 核对之后做；之前核对镜像里没有机密：最后一层只有二进制与 `/licenses`（`deploy/Dockerfile:41-47`），构建参数只有版本与 label，`.git` 只在构建的阶段；Release 的说明与附件在 rc 上先看一遍；版本号的 tag 在签名与证明之后才打 |
| **先发布、后验收**：v0.1.0 发出去之后，人工验收可能在输入法、真实的 MCP 客户端或读屏软件上找到问题 | 发布说明写明人工验收在发布之后、还没逐项核对过；v0.1.1 的移交与目录先定好（第 7 节"写给后面的 M"）；部署文档要求生产写确切的版本号，照升级的步骤换版本 |
| **打磨的面宽**：约 113 项，焦点的修复常波及别处（M5–M7 的修复核对一再找到回归，05-lessons 第 8 节） | 按组件分六个 Step；焦点的修复在三种引擎里探查；阅读视图先写 spec；修复的核对一轮一轮做到干净 |
| **第 302 条的修法改的是所有撤销都走的计划** | 先写 spec；改法小（`topOf` 在本身是回收站根的节点处停，审查 A 的原型约 10 行）；认出的三种形状写进 spec；两个现成的性质测试扩开；后台撤销的 `needed` 随之改；M8 原有的撤销测试照过；收尾审查 A 专看撤销的其余路径 |
| **标题键与标题 id 改了已有的键** | 发布之前做；发布说明写明试用库要 reindex、这类锚点变了；不升分词与提取规则的版本，免得白白重建 |
| **同一凭证上 5 个以上守契约的客户端轮流互顶** | 按凭证计数，不同设备与令牌不互顶；同一凭证的轮转受它自己的限流约束；被重置的一行是 `debug` 或限频；hub 的单元测试有"17 条活流在有限步之内停下" |
| **GBK 回退把别的编码误读成中文** | 先读 0x7075；回退的条件写明"结果里没有 U+FFFD"，一个名字解不开整包不回退；Shift-JIS、Big5、EUC-KR 仍会被误读，写进已知限制；报告列出解读的名字 |
| **T2、T4 改了任务的入口与启动** | 只改 `room` 与 `AtStart`；P3/S3 先写设计；T4 假定单实例，与部署文档一致；整个程序的测试 |
| **B5 的规则在本机上可能过不了**：本机比 M5 Max 慢，界线处的撤销单次记过约 6.2 秒（移交 M4-performance 第 15 条，哪台机器没写明） | B5 只用 M8 的规则判，不再乘倍数；最低配置的硬期限用 7763 上的实数加 `parse_max_wait` 判；规则一层没过不自动降常量，照移交第 14 条的两种做法（降常量，或写进部署要求）列给主持者与负责人定（4.7.5） |
| **实测的时长与机器**：搜索一批约 20 GB 的磁盘；交换已满，噪声大 | 范围先缩到有预算的与部署文档要的；独占时段跑、经 heavy 的锁；之前修剪 Go 的构建缓存；两轮取较差的；本机为参照，部署文档写明 |
| **页面树 1 万页没过** | 4.7.2 先量它，结论先于 P2/S6；改法要设计，是一个大的 Step，M12 随之变长；负责人可以改判为已知限制 |
| **并行的 Step 争机器与翻译文件** | 同时实施的不多于两个；`en.ts`、`zh-CN.ts` 按序合并、先 `git merge main` |
| **`/tmp` 的用户配额与 heavy 的锁** | 锁已固定在 `/tmp`（`8c247237`）；大的输出放 `~/.cache` |
| **第三方许可证收不全** | P5/S2 先做个小实验；收不全的列在 Release 的说明里，并进 v0.2 的移交 |

## 11. 负责人确认的决定

### 2026-11-13：负责人批准开始 M12 与四个产品问题

负责人的话："开 M12 吧，按标准流程推进"。主持者以选择题问了四个产品问题，负责人各选一项。原问与所选，逐字：

1. 问："M12 的打磨移交有 51 条，另有历史移交 6 条（含 M11 决定 302 的回滚撤销冲突）。v0.1 之内修到哪一步？"
   选："分级处理 (Recommended)"，即"会碰到的错误行为、无障碍问题、误导的文字在 v0.1 修；纯体验改进写成 v0.2 的清单，发布说明列为已知限制。"
2. 问："性能与内存实测（10 万、30 万页的页面树、搜索、导入导出、升级等）量出超标时怎么处理？"
   选："超出设计预算才修 (Recommended)"，即"量到总体设计写的规模；只修超出设计已写预算的（例如页面树超标就改分层加载），其余记成已知限制进 v0.2。符合你 11-06 说的不无限优化。"
3. 问："v0.1 发布成什么形式？仓库是公开的（AGPL-3.0）。"
   选："Release + GHCR 镜像 (Recommended)"，即"打 v0.1.0 的 tag、写 GitHub Release 说明，推 amd64 与 arm64 的镜像到 GHCR，带签名、SBOM 与来源证明。主持者把一切备好，推 tag 之前再问你一次。"
4. 问："欠你的人工验收（MCP 真实客户端清单 A–I、J、K，读屏软件）与发布 v0.1.0 的先后？"
   选："先发布、后验收"，即"先发 v0.1.0，人工验收发现的问题出 v0.1.1。"（推荐的是"验收通过才发布"，负责人没有选它。）

负责人 2026-11-06 另说过："不要把这次当做性能问题继续无限优化了，调整测试用例量级或者时限即可"。

### 2026-11-14：设计审查之后的两问（审查 C 的 M12DC-I1）

初稿把四项罕见的 A 与九项只从 ARIA 推断的 B 移出了 v0.1，偏离第 1 问所选的字面，却没有列为要问负责人。主持者照审查 C 问了两问，原问与所选，逐字：

5. 问："按你选的分级规则，用户会碰到的错误行为（A 类）要在 v0.1 修。但有四项 A 类很罕见：H1（同一间隔连着插约 30 次之后撤销误报冲突，改动大、要迁移）、24.1（网络结果未知时任务列表可能指到别处的任务，改动大）、T4（任务取到之后、开始之前停机，占着名额约 7 小时，小到中）、T2（一个人能占满全局的任务队列，小）。怎么处理？"
   选："小的修、大的进 v0.2 (Recommended)"，即"T2、T4 在 v0.1 修；H1、24.1 写成发布说明里的已知限制，进 v0.2。"
6. 问："打磨里有九项无障碍问题，只是从 ARIA 的写法推断'读屏可能念两遍或念得太急'，没有用真实的读屏软件验证过。你已把读屏验收放在发布之后。这九项怎么处理？"
   选："发布后随读屏验收 (Recommended)"，即"并进发布之后的读屏验收清单，确认有问题的在 v0.1.1 修。其余确定的无障碍问题（焦点丢到 body、缺名称、对比度、超宽）照样在 v0.1 修。"

照此：T2、T4 在 v0.1 修，放进 P3/S3（4.6.6，决定 336）；H1、24.1 写成已知限制，进 v0.2（4.5）。第 6 问的九项里，49 与留在 v0.1 的 24.3 是同一种"不念"的问题（审查 C 的 M1），移回 P2/S1；其余八项并进发布之后的读屏验收（4.5、4.12）。49 留在 v0.1 比第 6 问所选多做了一项，在 M12 的汇报里列出。

### 2026-11-14：开始 M12 之前的主持者的决定 304–313、315–322、325（负责人可以改判）

主持者在说明里写明的几条，加上三份调查里每个决定的推荐选项（主持者暂取）。编号从 304 起；初稿的临时号与它的对照在[审查记录](reviews/M12-design-review.md)。设计审查改过的标出。

- **304 第 302 条在 M12 修**。"v0.1 之内不修"本意是"M11 之内不修"；负责人第 1 问已点名 302，选的是"分级处理"，它是 A。修法：被这个变更集删了、现在本身是回收站根的节点，在 M8 的撤销计划里是它自己的顶；后台撤销的 `needed` 随之；两个现成的性质测试扩开（4.6.1；设计审查 A 改，初稿是"把更早删下的回收站根并进顶的组"）。
- **305 页面树的阈值是 1 万页**（I2-D1）。量之前写死每一条界线（界线由设计者提，审查者核，4.7.2）；1 万页过了保留整棵树；5 万、10 万只量、记成已知限制，分层加载进 v0.2；1 万页没过，在 v0.1 改，那时再设计。
- **306 发布的顺序**：收尾审查先做；两个 tag 一起问负责人（I1-D7）；推 rc、校验，再推 v0.1.0；收尾记录与 12.6 写"实现完成，待人工验收"，人工验收在发布之后（4.11）。
- **307 负责人自己在 GitHub 上做的设置**列在本节"需要负责人确认"，带"什么时候要"（I1-D19）。
- **308 漏洞扫描**：`make vuln` 跑 `govulncheck`（只扫 `server/`）与 npm 一侧的 `pnpm audit --prod`，只在 runner 上：一个每天跑、可手动触发、改了依赖清单的推送也跑的 `vuln.yml`，与发布的闸门；不进每次推送的 `ci.yml`；npm 一侧闸门只挡 `high`，修不了或无关的写进 `audit.ignore`；每个 Step 合并之前看一眼它的结论；红了开旁支 `m12-toolchain-<版本>`。本机不跑它们与 `make outdated`：调查时权限分类器拒过，规矩是不往外发。GitHub 的依赖图读不出 pnpm 12 的两个文档的锁文件，不当作 npm 一侧的手段（I1-D10，4.3；设计审查 B 改，初稿是"`govulncheck` 进 CI 的 `short` 一份，npm 一侧靠依赖图告警"；修订核对第 1 轮补了 `push` 触发、npm 的级别与忽略、谁去看）。
- **309 heavy.sh 的锁**固定在 `/tmp/nervewiki-heavy.lock`，仍可由 `NW_HEAVY_LOCK` 改；06-environment 写 `/tmp` 的配额与"改 `TMPDIR` 不再改变锁"。主持者已单独提交（`8c247237`，4.2）。
- **310 工具链与依赖只做与安全相关的更新**（I1-D9、D10、D12）：Go 1.27.4、PostgreSQL 18.7、三个镜像的摘要、两个 action、`make vuln` 报出的模块与包；Node 26 与功能性的大版本不升；加 `make outdated`（I1-D11）。Dockerfile 的 Go 版本由 tag 与构建时的断言两处守住；文档里写着 1.27.1 的五处随之（4.3；设计审查 B 补）。
- **311 e2e 的帮手**（I3 第 7 节第 14 条）：`goto` 与 `reload` 之前等在途的续期与读；跨文档的后退另给一种；显式的逃生口只给有意在请求途中离开的故事（P1 里是 A15），门禁数它；门禁含 `e2e/fixtures/`；M1 第 10 节的取舍不动（4.4；设计审查 C 改，初稿把 `goBack`、`goForward` 一律算进来，没有逃生口；P1 设计 2.1 第 2 条改，原来是"显式的逃生口给 TR2 与对照测试"；P1 设计 2.1 第 1 条：没有故事要跨文档的后退，不做）。
- **312 打磨的分级**（I3 第 7 节第 1、2、6、7、12、13、15 条）：照 I3 第 9 节的分类；只从 ARIA 推断的八项并进发布之后的读屏验收（负责人第 6 问），49 与 24.3 同改、留在 v0.1；H1、24.1 写成已知限制（负责人第 5 问）；第 48 条算 A；单复数用 `*One` 键；退出登录问"仍要退出吗"；第 40 条两份模板都改；M 类并进发布之后的人工验收，33.1 先排查（4.5；负责人第 5、6 问与设计审查 C 改）。
- **313 标题键与标题 id 在 v0.1.0 之前改**（I3 第 7 节第 4、5 条）：三处折叠一起改，删掉 `searchFold` 的映射，不升分词的版本；10.2 不升提取规则的版本、不要 reindex（4.6.3；设计审查 A 补）。
- **315 事件流的上限**（I2-D9）：每个凭证至多 4 条，超出以新的原因 `superseded` 重置同一凭证最老的一条；先按凭证判，只有净增一条时才看账户的上限：每个账户至多 64 条，超出答 429 带 `Retry-After`；两个都是常量（4.6.5；设计审查 A 改，初稿是"每个账户至多 16 条、配置项、超出重置最老的"）。
- **316 导入的名称编码**（I3 第 7 节第 8 条）：先读 Info-ZIP 的 0x7075，再 GBK 回退（结果里没有 U+FFFD；一个名字解不开整包不回退；读 `Flags`）；Shift-JIS、Big5、EUC-KR 的误读列进已知限制；不做编码选项；CP437 进 v0.2（4.6.6；设计审查 A 改）。
- **317 实测的范围与判据**（负责人第 2 问；I2-D3、D6、D7、D8、D10、D11、D12、D13）：只有 4.7.1 的预算判"超出"；B10 的两个守卫不是产品的预算，`index` 只量，过 3.5 秒调守卫、乘了之后过 15 秒才照 B4 修；搜索的尾巴与体积照 M8 已接受的；`DeleteNode` 与单个 `move` 只在正常形状到期限时修；按页的日志、lint 的 TotalAlloc、MCP 名额的配置项、慢的发送者与 Caddy 早答都只记、进 v0.2；附件列表的重读能稳定让保存失败才修；`apitest.Load` 不主动做；先做有预算的与部署文档要的，其余推算（4.7；设计审查 A 改）。
- **318 "目标硬件"**以本机为参照、EPYC 7763 为慢的主机的代表（I2-D2）；4.7.1 逐行写判在哪台机器上：有硬期限的，有 CI 上的实数时由实数定，没有时才乘记下的最大倍数（2.33 或 4.6）、写"推断"；B5 只用 M8 写下的规则在本机判，最低配置的硬期限用 7763 上的实数加 `parse_max_wait` 判，规则没过不自动降常量，两种做法列给主持者与负责人定；部署文档的最低配置是 7763 的那一档（4.7.1、4.7.3、4.7.5；设计审查 A 改，修订核对第 1 轮 A 的 I4 再改）。
- **319 测量的设施进仓库、不进 CI**；内存从外部采样，不加 pprof 或 metrics 端点；只写有预算的几项要的驱动（I2-D4、D5、D14，4.7.4；设计审查 A 改）。
- **320 部署文档**：`deploy/README.md` 作运维手册，加 `deploy/compose.yaml` 与它的检查；`compose.yaml` 的 `migrate` 在 profile 里、备份之后显式跑；要求写 PostgreSQL 18.7 或更新与硬件下限；根 README 缩成入口，删开发版的说明之前先挪走对新装也成立的数；开发版的升级说明看负责人的回答；单实例写明原因、时钟一条不适用；Caddy 的 502 只写代价、推荐 nginx；小项都写进去（I1-D13、D15–D18，4.8；设计审查 B、C 改）。
- **321 版本号**：tag 经 ldflags 注入，四份默认值有守卫，MCP 的版本由短测试守住，发布之后 `main` 是 `0.1.1-dev`；不打 `server/v0.1.0`，不宣传 `go install`（I1-D8，4.9；设计审查 B 补）。
- **322 发布流水线**（I1-D1–D7、D14，4.10；设计审查 B 改）：`ghcr.io/open-nerve/nervewiki`，只打确切的版本号，不打 `0.1` 与 `latest`；tag 触发；闸门只认 `ci.yml`、加 `make vuln` 与 `CHANGELOG.md` 的一节；两种原生 runner；演练是只读的单独作业，分支与 `main` 的每次推送都跑、不加路径条件；先打 `sha-<提交>`，证明与签名之后才复制出版本号的 tag；一时的失败在同一次运行上重跑；发布之后的核对在两种架构上；`actions/attest` 的来源证明与 SBOM 加 cosign keyless；前端另出一份 SBOM；镜像与 Release 带第三方许可证；Release 不附二进制、没有就建有就改、附件 `upload --clobber`，`v0.1.0` 不标 pre-release；先推 rc；不用 `docker/metadata-action`。`ci-wait.sh` 按工作流取已在 `b54cd679`。
- **325 T1、H4 写明、功能进 v0.2**（I3 第 7 节第 9、11 条）：结束的任务行不清理；永久删除写明与变通，界面写一句（4.5、4.8）。初稿里同一条的 T2 改为在 v0.1 修（负责人第 5 问，决定 336）。

### 2026-11-14：00-M12 的设计者的取舍 314、323、324、326–331（主持者的决定，负责人可以改判）

- **314 MCP `log` 的 `path`**：省略与 `"/"` 是整本；`""` 答字段错误，文字写"省略它就是整本"。`index` 不动：它的 `parent:"/"` 是根，写在工具说明里，`list` 同样。理由："path is required"是误导的文字（30.4，B）；`"/"` 在 `list` 与 `index.parent` 里都是根，`log` 跟它们一样（4.6.4；设计审查 A 改，初稿让 `"/"` 一律答错，拿掉了 `index` 已有的写法）。
- **323 发布说明是仓库根的 `CHANGELOG.md`，中文**。理由：`docs/` 按约定只放设计与过程文档；README 是中文；Release 的说明取它的一节，工作流在后面接摘要与校验命令（摘要在推送之前不知道）。rc 取去掉 `-rc.N` 的那一节，拼说明的脚本在本机对着假输入核对；日期写在 `v0.1.0` 的 tag 指的提交里。另写 `SECURITY.md`（4.10；设计审查 B 补）。
- **324 第 2 步的点头盖住 rc 的重推**：只改发布流水线、部署文档或发布说明的 rc.N，同一次点头盖住；改了产品代码的再问；收尾之后由闸门的 `make vuln` 发现、要升依赖的算改了产品代码，为此加的 `audit.ignore` 算改了产品的依赖配置；一时的失败在同一次运行上重跑，不算重推。GHCR 设为公开在 rc 核对之后、`v0.1.0` 之前（4.11）。理由：rc 的用处就是找出流水线的问题，每次重推都问会让负责人的一次点头变成几次；产品代码的改动改变了被发布的东西，要再问。
- **326 以后的移交与目录**：v0.2 的移交放在 `docs/v0.2/handoffs/`，五份按主题；v0.1.1 的文档放 `docs/v0.1.1/`，与 `docs/v0.1/` 并列，工作由负责人另行批准开工，M12 只写它的移交（4.13，第 7 节"写给后面的 M"；设计审查 C 改，初稿是 `docs/v0.1/v0.1.1/`）。理由：文档约定把移交放进目标的 `handoffs/`，v0.2 还没有 M 的划分；按主题分，v0.2 的设计者按主题读；版本目录不套版本目录。
- **327 人工验收的入口**：`docs/v0.1/M12-release/manual/README.md` 汇总，链到输入法清单、MCP 的清单与新写的读屏清单（4.12）。理由：先发布、后验收，负责人要一个地方看全部欠着的验收。
- **328 新的整个程序的测试点名进第七、第八份**；第四、第五份不加；名字避开第二份的模式，避不开时第二份的 `-skip` 加第七份的模式（第 7 节"分份"；设计审查 A 补）。理由：第五份余量 25 秒，第四份是补集、名字一不留神就落进去；第二份不跳过第七份。
- **329 页面树的三条界线**：T1 冷打开到树的答复已到、视口里的行已画出 ≤ 1 秒；T2 别人在视口里新建一页，新的一行 ≤ 1.5 秒出现、最长的长任务 ≤ 200 ms；T3 展开 1,000 个孩子到视口里的孩子画出 ≤ 500 ms；三种形状；回环、不降速；两轮的较大中位数（4.7.2）。理由：终点要在"只渲染看得见的行"的改法之后同样量得了；量在回环上，1 秒照 I2 的推荐；200 ms 是用户感到卡顿的量级；T3 盖住"平的大文件夹"那一种渲染（设计审查 A 改，初稿 T1 是 1.5 秒、终点是全部行进入 DOM）。
- **330 P4 分成设施、浏览器、服务端三个 Step，页面树先量**（第 7 节）。理由：页面树的结论挡 P2/S6；服务端的实测要等 P3 改完撤销、日志、标题键与任务的入口；设施是代码、实测是文档，分开审。
- **331 网页上显示 MCP 地址进 v0.2**。负责人 M9 的决定是"显示 MCP 地址的网页界面等打磨"（`04-owner-decisions-and-preferences.md` 第 5 节）。打磨分级之后它是 C（31.1、47），照负责人第 1 问进 v0.2，发布说明列为已知限制。负责人若要它在 v0.1，是 P2 的一个小 Step。

### 2026-11-15：设计审查之后新加的主持者的决定 332–336（负责人可以改判）

- **332 发布期的改动走 `m12-release` 分支**（设计审查 C 的 M6）：收尾合并之后为发布做的改动（rc 的修复、`CHANGELOG.md` 的日期、`0.1.1-dev`）在 `m12-release` 上做；主持者读 diff、看分支的 CI，`--no-ff` 合并，`main` 的 CI 绿之后才推 tag；收尾记录的"发布"一节逐个列出。只改流水线的 rc.N 照决定 324 不再问负责人；改了产品代码的，照 324 再问（4.11）。
- **333 移交的处理进展随设计定稿提交，写到 Step**（设计审查 C 的 Q1）：与 00-M12、审查记录与定稿时的总体设计修订同一个提交；初稿之前的单独提交不要了（第 7 节"设计定稿"）。
- **334 收尾记录列出 P1/S1 之后 `main` 的 e2e 红过的每一次与总次数**，不逐次列全部（设计审查 C 的 Q3 与过度设计 O2）。
- **335 MCP 调用超时的文字在 P3/S1 改，`batch` 不设单独的期限**（设计审查 C 的 I2；移交 M4-performance 第 22 条之后的补记 `:490`）：通用的"the call ran out of time"改为说明期限已到、什么也没有写、拆小再试，是误导的文字（B）；单独的期限进已知限制与 v0.2；硬件下限写进部署文档（4.6.4、4.8）。
- **336 T2、T4 在 v0.1 修**（负责人 2026-11-14 第 5 问）：T2 是按人的常量上限，答一个新的码（例如 `transfer.too_many_jobs`，状态码推荐 429，P3/S3 照契约与码的约定定），有自己的两种语言的文字，网页照它显示，全局的 503 `server_busy` 照旧；T4 是启动时把上一个进程在 River 取到之后、开始之前停掉的任务记成失败，River 那一边照 `Job.DiscardRunning` 的先例、注意它的次序。都在 P3/S3（4.6.6；修订核对第 1 轮 A 的 M9、N12，C 的 M10 改，第一次修订是"同样答 503"）。

### 2026-11-15：P1 的设计审查之后的取舍 337–352（主持者的决定，负责人可以改判）

337–350 是 P1 初稿的十四条（P1-1 到 P1-14，P1-k 是 336 + k），设计审查之后照次序编号，344、347、349 照审查改写；351、352 是审查之后新定的（P1-15、P1-16）。每条在 [P1 设计](01-P1-foundation.md) 2.3，审查里的出处在 [P1 审查记录](reviews/P1-design-review.md)"主持者的决定"一节。

- **337 e2e 换文档的帮手是函数 `goto(page, url, options)`、`reload(page, options)`，在 `e2e/fixtures/navigation.ts`，不是 `page` 的方法**（P1-1；P1 设计 3.2）：`anotherPage`、`anotherTab` 与故事自己开的标签页也要它，包一层 `Page` 会让 Playwright 的类型与 `page.context()` 的页面对不上。
- **338 在途的请求只算 `/api/v0/` 之下的 fetch 与 XHR，事件流除外**（P1-2；P1 设计 3.2）：图片、媒体与附件的内容打断不了续期，媒体的请求还可能一直开着。
- **339 帮手等三样：在途的接口请求答完、这个文档的第一次续期已答（文档的 `performance` 里有 `/api/v0/auth/refresh` 一条）、续期的锁空着（没有 Web Locks 时看租约）**（P1-3；设计审查 A-I1、C-I1 之后每一样各有 A15 的一格只在它去掉时失败；P1 设计 3.2、3.6）。
- **340 帮手等 10 秒，然后失败并写出在等什么；在页面里读的那一次与剩下的时间赛跑**（P1-4；设计审查 C-N2；P1 设计 3.2）：比续期自己的 8 秒长、比测试的 30 秒短；Chromium 在下一个文档还没提交时 `page.evaluate` 不返回。
- **341 没被监视的页面，帮手直接失败**（P1-5；P1 设计 3.2）：在第一次调用时才开始监视，那时已在途的请求看不到。
- **342 落到 `/sign-in` 只写进失败时的页面状态，不让通过的测试失败**（P1-6；P1 设计 3.3）：有意走到登录页的故事很多；收尾照 CI 注解里 `sent back to sign in:` 一行数（决定 334）。
- **343 门禁是读源码的 Node 脚本 `tools/e2e-navigation/check.mjs`，进 `make lint-web`，每次先在内置的样例上证明它看得见违例**（P1-7；P1 设计 3.5）：oxlint 的 `no-restricted-properties` 禁得了 `.goto`，却数不了逃生口、核不了表。
- **344 门禁按文字找，注释与字符串也算；帮手的模块整个例外**（P1-8，照设计审查 A-M1、C-M2 改写；P1 设计 3.5）：oxfmt 把帮手的报错信息改成单引号之后，源码里就有逃生口的文字；不引入 TypeScript 的解析器。
- **345 A15 是身份分组里的新故事，在三种浏览器里跑，第 3 格例外（351）**（P1-9；P1 设计 3.6）：它是用户看得到的行为，又是帮手的对照；不放进 A5，A5 的第二格的控制台期望是 Chromium 的原文。
- **346 `make vuln`、`make outdated` 在 GitHub 的 runner 之外拒绝运行**（P1-10；设计审查 A-M3、B-N3 改了它的变异；P1 设计 3.12）：本机不往外发依赖清单（决定 308），拒绝比写在注释里可靠。
- **347 Go 版本的守卫放在 `platform/buildinfo`；P5/S1 的版本号守卫放在哪里由 P5/S1 的设计定，P1 设计建议同一个包**（P1-11，照设计审查 A-M6 改写；P1 设计 3.9、第 9 节）。
- **348 Dockerfile 构建时的版本断言放在 server 阶段 `FROM` 之后的第一行**（P1-12；P1 设计 3.9）：在 `go mod download` 之前，摘要不对时最早失败。
- **349 第 50 条只在网络等待真的发生时打日志，S1 先做一次正向对照**（P1-13，照设计审查 A-M5 补；P1 设计 3.7）：有了正向对照，CI 上没看到这一行才算没触发。
- **350 `search_guard_test.go:13` 的 "Go 1.27.1" 不改**（P1-14；P1 设计 2.3）：它记的是 M10 校准那一刻的环境，不是要求。
- **351 A15 的第 3 格（第一次续期发出之前离开）只在 Chromium 与 Firefox 跑，WebKit 跳过并写明理由；第 4 格三种都跑，WebKit 里只证明帮手等第一次续期**（P1-15；设计审查 C-I1；P1 设计 3.6）：WebKit 在请求下一个文档之后，旧文档的请求一个也发不出去，没有这个窗口（审查 C 的探针 B、D 与修订时的原型）。
- **352 S2 合并之后，`main` 的第五份第一次落在 EPYC 7763 上时，记下它的 Test 一步，与收尾之后的 425 秒比；两次落在 7763 上的平均过 450 秒时，照 06-environment 第 4 节挪份；不为等 7763 拖住 S2 的合并**（P1-16；设计审查 A-Q1，主持者定；P1 设计 7.4、7.5、第 8 节第 1 条）：照 7.5 写明、不硬比不同 CPU 的数。

### 对主持者说明的核实

- **第 302 条的修法**：读 `page/domain/revert.go` 与 `page/app/rollback.go:70-125`，形状与 I3、移交的描述一致。初稿没有造过、没有跑。设计审查 A 在 domain 与整个程序上复现了形状，指出先失败的是 `rootHolds`、一次恢复只放回一个回收站项，并在原型上跑通了更小的改法；修订照它改（4.6.1）。
- **heavy.sh 的锁**：`5e8af5b5` 的 `docs/v0.1/claude_to_codex/tools/heavy.sh:8` 是 `L=${NW_HEAVY_LOCK:-${TMPDIR:-/tmp}/nervewiki-heavy.lock}`，与说明一致；已由主持者改好、提交（`8c247237`）。
- **`ci-wait.sh`**：`5e8af5b5` 的 `:13` 按提交取第一个运行，与审查 B 说的一致；已由主持者改为按工作流取、提交（`b54cd679`）。
- **第四份 413.5 秒**：这是收尾之前那两次 `main`（`07abd0f7`、`9e787bab`，都在 7763 上）的平均。收尾之后的两次（`05dac827`、`a057b6ad`）第四份是 339、285 秒（平均 312，都不在 7763 上）；第五份 427、423 秒（平均 425，都在 7763 上）（[M11 收尾审查](../M11-upgrade/reviews/M11-closeout-review.md)"守卫与分份的数"一节的表）。第 7 节"分份"与第 10 节照此写。
- **`TestRevertingARollback…` 在第八份**：`Makefile:296`、`:328` 都点了它的名。
- **Playwright 不重试**：`e2e/playwright.config.ts` 没有 `retries`。
- **T2、T4 的代码**：`transfer/app/start.go:212-227` 的 `room` 只比全局的计数；`transfer/app/maintain.go:84-95` 的 `AtStart` 只失败 `running` 的行，`Run` 的注释写明了 River 取到之后的那个窗口（4.6.6）。
- **MCP 超时的文字**：`platform/httpserver/mcpserver/errors.go:77` 的 `internalDeadline`，`:101` 用它（4.6.4）。
- **调查之外另查到的**：
  - `platform/httpserver/mcpserver/edge.go:106` 写着 go1.27.1 里 `encoding/json/v2` 在 GOEXPERIMENT 之后，升工具链时要核对（4.3）。
  - `server/tools/go.mod` 的注释把 kin-openapi 等钉在 oapi-codegen 的要求上，所以 `govulncheck` 不进那里，用 `go run …@版本`（4.3）。
  - 事件流已有 `reset` 帧，按凭证的上限沿用它（4.6.5；13.1 第 30 条）。
- 其余（三份调查的事实与推荐）照做。三份设计审查找出的做不到或不对的地方，照[审查记录](reviews/M12-design-review.md)改了。抽查的行号见附录 B。

### 需要负责人确认

没有挡住开工的问题。下面几项各写"什么时候要"：

| # | 事项 | 推荐 | 什么时候要 |
|---|---|---|---|
| 1 | 推 `v0.1.0-rc.1` 与 `v0.1.0` 两个 tag（4.11 第 2 步） | 照 4.11 | 收尾合并、`main` 的 CI 绿之后，推 rc 之前 |
| 2 | 组织允许公开的容器包；"允许的 actions"包括 `docker/*`、`sigstore/cosign-installer`、`anchore/sbom-action` 这几个第三方的（`ci.yml` 今天只用 GitHub 自己的）；组织的 Actions 设置允许工作流申请 `packages: write`、`id-token: write`、`attestations: write`（I1-D19；审查 B 指出最后一项可能本来不限，照实核对） | 照做 | 推 rc 之前（P5/S2 合并之后就可以） |
| 3 | 给 `v*` 的 tag 加规则集，只有负责人与主持者能推（I1-D19） | 照做 | 推 rc 之前 |
| 4 | 开私密的漏洞报告（`SECURITY.md` 指向它）；Dependabot alerts 可以开，只管 Go 与 actions：GitHub 的依赖图读不出 pnpm 12 的锁文件，npm 一侧靠 `make vuln` 里的 `pnpm audit --prod`（I1-D19；设计审查 B 改） | 照做 | 私密的漏洞报告在 P5/S3 写 `SECURITY.md` 之前；alerts 随时 |
| 5 | 把 GHCR 的包设为公开，**不能撤回**（I1-D19） | rc 核对之后 | 4.11 第 5 步：rc 核对之后、推 `v0.1.0` 之前 |
| 6 | Dependabot 的版本更新 PR 要不要开（移交 M0-P6 第 4 条写明由负责人定） | 不开；发布之前在 runner 上跑 `make outdated`（4.3） | M12 收尾之前（写 v0.2 的移交时） |
| 7 | 有没有 v0.1.0 之前就在跑、要升级到 v0.1.0 的实例（I1-D13） | — | P4/S3 开始之前：没有就不写开发版的附录、不量批次 H |
| 8 | 输入法清单（M4–M8 的 21 步）是否同样在发布之后验收（4.12） | 是：同 2026-10-13 的原话 | 写发布说明之前（P5/S3） |
| 9 | 罕见的 A 类（H1、24.1、T4、T2）怎样处理 | **已确认（2026-11-14）**：T2、T4 在 v0.1 修，H1、24.1 进 v0.2（第 5 问） | — |
| 10 | 只从 ARIA 推断的九项怎样处理 | **已确认（2026-11-14）**：并进发布之后的读屏验收（第 6 问）；其中 49 与 24.3 同一种问题，留在 v0.1 | — |

改了负责人原话或负责人可能想自己定的，在 M12 的汇报里列出：315（流数的上限与重置）、324（rc 的重推不再问）、331（MCP 地址进 v0.2）、312 里 49 留在 v0.1（比第 6 问所选多做一项）、322 里不打 `0.1`（I1 推荐打）。

## 12. Phase 进度表

| P | Phase 文档 | 状态 | 审查 | 合并 |
|---|---|---|---|---|
| P1 地基（S1、S2） | [01-P1](01-P1-foundation.md) | 进行中（设计已审查） | [设计](reviews/P1-design-review.md) | — |
| P2 网页的打磨（S1–S6） | — | 未开始 | — | — |
| P3 服务端的正确性与历史（S1–S3） | — | 未开始 | — | — |
| P4 性能与内存实测（S1–S3，条件的 S4 起） | — | 未开始 | — | — |
| P5 部署文档与发布流水线（S1–S3） | — | 未开始 | — | — |
| 收尾与发布 | — | 未开始 | — | — |

## 13. 变更记录

| 日期 | 变更 | 依据 |
|---|---|---|
| 2026-11-14 | 初稿：五个 Phase、17 个 Step，另有条件的修复；不加接口操作与故事；页面树的阈值与三条界线；第 302 条在 M12 修；发布的顺序与对外的步骤；主持者的决定 M12-1–M12-10、M12-12–M12-19、M12-22，设计者的取舍 M12-11、M12-20、M12-21、M12-23–M12-28 | 负责人 2026-11-13 的批准与四个回答；三份只读调查（基点 `5e8af5b5`）；六份 `open` 的移交；主持者的说明 |
| 2026-11-15 | 第一次修订。决定编号从 304 起，初稿的 M12-1–M12-28 依次是 304–331，新加 332–336。302 改为"本身是回收站根的节点自成一顶"，后台撤销的 `needed` 随之，性质测试扩现成的两个；流数按凭证计数、账户的硬上限答 429，都是常量；30.2 改为整个名字的判断；`log` 的 `"/"` 是整本、`index` 不动；GBK 先读 0x7075，Shift-JIS 等的误读列为已知限制；T2、T4 在 P3/S3 修，H1、24.1 进 v0.2；读屏推断的八项随发布之后的读屏验收，49 留在 P2/S1；MCP 超时的文字在 P3/S1 改；页面树 T1 改终点、1 秒；实测逐行写判在哪台机器上，范围缩小；`make vuln` 加 npm 一侧、移到每天的 `vuln.yml`；`ci-wait.sh` 与闸门按工作流；演练只读、`main` 每次推送都跑；删本机 registry 的演练与 `docker/metadata-action`；只打 `0.1.0`；版本号的 tag 在签名之后打，一时的失败重跑；e2e 的帮手只管 `goto` 与 `reload`、有逃生口；发布期的改动走 `m12-release`；移交的处理进展随定稿提交；v0.1.1 的文档在 `docs/v0.1.1/`；第 11 节换成原问 | [设计审查](reviews/M12-design-review.md)（A 3I 8M 8N 1Q、B 3I 12M 7N 1Q、C 3I 8M 7N 3Q）；负责人 2026-11-14 的两个回答；主持者的第一次修订说明 |
| 2026-11-15 | 第二次修订。B5 只用 M8 写下的规则在本机判，最低配置的硬期限用 7763 上的实数加 `parse_max_wait`，规则没过不自动降常量；有硬期限的各行有实数时由实数定。T2 的拒绝用一个新的码与它自己的文字。24.1 进 `M12-transfer.md`，已知限制逐项有去处。`vuln.yml` 加 `push` 触发；`pnpm audit` 闸门只挡 `high`，修不了的写进 `audit.ignore`；每个 Step 合并之前看它的结论。演练在每次推送上都跑。`ci-wait.sh` 写成 P1 之前已改好（`b54cd679`）。reset 加原因 `superseded`；凭证与账户上限的先后；T4 照 `Job.DiscardRunning`；帮手所在的模块不受门禁；Release 的附件 `--clobber`；v0.1.1 的移交改名 `M12-after-release.md`，收 `vuln.yml` 的两件 | [审查记录](reviews/M12-design-review.md)"修订核对第 1 轮"（A 新 1I 1M 4N，B 新 4M 2N，C 新 2M 1N）；主持者的第二次修订说明 |
| 2026-11-15 | 修订核对第二轮之后的修订（主持者）：实数要与预算同一规模，B7 在 10 万页上只能推断（A-N13）；升级的回滚不受按人的上限拒绝（A-N14）；B5 的硬期限只对请求一路的守卫核，`MaxJobItems` 只用规则一层（A-N15）；查 `vuln.yml` 结论的 `gh run list` 加 `--branch main`（B-N11）；`vuln.yml` 的 `push` 路径加 `pnpm-workspace.yaml` 与 `Makefile`（B-N12） | [审查记录](reviews/M12-design-review.md)"修订核对第 2 轮"（A 新 3N，B 新 3N，C 没有新发现） |
| 2026-11-15 | 设计定稿：第 7 节"总体设计的修订"第 1–3、14 条以脚本落进 `v0.1-design.md`（每段旧文字在替换时恰好出现一次），其中 12.2 的 M12 一行是中、12.3 加 M12 的依赖、12.6 的 M12 一行是进行中、第 15 节加 M12 开工一行；第 16 条的前一半落进 `docs/README.md`（M12 的提交说明一句），目录结构的两行在收尾时加；其余各条随实现它们的 Step 或收尾改；六份移交各加"处理进展"，写到 Step（主持者的决定 333）；`06-environment.md` 里 `ci-wait.sh` 那一句改为"取最后建的那一次"（修订核对第 2 轮 B 的 N10）；第 7 节第 11、13 条引的链接改成不在本文件里成为断链的写法（`check_links.py`）；状态改为进行中 | 设计审查与两轮修订核对没有 Important 与 Minor（[审查记录](reviews/M12-design-review.md)）；`02-workflow.md` 第 1 节第 8 步 |
| 2026-11-15 | P1 设计已审查，S1 开工；P1 设计 2.1 的修订落进本文：不做跨文档的后退；TR2 不用逃生口，逃生口只有 A15 的两格；帮手另等这个文档的第一次续期与续期的锁，各由 A15 的一格定下；门禁管整个 `e2e/`、按表核对逃生口、对帮手的模块整个例外；`pageWatch` 在 `browser.ts` 的 `watchPage`；对照测试是故事 A15；S1 另改 `tools/` 与构建；9.6 的变异照 P1 设计 7.2；`govulncheck@v1.9.0`；README 的 Go 版本进守卫、断言带一句错误信息；`make outdated` 也在改了依赖清单的推送上跑、`continue-on-error`；`vuln.yml` 的 `push` 只在分支上、路径加 `tools/outdated.sh`（2.1 第 10 条撤回）。第 11 节加 P1 的取舍 337–352 | [P1 设计](01-P1-foundation.md)；[P1 审查记录](reviews/P1-design-review.md)（A 2I 6M 10N 1Q、B 1M 4N、C 1I 3M 4N；两轮修订核对新 A 1N、C 2M 2N，都已改，没有 Important 与 Minor） |

---

## 附录 A：打磨的逐项去处

编号是 [M5-polish](handoffs/M5-polish.md) 的条目与 I3 拆出的子项；分类照 I3 第 9 节，例外在 4.5。

### 进 v0.1 的（A 31 项、第 48 条、B 81 项；任务的 T2、T4 在 P3/S3，4.6.6）

| Step | A | B | 一起做的 C、D |
|---|---|---|---|
| P2/S1 焦点与共用组件 | 7 | 6.3、9.1、11.7、12.3、16.1、18.6、19.3、21.14、22.17、23.14、23.16、24.3、26.4、28.4、28.5、35.2、38.1、38.3、49（与 24.3 同一个改法） | C：23.17、28.2（随 23.16） |
| P2/S2 文字 | — | 6.1、9.5、15.3、16.6、19.1、21.8、21.11、22.11、22.12、22.13、22.16、23.1、23.3、23.4、23.11、23.12、23.13、24.7、26.11、26.13、26.14、35.1、38.6、38.8、40、41.1、41.2、41.3 | H4 的界面一句（移交 M8-history 第 4 条） |
| P2/S3 375 px、`bdi` 与对比度 | — | 12.4、26.9、26.12、26.17、32.1、32.2、32.4、35.8、38.10、38.11、38.12 | D：32.3；M：33.1 的排查 |
| P2/S4 阅读视图 | 6.2、11.1、17.1、17.8、48（C 改判为 A） | 3、8.1、8.2、12.2、29 | C：11.6 的失焦部分；D：14d |
| P2/S5 编辑器、附件、上传、粘贴与导入导出 | 12.8、15.1、15.2、16.3、16.4b、16.11、17.2b、17.4、17.6、17.7、18.1、18.2、19.4、20.3、38.7 | 16.4c、16.10、20.1、20.2、28.1、38.9 | — |
| P2/S6 版本、回收站、历史、目录、后台任务与搜索 | 2、12.12、21.7、22.14、23.10、23.19、24.2、35.3 | 21.2、21.3、21.10、22.2、22.19、24.9、26.2 的前一半、26.5、26.18a、27.2 | D：24.8、24.10、26.15 |
| P3/S1 小的正确性 | 10.1、10.2、25.1（与 25.2） | 30.2、30.4 | C：30.5（与 30.4 定一种约定） |

### 发布之后的人工验收（4.12）

- **M 类**（14 项）：5、8.4、11.8、12.11、13.3、16.12、17.9、18.5、18.8、26.18b、27.3、33.1（P2/S3 先排查）、33.2、39。
- **只从 ARIA 推断的 B**（8 项）：16.8、18.4a、21.15、21.16、22.10、26.1、26.2 的后一半、28.3。并进读屏验收（负责人 2026-11-14 第 6 问），确认有问题在 v0.1.1 修。初稿的第九项 49 移回 P2/S1（4.5）。

### 已知限制，进 v0.2

- **罕见、改动大的 A**：24.1 进 v0.2 的 `M12-transfer.md`，历史的 H1 进 `M12-history.md`（负责人 2026-11-14 第 5 问）。初稿并列的 T4 改在 P3/S3 修。
- **C 类 87 项与 3、12.2、11.6 的其余**（v0.2 的 `M12-polish.md`）：
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
- `Makefile:12-14`（`:12` 是注释，`VERSION ?= 0.1.0-dev` 在 `:13`；ldflags）、`:85-90`（`IMAGE`、`make image` 与 worktree 的检查）、`:263`（第五份）、`:296`、`:328`（第七、第八份点名 `TestRevertingARollback…`）。
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

第一次修订另核对的（同在 `5e8af5b5`；审查者核对过的照审查记录，不重列）：

- `transfer/app/start.go:204-227`（`room`）、`transfer/domain/errors.go:49-54`（`QueueRetry`、`ErrQueueFull`）、`transfer/app/maintain.go:84-120`（`AtStart`、`Run`、`lost`）、`server/configs/config.yaml:148`（`max_queued: 20`）。
- `platform/httpserver/mcpserver/errors.go:72-78`、`:101`（`internalDeadline`）。
- `shared/actor.go:15-23`（`SessionID`、`APITokenID`、`JobID`）；`events/domain/frame.go:10-11`；`api/modules/events.yaml:100`（已有 `rate_limited`）。
- `page/app/jobs.go:575-585`（`needed` 的注释）；`page/app/rollback_test.go:560`；`page/app/revert_job_property_test.go:64`；`page/domain/rollback.go:58-69`（`PackFreeSubtrees`）。
- `docs/v0.1/M12-release/handoffs/M4-performance.md:488-492`（第 22 条之后的补记）。
- `v0.1-design.md:1178`（13.1 第 32 条）、`:1184`（13.2 第 1 条）、`:1229`（13.2 第 26 条），13.4 第 8、9 条的三段新锚点，各恰好出现一次。
- Go 1.27.1：`06-environment.md:10`、`:52`，`05-lessons.md:89`，`03-project-background.md:15`。
- `README.md:43`、`:193`；`e2e/fixtures/wiki-health.ts:157`；`Makefile:13`、`:342`；`knip.jsonc` 没有 e2e 的入口。
- heavy.sh 的锁已在 `8c247237`。

第二次修订另核对的（同在 `5e8af5b5`）：

- `api/modules/events.yaml:239-243`（reset 的 `reason` 是必填的枚举，六个值）。
- `platform/jobs/jobs.go:30-39`（`DiscardRunning`）、`:99-107`（discard 排在任务自己的 `Start` 之前）。
- 开任务的四个操作都在 `api/modules/transfer.yaml`（导出、导入、`trash-restores`、`reverts`）；各模块的 `adapter/mcp` 不调 transfer。
- `transfer/app/start_background.go:182`、`:200` 与 `start_import.go:111`、`:269` 也经 `room`。
- `ci-wait.sh` 已在 `b54cd679` 改好。
