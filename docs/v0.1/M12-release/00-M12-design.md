# M12 发布：总设计

| 项 | 内容 |
|---|---|
| 里程碑 | M12 发布（`M12-release`） |
| 日期 | 2026-11-14（初稿）；2026-11-15（第一次、第二次修订；定稿） |
| 状态 | 进行中（P1、P2、P3 完成；P2–P5 的设计都已审查；P4/S1、P4/S2、P4/S3、P4/S4、P4/S5、P4/S6、P5/S1、P5/S2 已合并；P4/S2 量得页面树平的形状与编辑器的解析（B9）超，页面树由 P4/S4 修好（三种形状都过；P2/S6 排在它之后），P4/S5 修好了 B9（在原规模上再量过了）；P4/S3 量得 B4 的树写、B5 的 `MaxJobItems` 与 B2 的病态正文超，负责人 2026-11-21 第 13 问定了：条件的 P4/S6（`MaxJobItems` 降到 45,000）与 P4/S7（B4 的树写）在 v0.1 修，B2 改判、B5 的硬期限写明余量；P4/S7 的设计已审查；P5/S3 可以开工；[审查记录](reviews/M12-design-review.md)；总体设计照第 7 节"总体设计的修订"中标"设计定稿时"的几条改过） |
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
   - `TestRevertingARollbackConflictsUnderANewFolderHoldingBaselineAndNewPages`（`bootstrap/upgrade_rollback_revert_test.go:30`）的 nested 一格（`:87`）翻过来：请求的撤销与后台的撤销各在一本笔记本上成功，树回到回滚之前。测试随之改名，第七、第八份的模式（`Makefile:296`、`:328`；`194ca350` 上是 `:321`、`:365`）一并改。
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
   - 4.7.1 表有预算的每一行在 P4 的记录里有：机器、规模、种子、两轮的数、在哪台机器上判（本机、CI 的实数，或乘了哪个比）、对照预算的结论（过、超）。B5 的规则一层没过、或硬期限一层的推断过了 15 秒的，记录写明列给主持者与负责人定的两种做法与他们的决定。只量的各项有数，或写明"推算"与它依据的数。P4/S3 的证据在 [P4 设计](04-P4-performance.md) 10.3 与 [S3 的审查记录](reviews/P4-S3-server-measures-review.md)；规则一层的 `MaxJobItems` 没过与硬期限的第一行推断过 15 秒，同 B4、B2 一起列给负责人，他的决定在第 11 节第 13 问。P4/S6 把 `MaxJobItems` 降到 45,000，在原规模上两轮再量过了（至多 3,683 ms，过；[P4 设计](04-P4-performance.md) 10.4、[S6 的审查记录](reviews/P4-S6-max-job-items-review.md)）。
   - 页面树的三条界线在量之前写进 P4 的设计，量之后逐条对照。
   - 每一个超出的已修，带缩小规模的守卫；或经负责人改判。其余写进已知限制与 v0.2 的移交。
7. **部署文档**（P5/S3）：
   - `deploy/README.md` 覆盖第 7 节"移交的落实"里每一条部署项，逐条对照表在 S3 的记录里。
   - `deploy/compose.yaml` 由 `make deploy-check` 核对，CI 的 `image` 一份跑它。
   - 照 `deploy/README.md` 用 `deploy/compose.yaml` 在本机做过三遍：从零装一遍；改镜像的 tag 升一次级（备份之后 `docker compose run --rm migrate`）；备份再恢复一遍。记录写命令与结果。包设为公开之后，用 GHCR 的镜像再装一遍（4.11 第 5 步）。
8. **版本号**（P5/S1、发布）：
   - Makefile、buildinfo 与 Dockerfile 两个 `ARG` 的默认值一致，由短测试守住，有反向对照。
   - `0.1.0` 的镜像里 `nervewiki version` 与 `/api/v0/instance` 都是 `0.1.0`：两种架构上的 image-smoke 核对。
   - MCP 的 `serverInfo.version` 取 `buildinfo.Get().Version`（`bootstrap/mcp.go:90`），一个盖了版本号的短测试断言两者相等，有反向对照（P5/S1；P5 设计 2.1 第 6 条）。
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
11. **CI**：每个 Step 合并之后，两次 `main` 的组合根每一份的 Test 一步平均在 450 秒之下（06-environment 第 4 节）；e2e 的 E2E 一步在它自己 18 分钟的时限之内（`.github/workflows/ci.yml:361`），超过时这一步失败、作业红，不是被取消。原来这一条的界是作业的 20 分钟，含前几步与装浏览器；装浏览器一步的 30 分钟与作业的 55 分钟不是这一条的界（旁支 `m12-e2e-browsers-cache`，决定 599；`TestTheE2EStepHasALimitOfItsOwn` 守着）。`release.yml` 的演练与 `vuln.yml` 不在组合根里，不算进各份。

## 4. 关键决定

决定的编号从 304 起（303 是 M11 收尾用掉的，`00-M11-design.md:950`）。初稿的临时号 `M12-n` 与正式号的对照在[审查记录](reviews/M12-design-review.md)。设计审查改过的决定标"（设计审查 X 改）"。

### 4.1 次序：地基先行，实测在代码定下之后

- **P1 先做**：
  - e2e 的 `goto` 挡发布。发布要 `main` 的 CI 为绿，而 Playwright 不重试（`e2e/playwright.config.ts` 没有 `retries`）。M11 收尾之后 `main` 的 run 43339247136 里，LW4 落到了登录页，与 M10 查明的 LW6 同形；是不是续期没有核实（[M11 收尾审查](../M11-upgrade/reviews/M11-closeout-review.md)"门禁"一节；移交 M5-polish 第 51 条）。它先做，后面每个 Step 的 CI 才稳。
  - 工具链与 PostgreSQL 的升级随后。之后的实测、计划测试与成本守卫都在最终的工具链与数据库上跑（I1-D12）。
- **P2、P3、P4/S1、P4/S2、P5/S1 可以并行**：它们改的部件除翻译文件外不重叠，P3/S3 的网页部分另有几处与 P2 重叠（第 7 节"次序与并行"）。重的运行照旧经 `heavy.sh` 串行。
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
  - **不进每次推送的 CI**：Go 的小版本约每月一个，多数带安全修复；`govulncheck` 找到就非 0 退出。放进必绿的 `short` 一份，下一个安全版本一出，每个 Step 都红，连只改网页的 Step 也合并不了。`make vuln` 放在一个单独的工作流 `.github/workflows/vuln.yml` 里：`schedule` 每天一次，加 `workflow_dispatch`，另加一个 `push` 触发，`paths` 只写它自己与依赖清单（`.github/workflows/vuln.yml`、`server/go.mod`、`server/go.sum`、`pnpm-lock.yaml`、`pnpm-workspace.yaml`，以及定义 `make vuln` 的 `Makefile`、`make outdated` 的 `tools/outdated.sh`，与以 `packageManager` 定了跑 `pnpm audit` 的 pnpm 的根 `package.json`；修订核对第 2 轮 B 的 N12，P1 设计 2.1 第 13 条；`package.json` 是 P1/S2 审查 C 的 N2，P1 设计 3.16 第 13 条），只在分支的推送上（`branches: ["**"]`），tag 不触发。`workflow_dispatch` 与 `schedule` 只对已在默认分支上的工作流生效，引入它的 P1/S2 分支与升依赖的分支靠 `push` 跑（修订核对第 1 轮 B 的 M13）。发布的闸门硬挡（4.10）。
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
  - `page` 夹具记下每个在途的 `/api/v0/` 请求（`request`、`requestfinished`、`requestfailed`），事件流除外；主框架提交它最后请求的那个文档时整个清掉，留下的都属于走了的文档（P1 设计 3.17，决定 356）。
  - 帮手在 `goto` 与 `reload` 之前等这些请求答完，其中一定等 `POST /api/v0/auth/refresh`。这两个换掉文档，打断在途的请求。帮手读完页面里的锁与令牌之后，隔一个轮询再看一次这些请求，同一轮都空才换（P1 设计 3.17）。
  - `goBack`、`goForward` 不一律算进来：故事里的 14 处多半是单页应用自己的历史条目，同一个文档里的 popstate，打断不了续期。14 处都在同一个文档里（P1 设计附录），帮手不另给跨文档的后退；以后要它的故事先在帮手里加，照 A15 加一格对照（P1 设计 2.1 第 1 条）。
  - **逃生口**：帮手带一个显式的选项 `{ inFlight: "leave" }`，不等就离开，只给有意在请求途中离开的故事用；P1 里只有 A15 的三格：第 1、3 格经它登出，帮手的旁支加的第 11 格经它留下一个读（P1 设计 3.6、3.17）。TR2 扣住导入的 POST 之后的后退（`e2e/stories/transfer/tr2-import.spec.ts:241-259`）在同一个文档里，帮手与门禁都不管，用不到逃生口。
  - `pageWatch`（`e2e/fixtures/browser.ts` 的 `watchPage`）认出没有预期的 `/sign-in` 落点，失败时写明"被跳回登录页"：它记下每一次落到 `/sign-in` 与之前那次续期是否答 401，失败时页面的状态第一行写出（P1 设计 3.3）。
  - **第一次续期**（审查 C 未核实的怀疑 U1）：`goto` 返回时，新文档的第一次续期常还没发出（探针：Chromium 15 次里 9 次、WebKit 15 次里 8 次；续期在 `navigator.locks` 授予之后才发，`session/token-manager.ts:289`）。只等在途的请求时没有复现失败（三种引擎各 20 次，0 次落到登录页）。帮手再等续期的锁空着、这个文档有了给它令牌的答复（第一次续期，或在这个文档里登录、注册：那样的文档不发第一次续期）；锁先于令牌看，登录、注册在锁里写记录（P1 设计 3.15 第 2、9 条）。A15 的第 4 格不经锁推迟第一次续期，只看在途请求与锁的帮手在 Chromium 与 Firefox 里落到 `/sign-in`；第 6 格在第一次续期之后持住锁，不看锁的帮手在放开之前就换页。WebKit 在请求下一个文档之后旧文档发不出请求，没有这个窗口（P1 设计 3.2、3.6；P1 设计审查 A-I1、C-I1）。
- **门禁**：`e2e/` 里（故事、夹具与以后的 `e2e/perf/`）不许直接调 `goto`、`reload`，只许经帮手。帮手所在的那一个模块是唯一直接调用它们的地方，门禁对它整个例外：不报它的直接调用，也不数它里面的逃生口（修订核对第 1 轮 C 的 N8；P1 设计审查 A-M1、C-M2）。逃生口要显式写，门禁按它的表核对每个文件的逃生口的个数，理由写在表里。门禁是 `tools/e2e-navigation/check.mjs`（`make lint-web`），先在样例上证明它看得见每一种违例，并核对读到了帮手的模块（P1 设计 3.15 第 7 条），带反向对照（13.4 第 1 条）。
- **对照测试**：把续期的答复延后（`route.fetch()` 之后、`fulfill` 之前扣住，同 `a5-refresh-reuse.spec.ts` 的第二个测试），再 `goto`。经逃生口不等时落到 `/sign-in`，经帮手时不落。对照测试是故事 A15（`e2e/stories/identity/`，三种浏览器）：续期的答复在途时、第一次续期还没发出时，经逃生口离开都落到 `/sign-in`（后一格只在 Chromium 与 Firefox：WebKit 在请求下一个文档之后旧文档发不出请求）；经帮手都不落；另几格各核对帮手等第一次续期、续期的锁（一格在第一次续期之后，一格在登录写记录之前）、没有 Web Locks 时的租约与在途的读，以及自己注册的文档不必等；帮手的旁支加四格，核对帮手读页面时发出的读也等、走了的文档留下的读不等、同文档的移动（204 之后移到别处、移到它提交的地址）不当作下一个文档（P1 设计 3.6、3.15、3.17）。
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
  - 33.1（Safari 窄屏的搜索页宽到 704）：在 P2/S3 里用 Playwright 的 webkit 只跑 SE1 的 375 一段排查。复现了就是 B（WCAG 1.4.10），在 S3 里修；没复现，留在发布之后的人工清单（I3 第 7 节第 15 条）。实际复现了（Playwright 1.63 的 WebKit 26.6，页宽 704），在 S3 修（`NativeSelect` 的 `contain-paint`，决定 589）；真的 Safari 上仍进发布之后的人工清单（决定 590）。
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
  - 后台撤销的 `revertSteps`/`needed`（`page/app/jobs.go:578-585`）随之改。它的注释写明的前提是"a restore under a member of another group is a nested trash item, which L refuses"，修法让 L 接受了嵌套的回收站项，前提不再成立。`needed` 也看段内后面的放回的父节点。审查 A 没有造出出错的交错，前提已不成立，照改（设计审查 A 的 M3）。P3 的设计造出了出错的交错，它的审查 A 在原型上复现（不改时 400 次里 17 次失败），并沿操作推出生产里也走得到（推出的，没跑）：这是修正，不是防备。批的核对 `CheckRevertSteps` 整组放回，挡不住；`RestoreNodes` 的 SQL 不查父节点是否活着（推断真库也不挡，没在 PG 上跑）；只有 `needed` 挡（P3 设计 3.10.6）。
  - 不改 M11 的"一个任务一个变更集"与任务答复的 `changeset_id`，不改契约。
  - 审查 A 的原型（`split-tops.diff`，约 10 行）在 nested 形状上跑通：PlanRevert 给出 `[restore Made, restore Inner, place Guide]`；整个程序上请求的撤销答 200，后台的撤销 succeeded，笔记本等于 agent 留下的样子；page 模块的测试全过。原型只作参照，P3/S2 的 spec 照它核对，不是定稿。
- **认出的形状**，照实写。初稿写"只认这一种"，列了三个条件；它们实际上就是"本身是回收站根、被这个变更集删的节点"（设计审查 A 的 M2）。认出的有三种：
  1. 回滚第一遍对计划那一遍：新文件夹收着基线页与新页；
  2. `PackFreeSubtrees` 的自下而上，可以多层；
  3. 不是回滚的：变更集 X 一次删了一个文件夹与它下面的页（一个回收站项）；之后有人恢复了这一项，先删那页，再删文件夹。撤销 X 今天答冲突，修了之后照常撤销。按状态比较的原则这是对的：今天"恢复之后整个再删"就照常撤销。spec 写明这一点，有测试钉住。
- **测试**：
  - 性质测试走现成的路（设计审查 A 的 M2、M3）：
    - `TestTheRollbacksStepsReplayToTheBaseline`（`page/app/rollback_test.go:560`）的 `whole` 本来就是 1–4，嵌套的根自然出现（前两种来源都算；P3 的设计审查 A 量过：每 400 次 14–30 次）。在回滚 `Done` 之后撤销回滚的变更集，请求与后台的撤销轮流，对照 agent 留下的状态。它只数嵌套的根：挂在中间节点下的与两个成员以上的嵌套组在这里每 400 次只有 0–3 次，交给下一个测试（P3 设计 3.10.7）。
    - `TestTheJobsStepsReplayToStar`（`page/app/revert_job_property_test.go:64`）的生成器加嵌套回收站根的形状（含挂在组的中间节点下的、两个成员以上的与 `needed` 截短的段），各给它出现的次数加一个下限：数形状真的出现的次数，不是规则触发的次数（05-lessons 第 1 节）。装载要照每一项的时刻：今天的 `load` 把回收站的节点都记在一个时刻，嵌套的根会被读进外层的组；domain 测试的 `world` 同样按时刻读组（P3 设计 3.10.5、3.10.7）。
  - 确定的测试：domain 的一张表（三种形状、反例与"太深"），后台撤销的 `needed` 形状一格（P3 设计 3.10.5、3.10.6）。
  - 变异：去掉新规则；`topOf` 走过回收站根；`needed` 不看段内后面的放回。初稿的"放宽到任何回收站根"与新规则等价，会活下来，删掉；"`RevertGroups` 不读里面的组"随修法不再适用，删掉。
  - `TestRevertingARollbackConflictsUnderANewFolderHoldingBaselineAndNewPages` 的 nested 一格翻过来。今天那一格在同一个变更集上先同步撤销、再后台撤销（`bootstrap/upgrade_rollback_revert_test.go:99-110`），只因两次都冲突才行得通；修好之后第二次撤的是已经撤过的变更集。改为两本笔记本，每条路各回滚一本（审查 A 的 `bootstrap_zz_m12review_test.go` 的做法）。测试照新的结论改名，`BOOTSTRAP_SEVENTH`（`Makefile:296`；`194ca350` 上是 `:321`）与 `BOOTSTRAP_EIGHTH`（`:328`；`194ca350` 上是 `:365`）的模式同时改；漏了的话新名字以 `TestR` 开头，落进补集的第四份（设计审查 A 的 N4）。
- **不去造的**：初稿要 P3/S2 的 spec 去造"后台恢复与后台撤销里的同一形状"。审查 A 已核对造不出，从 spec 里拿掉（设计审查 A 的过度设计 1）。理由：后台恢复的任务只放回、不删；后台撤销的一次删除是一个步、不跨批（`page/app/jobs.go:494-499`），`tops(s)` 不会先删"上级是本变更集新建且还活着"的节点（`page/domain/revert_schedule.go:370-384`）；MCP 的 `batch` 把"后删的上级盖住先删的子树"并成一次删除（`page/domain/batch_plan.go`）。只有回滚造得出，就是上面前两种形状。
- **放弃要不要输入名称**：第 302 条让放弃照旧不要求输入名称。修好之后理由更足，不改。
- **P3/S2 先写 spec**：计划是所有撤销都走的路，改动的边界要先定清。

#### 4.6.2 日志的读放进一个快照（H5）

- 日志的读不开事务、不加锁（`page/app/list_changesets.go:51` 的注释）。现在是五条读：`ListChangesets`（`:192`），加 `page/app/history.go:165-180` 的 `ChangesetFacts`、`FirstChanges`、`FirstIngestSources`、`DisplayNames`（I3 第 3 节 H5）。读跨过一次撤销的提交时，可能列出"已撤销"却没有那条撤销。
- 修法（I3 的推荐，P3 的设计改了位置）：快照（`WithinSnapshot`，`platform/postgres/tx.go:137-144`，搜索已在用，`page/app/search.go:222`）放在四个用例里，从各自第一条属于答复的读起，到 `alivePaths` 为止：`ListChangesets.Execute` 从 `ListChangesets`，`AtPath`（MCP 的 `log {path}`）从找路径的 `StateOf`，`GetChangeset.Execute` 与 `InNotebook`（MCP 的 `changeset`）从 `FindChangeset`。只包 `page` 不会嵌套，但会把 `FindChangeset`、找路径与 `alivePaths` 留在快照之外，它们的结果都在答复里（P3 设计 3.8；P3 设计审查 B 的 N4）。REPEATABLE READ 的快照冻结在第一条读开始时、早于它隐含的表锁（P3 的设计与审查 B 实测）。`HistoryReads` 加 `Snapshots`。
- 一个变更集的详情（`GetChangeset` 的 `detail`，`page/app/history.go:83-93`）同样调 `logged`，五条读不在快照里，会有同一种不一致。一并包上（设计审查 A 的 N2）。
- 审查 A 核对过：`WithinSnapshot` 不许嵌套；`page` 只由 `Execute` 与 `AtPath` 调，都不在事务里；LLM Wiki 的虚拟 `log.md` 直接调 store（`page/virtual_files.go:108`），不经 `page`。
- 代价：每次读多占一个只读快照的连接。P4/S3 的日志一项照新的读量。

#### 4.6.3 标题键与标题 id：赶在 v0.1.0 之前（25.1、10.2）

两处都改已有的键或 id。发布之前没有生产的库，改动只碰开发库；之后再改，就要每个部署都 reindex（I3 第 7 节第 4、5 条；主持者的决定 313）。

- **25.1**：`shared.TitleKey` 对切罗基文不幂等（`shared/title_key.go:18-28`）：`cases.Fold()` 把切罗基文的大写折成小写、小写折成大写，`TitleKey(TitleKey(x))` 不等于 `TitleKey(x)`。审查 A 复现了：`TitleKey("Ꭰ")` 得 `"ꭰ"`，`TitleKey("ꭰ")` 得 `"Ꭰ"`。
  - 三处同样的折叠一起改：`shared/title_key.go`、`shared/title_key_context.go:29`、`linking/domain/index.go:80,97`（I3 第 5 节第 6 条：移交只写了第一处；审查 A 核对过非测试代码里恰好这三处）。
  - 改为在折叠之后把切罗基文的小写换成大写，同 `searchFold` 的映射；然后删掉 `searchFold` 的映射（`page/domain/search.go:245-256`）。删掉前后同一组查询的搜索结果相同，由测试比对：测试里留一份改之前的 `searchFold`，在一组语料上与改之后的 `TitleKey` 逐个比（P3 设计 3.2）。
  - 不升分词的版本，也不升 `Extractor`：改了之后 `searchFold` 的映射不再起作用，分出的词不变；升版本只会让 reindex 白白重建全部搜索的行（I3 第 5 节第 7 条；README:534：30 万页约 26 分钟）。
  - 网页的 `lib/title-key.ts` 把两种大小写都键成小写，等价类与服务端一致，不必改（审查 A 核对）。
  - 13.1 第 28 条（标题键）随 P3/S1 改。发布说明写一句：v0.1.0 之前的试用库若有切罗基文的标题，跑一次 `nervewiki reindex`。`reindex` 重算标题键；同一父页下已有只差切罗基文大小写的兄弟页时它报撞键、这本整个不重建，要先改名其中一页（P3 设计第 9 节；P3 设计审查 B 的 M2）。
- **10.2**：标题的 id 丢了自动链接与原始 HTML 的文字（`platform/markdown/parse.go:83-103` 的 `PlainText` 跳过它们，`:174-177` 的 `AnchorID` 用全文）。改为带上它们的文字。
  - 不升提取规则的版本（13.3 第 6 条），不要 reindex。链接的锚点按文字存在 `Link.Anchor`（`linking/domain/facts.go`）；标题的 id 只在渲染时算，`PlainText`、`HeadingID` 在非测试代码里只有 `platform/markdown/parse.go:69` 一处调用；没有渲染的缓存（设计审查 A 的 N1）。
  - 发布说明写一句：这类标题的锚点 id 变了。

#### 4.6.4 小项

- **10.1**：标签 `/`（写作 `#//`）列得出、叫不到。最小的改法：`listTags` 不列 `/`（`linking/adapter/postgres/queries/reads.sql:87-94`，按 `tag_key` 分组，加一个 `<> '/'`），补全随之不列（I3 第 9 节 10.1）。`TagPages` 的前缀比较对 `/` 本来就走不到（审查 A 核对）。
- **30.2**：`shared/name.go:36-38` 的 `Unshowable` 只去掉 Cc、Zl、Zp 与 Bidi_Control。由看不见的字符组成的客户端名与显示名可以冒充空白（移交 M5-polish 第 30 条第 2 点）。改法（设计审查 A 改）：
  - `Unshowable` 不动。它不只管名字：标题（`shared/title.go:35`、`:71`、`:87`）、客户端名（`shared/client_name.go:30`）、库的 CHECK（`server/migrations/sql/00044_page_client_checks.sql:10-15`，由 `migrations/schema_test.go` 的 `TestChecksRejectCounterexamples` 逐码位核对）与网页的 `lib/upload-name.ts:17-19` 都是同一个集合。初稿要把它扩到默认可忽略的字符，会拒带 ❤️（U+FE0F）、ZWJ 的 emoji 与 ZWNJ 的波斯文的新标题与名字，与 `shared/name.go:29-35` 写明的取舍相反，还要迁移两张表的 CHECK。
  - 改为对整个名字的判断，同移交的原意（"没有图形字符的名称算空"）：在 `CheckName` 与 `CleanClientName` 之后，去掉默认可忽略的字符与空白之后什么也不剩，`CheckName` 答 `required`（它管显示名、工作区名与 PAT 的名字，三处一条规则），客户端名当作空的（`""`）。默认可忽略的字符照 Unicode 的推导精确地算（Other_Default_Ignorable_Code_Point ∪ Cf ∪ Variation_Selector，减去 White_Space、U+FFF9–FFFB、U+13430–13440 与 Prepended_Concatenation_Mark），`۝` 这类单独有字形的照旧通过；U+2800 是图形符号，不算空。只管新的输入（P3 设计 3.5、决定 367）。
  - 标题、库的 CHECK 与迁移都不变。
  - 标签块 U+E0000–U+E007F 不从客户端名里去掉：它们是苏格兰、威尔士这类旗帜的 emoji 序列的一部分；只由它们组成的名字算空（决定 367）。
- **30.4、30.5**：MCP 的 `log {path:""}` 与 `{path:"/"}` 答"path is required"（`page/app/list_changesets.go:132-134`），文字误导（30.4，B）。`index` 的筛选给 `""` 答 `too_short`（`linking/app/list_index.go:196-201`），工具说明写着 `"" is refused` 与 `"/"` 是根（`linking/adapter/mcp/tools.go:61-62`）（30.5，C）。约定（主持者的决定 314，设计审查 A 改）：
  - `index` 不动：它的 `parent:"/"` 是根（`list_index.go:205-211`），写在工具说明里；`list` 同样以 `""` 与 `"/"` 表示根（`page/adapter/mcp/tools.go:170`）。初稿让 `"/"` 一律答字段错误，会让 agent 列根下页的现有写法 `index {parent:"/"}` 改答 422，是公开的 MCP 行为的回退。
  - 只改 `log` 的 `path`：省略是整本；`""` 答字段错误，文字写"省略它就是整本"（同 `index` 的 `""`）；`"/"` 当作整本（同 `list` 与 `index.parent` 的根）。
  - 细节在 P3/S1 的设计里。
- **MCP 调用超时的文字**（移交 M4-performance 第 22 条之后的补记 `:490`；主持者的决定 335，设计审查 C 的 I2）：
  - 一次 MCP 调用超过 `server.request_timeout` 时答 `internal_error`，文字是通用的"the call ran out of time"（`platform/httpserver/mcpserver/errors.go:77`、`:101`）。最大合法的 `batch` 在慢的 4 核上就会这样：事务整个回滚，不留半截。只说内部错误，agent 不知道该拆小再试，属误导的文字（B）。
  - P3/S1 改文字：说明这次调用超过了请求的期限、什么也没有写，`batch` 拆成几次再试。码照旧是 `internal_error`，不改：改码会改 MCP 的码表与每个工具的码的列表，agent 已有的处理随之失效（决定 369）。一处例外不说"什么也没有写"：写的事务在提交时跨过期限，结局未知；`TxManager` 给读写事务的提交错误加一个标记，MCP 认出它时另用一句，请 agent 先读笔记本看写下了没有（P3 设计 3.7；P3 设计修订核对 B 的 N14）。P3/S1 另：提交在调用的期限之前失败的（默认 `commit_timeout` 2 秒短于请求的 15 秒，这一种更常见），`FaultUnknown` 一支也认标记，答"保存时失败、可能保存了、先读笔记本再重试"（决定 483）。
  - 不给 `batch` 单独的期限：负责人 2026-11-06 的话是不无限优化。写进已知限制，进 v0.2。
  - 硬件的下限写进部署文档（4.8）。

#### 4.6.5 每个凭证与每个账户的事件流数（移交 M5-performance 第 6 条）

- hub 不按账户设上限（`events/app/hub.go:26-31`；`open` 在 `:100-109`，不数）。每条流占一个连接、一个 goroutine 与 64 格缓冲。网页一个浏览器只用一条。
- 加上限（I2-D9；主持者的决定 315，设计审查 A 改）：
  - **按凭证计数**：`shared.Actor` 的 `SessionID` 或 `APITokenID`（`shared/actor.go:15-23`）。每个凭证至多 4 条；第 5 条打开时，重置**同一凭证**最老的那条。被重置的客户端照常重连。
  - **先后**（修订核对第 1 轮 A 的 N10）：先按凭证判。凭证已有 4 条时，新开的顶掉它自己最老的一条，账户的总数不变，不看账户的上限；只有凭证不到 4 条、要净增一条时，才看账户的 64 条。否则账户满了时，睡眠的笔记本电脑留下半开连接、新开的标签页会被拒，这正是选"重置"而不选 429 的原因。
  - **账户一级的硬上限** 64 条：超出时新开的答 429，带 `Retry-After`：一个凭证已被限在 4 条，这一层挡的是一个账户用很多会话或令牌开一批（移交的原意是挡一个账户开太多）。沿用顶层的 `rate_limited`，`Retry-After` 30 秒（网页最长的退避），文字是它自己的；网页对任何状态码都照 `Retry-After` 等（`services/api.ts:61`、`:70`），不改代码、不加测试（P3 设计 3.9）。
  - 两个上限都是**常量**，不做配置项（设计审查 A 的过度设计 2）。
  - 被重置的那一条记一行 `debug`（限流本身也只记 `debug`，决定 371），不做限频的 `info`：带账户 id、凭证的种类与条数，不带令牌（13.1 第 10 条）。
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

     一个包里只要有一个非 UTF-8 的名字这样解不开，整个包都不回退，照旧跳过。三条都满足的名字（候选）只用来判"这个包是不是 GBK 的"：是的话，没有标志、没有有效 0x7075、含非 ASCII 字节、按 GBK 解得开的名字都按 GBK 读，恰好也是合法 UTF-8 的一并读（GBK 的 `目录`、`图片` 就是合法的 UTF-8，只改不合法的会把同一个文件夹拆成两个；决定 376）。0x7075 对每个没有标志的名字都读。
  - 报告写明哪些名字是按 0x7075 或 GBK 解读的：两种新原因 `name_unicode_path`、`name_gbk`，在任务结束时加，排在这次导入的别的问题之后，不挤掉真正跳过的条目。解码在 archive 适配器里（纯的层不许导入 `x/text/encoding`），文件夹按读出的名字判（GBK 的尾字节可以是 0x5C）（P3 设计 3.12、决定 372）。
- **已知限制**：日文的 Shift-JIS 名字按 GBK 也完整解开，没有 U+FFFD（审查 A 实测："テスト資料.md"成了"僥僗僩帒椏.md"）；Big5、EUC-KR 与 GBK 的双字节区大面积重叠，推断也是这样。这些包今天被清楚地跳过，修了之后会被建成乱码名的页，报告里另列。另有两条（决定 376）：只有恰好合法 UTF-8 的 GBK 名字、没有一个候选的包认不出；一个包同时有 GBK 候选与没有标志的真 UTF-8 名字时，今天真 UTF-8 名字导入得对，之后会被读成 GBK，是相对今天的回归，报告里逐个列成 `name_gbk`。写进已知限制（4.13）。
- **不做**：导入时选编码的选项；CP437（任何字节串都是合法的 CP437，要另定判别），进 v0.2。
- 外来格式的读取照 13.4 第 11 条，有差分的测试：没有候选的包，除了有效的 0x7075 一个字节也不改；有标志的名字不改；GBK 的包里改了的名字等于解码器对原始字节的输出（决定 376 之前这里写"解读只改原本不是合法 UTF-8 的名字"）；预读与标准库的对照照旧。`golang.org/x/text` 已是依赖。
- 报告的新原因带两种语言的文字，与服务端一起加（13.2 第 4、11 条）。

**T2 按人的队列上限**（主持者的决定 336；负责人 2026-11-14 第 5 问）

- 现在：`room`（`transfer/app/start.go:212-227`）只比全局的 `CountActive` 加正在上传的导入，至多 `transfer.max_queued`（默认 20），超出答 503 `server_busy`（`ErrQueueFull`，`Retry-After` 5 分钟，`transfer/domain/errors.go:49-54`）。同一人同一本只有一个导出，一本笔记本只有一个写入的任务，跨笔记本不限：一个人可以占满全局的队列（I3 第 4 节 T2）。
- 改法：`room` 另数这个人开始的排队与运行中的任务（`created_by_id`）与他正在上传的导入，到一个常量（`MaxPersonJobs = 5`，P3 设计 3.13）就拒。常量，不做配置项。
- **拒绝用一个新的码**（修订核对第 1 轮 A 的 M9、C 的 M10）：例如 `transfer.too_many_jobs`，有自己的两种语言的文字，说的是"你自己已有几个任务在排队或运行，等一个做完，或取消一个"。
  - 第一次修订让它沿用 503 `server_busy`、契约不加码。可网页按码出文字：导入对话框把 `server_busy` 显示成"Too many jobs are waiting on the server. Try again in a few minutes."（`pages/notebook/import-dialog.tsx:220`、`en.ts:905`），别处是"The server is busy. Try again in a moment."（`en.ts:1202`）。到了自己上限的人读到的是"服务器忙"，别人的任务却照常能开始，他自己的任务可能还要跑几个小时，`Retry-After` 却是 5 分钟。这是负责人第 1 问要在 v0.1 修的误导的文字，而且是 M12 自己新造的。
  - 状态码是 409，不带 `Retry-After`（决定 373）：按人的上限不随时间解除，要等自己的任务做完或取消一个，同模块的 `transfer.busy` 也是 409；429 带 `Retry-After`，会引着客户端几分钟后白试一次。码的次序照今天的检查：导出是 `transfer.too_many_jobs` → 503 → `transfer.busy`；导入、后台的恢复与撤销是 `transfer.busy`（撤销另有冲突、`page.locked`）→ `transfer.too_many_jobs` → 503（13.1 第 4 条随 P3/S3 改）。
  - 开任务的四个操作（导出、导入、后台的恢复与撤销，都在 `api/modules/transfer.yaml`）的 `x-problem-codes` 与说明随之；码与原因由 `problem-messages.test.ts` 读契约核对（13.2 第 11 条）。
  - 网页在开任务的各处照新码显示。MCP 的工具不开任务（各模块的 `adapter/mcp` 不调 transfer），不涉及。
  - 全局的 503 `server_busy`（`ErrQueueFull`）照旧。
- **升级的回滚不受按人的上限拒绝**（修订核对第 2 轮 A 的 N14）：回滚的开始也走 `room`（`transfer/app/rollbacks.go:32`、`start_background.go:175-182`、`:200`），`abandonUpgrade` 与驳回因此今天就可能答 `server_busy`。按人的检查若不另作处理，一个人有 5 个任务时连放弃升级都被拒，笔记本停在冻结里，与负责人"任何管理员都能随时放弃、整体回滚"相悖。所以 `room` 对回滚的任务不做按人的检查。回滚算进开始它的人的计数，但从不被拒；计数与全局的计数是同一条语句、同一把队列的锁（P3 设计 3.13）。
- 测试：一个人到上限时他的下一个答新码，别人的照常开始；全局满了照旧答 `server_busy`；网页的文字由 vitest 整句比对。去掉按人计数、按人的拒绝答成通用的 `server_busy`，两个变异都让它们失败。

**T4 启动时收拾上一个进程取到而没开始的任务**（主持者的决定 336；负责人 2026-11-14 第 5 问）

- 现在：服务启动时 `AtStart`（`transfer/app/maintain.go:84-95`）只把 `running` 的行记成失败。进程恰在 River 取到任务、任务开始之前停掉时，行仍是排队，River 把它的任务记成 running，直到救援（`transfer.job_timeout` 加 1 小时，默认 7 小时）。其间它占着队列的一个名额，这本笔记本开始不了新的导入、恢复与撤销（`README.md:193`）。
- 改法：v0.1 只有一个实例（4.8）；`AtStart` 在 River 开始取任务之前跑，这时 River 记为 running 的导入导出任务都是上一个进程留下的。`AtStart` 把这些任务对应的、仍在排队的行也记成失败 `interrupted`，名额立刻空出。
- River 那一边（修订核对第 1 轮 A 的 N12）：平台已有 `Job.DiscardRunning`（`platform/jobs/jobs.go:33-39`、`:134-148`），在 River 启动之前把某一种上一个进程留下的 `running` 任务记成 discarded，搜索的任务在用（`page/adapter/river/search.go:88`）。这是 River 那一边最自然的做法。
- 要注意次序：discard 排在同一个任务自己的 `Start` 之前（`jobs.go:101-106`）。transfer 若照抄，`AtStart` 就看不到"River 记为 running"的那些了。所以要么 `AtStart` 自己先读、再 discard，要么按"discarded 且错误是 left running by a stopped server"来认。P3/S3 取第三种（决定 374）：五种任务各设 `DiscardRunning`，它们在模块的任务表里排在收拾之前，收拾的 `AtStart` 运行时上一个进程留下的都已丢掉；`AtStart` 之后对每一种跑一次 `lost`（同 `Run` 每 5 分钟做的），River 不再持有的排队行记成失败 `interrupted`。它也碰到以 SQL 只种排队行、不种 River 行的已有测试，这些种子改为带一行 River 持有的任务（P3 设计 3.14.3；P3 设计审查 C 的 I3，主持者定）。
- 测试：已有的 `TestServeFailsTheJobsLeftRunning` 加两种排队的行（River 的行在运行中的、River 不持有的），启动之后都是失败，River 持有的照旧排队；不另起一个应用（P3 设计 3.14.2；P3 设计审查 C 的过度设计）。
- `README.md:193` 的那一句随之改。

### 4.7 性能与内存实测

照负责人第 2 问（主持者的决定 317，设计审查 A 改）："量到总体设计写的规模；只修超出设计已写预算的（例如页面树超标就改分层加载），其余记成已知限制进 v0.2"。负责人 2026-11-06 另说过："不要把这次当做性能问题继续无限优化了，调整测试用例量级或者时限即可"。

#### 4.7.1 预算与超出的判据

写下了预算的只有下表几行（I2 第 3 节）。没有预算的只量、记下，能推算的推算。

| 编号 | 预算 | 出处 | 量的规模 | 超出的判据 | 判在 | 超出时 |
|---|---|---|---|---|---|---|
| B1 | 搜索：每类查询第一页与"更多"的 p99 ≤ 300 ms；体积 ≤ 正文 3 倍；任务 ≥ 100 页/秒；30 万页从空建 ≤ 1 小时；一页的保存 p99 多 ≤ 2 ms | `M8-history-search/05-P5-search.md:890-902` | 30 万页，两种分布（200 本、5,000 本） | 300 ms 的尾巴与约 5 倍的体积是已接受的超出（`05-P5-search.md:894`、`:899`、`:187`）。只有超出 M8 已记的最大值才算回归：200 本 719 ms（P5A 真语料"为任意的参数规划"那一阶段；无统计是 348 ms，R1 之后的合成语料上 474.6 ms），5,000 本 827.2 ms（R1 之后，无统计），体积 5.25 倍（R1 之后）；超了先在同机同语料上对照 `308eafc6` 两轮，也慢 20% 以上才算回归；建索引时与导入的清空期间沿用 M8 的数（908、1,149 ms）、不重量，积压只记（I2-D3；P4 设计 2.1 第 2 条、3.6 的 B1）；其余几项照原预算直接判（任务 ≥ 100 页/秒、30 万页从空建 ≤ 1 小时、保存 p99 多 ≤ 2 ms，保存可以推算），不经对照 | 本机的数；尾巴与体积对照 M8 记的数看回归，其余对原预算 | 回归与超了原预算的要修；尾巴与体积没回归，维持已知限制 |
| B2 | 解析预算 8 MiB，最坏约 2.4 GB 堆；取不到等 `parse_max_wait` 2 秒后 503 | `server/configs/config.yaml:95-98`，`v0.1-design.md:1300` | k = 2、4、8、16 个并发的 5 MiB 阅读与保存，普通与病态两种正文 | Go 的堆峰值超过 2.4 GB 加空闲时的基线；或没等满 `parse_max_wait` 就 503（堆峰值是 gctrace 每行 GC 开始与结束时的堆的最大，减每轮空闲时强制 GC 那一行的存活堆；超 2,400 MiB，即 8 MiB × 300，为超出，增量落在 2,289–2,400 MiB 之间时（README 的"约 2.4 GB"是同一个乘积的约数）两样都写、列给主持者；早答是客户端不到 2.0 秒的 503；P4 设计 2.1 第 9 条、3.6 的 B2） | 本机的数（内存不随 CPU 变，不乘） | 修。P4/S3 量得病态正文 k = 16 超约 1–8%（复核也超），负责人 2026-11-21 第 13 问第 4 条改判：文档写最坏约 2.6 GiB 堆（2,662 MiB；最坏实测 2,592 MiB），进程的 `VmHWM` 至多 2,718 MiB（约 2.65 GiB），部署按约 3 GiB 留内存（P5/S3；P4 设计 10.3） |
| B3 | 大请求体整个服务两个名额、每凭证一个、等 2 秒；两个合起来约 1 GB | `bootstrap/mcp.go:40-41`，`README.md:627` | 两个名额加排队的，几种形状 | 内存超过约 1 GB 加基线（同 B2 的读法，两轮六次里的最大超 1,024 MiB；P4 设计 3.6 的 B3） | 本机的数（不乘） | 修 |
| B4 | 每个请求 15 秒（`server.request_timeout`） | `server/configs/config.yaml:17` | 正常形状：导入上限的 6 万个节点（`config.yaml:153`；P4/S6 起导入至多 4.5 万，6 万个节点的最上层由两次导入搭成，P4/S7 决定 658）、每页几十条链接 | 正常形状下删除、移动、改名到了期限（I2-D6） | 有 CI 上的实数时由它定；没有时本机 × 回收站与撤销一族的实测之比（推断；另写 × 4.6 的结论，P4 设计 3.5.4） | 修（照用户碰到的错误）；病态形状（20 万个节点的子树、400 页 × 2,000 条链接）记成已知限制 |
| B5 | 满负载下一个单元 ≤ 5 秒（`MaxUnitNodes` 1 万、`MaxUnitHistory` 40 万、`MaxRevertSteps` 1,500、`MaxJobItems` 6 万，P4/S6 起 4.5 万） | `v0.1-design.md:1171`；`page/domain/trash.go:57`、`:66`，`revert.go:58`，`jobs.go:29` | 各界线处 | 只用 M8 写下的规则：参照机（本机）上、满负载、两轮的较大值超过 5 秒（每轮的值是 M8 的量法：回收站与撤销的基准三次的均值，后台的一个样本；P4 设计 2.1 第 13 条）。5 秒是 15 秒期限的三分之一，余量已含慢的主机与解析（`M8-history-search/02-P2-trash.md:470`），不再乘。最低配置上的硬期限另核：CI 守卫在 EPYC 7763 上的实数加 `parse_max_wait`（2 秒）超过 15 秒；硬期限只对请求一路的守卫核（界线处的恢复、撤销一次删除、撤销很多最上层），`MaxJobItems` 是后台单元、没有请求期限，只用规则一层（修订核对第 2 轮 A 的 N15）。实数只取这几条守卫与它们量的文件没再改过的那一段里的 7763 各次：不到三次就用已有的、写明次数；一次也没有时，取改动之前最近的几次，乘本机在两个提交上之比，写"推断"。实数之外并列两行推断：7763 各次的中位 × 1.25 加 2 秒；本机基准里最慢的一阶段（多半是 ANALYZE）对本机守卫之比 × 7763 的守卫加 2 秒；任一行过 15 秒，就同规则一层列给主持者与负责人，部署文档的硬件下限照他们定的写（P4 设计 3.6 的 B5） | 规则在本机；硬期限用 7763 上的实数（`test-go-guards` 带 `-v` 的那几行），不乘 | 不自动降常量：照移交 M4-performance 第 14 条的两种做法（降常量，或写进部署要求）列给主持者与负责人定（4.7.5）。P4/S3：`MaxJobItems` 没过，负责人 2026-11-21 第 13 问第 2 条定降到 45,000（P4/S6；已合并，在原规模上两轮至多 3,683 ms，过，P4 设计 10.4）；硬期限的第一行推断过 15 秒，第 3 条定最低配置仍是 7763、部署文档写明余量（P5/S3；P4 设计 10.3） |
| B6 | lint 名额与期限 `min(10 秒, request_timeout/2)`，生产 7.5 秒 | `bootstrap/wire.go:263-264`，`M10-llm-wiki/00-M10-design.md:295` | 10 万页（设计规模）；30 万页只量 | 10 万页上每条规则与摘要过 7.5 秒 | 有 CI 上的实数时由它定；没有时本机 × 升级一族的实测之比（推断；另写 × 2.33 的结论，P4 设计 3.5.4） | 修 |
| B7 | 升级的发起与批准在请求期限的 3/4 之内，生产 11.25 秒 | `bootstrap/upgrade_start_cost_test.go:31-40`（M11 第 232 条） | 10 万页；30 万页只量 | 10 万页上过 11.25 秒 | 有 CI 上的实数时由它定（守卫在 7763 上记下的）；没有时本机 × 升级一族的实测之比（推断；另写 × 2.33 的结论，P4 设计 3.5.4） | 修 |
| B8 | 导入导出一个任务 ≤ `transfer.job_timeout` 6 小时 | `config.yaml:144` | 导入的上限：5 万个条目、512 MiB 的 zip、4 GiB 解压（`config.yaml:151-157`）；几 GB 的导出 | 过 6 小时 | 本机 × 两族的实测之比的较大（推断；另写 × 4.6 的结论；6 小时之下乘多少都不改结论，P4 设计 3.5.4） | 修 |
| B9 | 编辑器的段落、标题每次解析至多约 80 ms | `v0.1-design.md:1227` | Chromium 里的最坏形状 | 过 80 ms | 本机的 Chromium（在用户的机器上跑，不乘） | 修 |
| B10 | 日志读的守卫 200 ms；`index` 的守卫 3.5 秒 | `v0.1-design.md:1275`；移交 M4-performance 第 18、20 条 | `index` 在 10 万页；按页的日志 10⁵、10⁶ 个变更集只量 | 两个都不是产品的预算，是 CI 守卫的界（`index` 的是 `linking/adapter/postgres/index_cost_test.go:37` 的 `indexCallBound`，13.4 第 8 条的口径）。只量；`index` 乘了之后过 15 秒的请求期限才算超出 | `index`：有 CI 上的实数时由它定，没有时本机 × 升级一族的实测之比（推断；另写 × 2.33 的结论，P4 设计 3.5.4），对 15 秒 | `index` 过 3.5 秒照 13.4 第 8 条调守卫；过期限照 B4 修。按页的日志约 13–25 万个变更集时过 200 ms，记成已知限制，迁移进 v0.2（I2-D7） |
| B11 | lint 十万页摘要 TotalAlloc 的"初始 250 MiB" | `v0.1-design.md:1275` | — | — | — | 不是产品的预算：它被守卫的 1,571 MiB 取代，358 MiB 记下，不修（I2-D8） |
| — | 页面树 | 第 14 节"上万页"（`v0.1-design.md:1316`），没有数 | 见 4.7.2 | 见 4.7.2 | 本机的 Chromium | 见 4.7.2 |

- **判在哪台机器上**（设计审查 A 的 M7，修订核对第 1 轮 A 的 I4）：同一个数，换个判法结论相反：本机 7 秒的改名，在慢 4.6 倍的主机上是 32 秒，已经过了 15 秒，是用户碰到的 500。所以逐行写明：
  - "本机的数"：内存、浏览器、B1（尾巴与体积只看回归，其余照原预算），与 B5 的规则。
  - 有硬期限的 B4、B6、B7、B8、B10 的 `index`，与 B5 的硬期限：**有 CI（EPYC 7763）上守卫记下的实数时，由实数定结论；没有时才乘本机对 7763 的实测之比**，写明"推断"。实数要与预算同一规模：规模不同时仍是推断。B7 的守卫量的是 5 万页（`bootstrap/upgrade_start_cost_test.go:41-44`），注释里 CI 上 10 万页的数是乘出来的，所以 B7 在 10 万页上只能推断（本机 × 升级一族的实测之比）；B6 有 10 万页 lint 摘要的守卫，同规模（修订核对第 2 轮 A 的 N13）。
  - 推断用本机对 7763 的实测之比（P4 的设计审查之后的决定 454 改了这一条与决定 318 的倍数一半；P4 设计 2.1 第 3 条、3.5.4）：同一组守卫在 7763 上各次的中位（每个作业取最近的至多三次），除以本机在同一批提交上的中位，按族取最大；回收站与撤销一族给 B4，升级一族给 B7（B6、B10 的 `index` 没有实数时也用它），B8 取两族的较大；对硬期限的推断另并列 × 1.25 的一行（决定 359 记的 7763 的尾巴）。移交里记下的倍数（lint、`index` 与升级 1.16–2.33 倍，取 2.33；回收站、撤销与后台 2.1–4.6 倍，取 4.6，I2 第 1.1 节）的分母是当时更快的开发机（M5 Max），其中还有 CI 的摆动，拿它乘现在更慢的本机会多算一层；照它的结论并列写出，与实测之比的结论不同时列给主持者。
  - 部署文档的最低配置是 EPYC 7763 的 4 核一类，那一档的数用它自己的实测（4.8）。
- **B5 只用 M8 写下的规则判**（修订核对第 1 轮 A 的 I4）：第一次修订加了"本机 × 4.6 ≤ 15 秒"一层，那是在设计已写的预算之上另立一个更严的，余量算了两遍，与负责人第 2 问"只修超出设计已写预算的"不合；它又比 5 秒严，规则一层成了摆设，"超出时"的做法也对不上。去掉。界线处的单元在 7763 上有守卫的实数（例如 `TestARevertAtTheUnitsBoundTakesItsTime` 记过 12.3 秒，移交 M4-performance 第 15 条），最低配置上的硬期限用它判。
- B4、B5 是硬界与定界线的规则，不是体验的目标：过了 15 秒是 500 或 503，什么也不写（I2 第 3 节）。

**只量、不判的**（移交的条目，归类照 I2 第 5 节）：M4-performance 第 1、5–8、10–13、15 的日志、16 的整个任务、18 的多数分项、19 的 30 万页、20、21 的 30 万页，与第 22 条之后的补记（第 7 节"移交的落实"）；M5-performance 第 3 条；M7-transfer 第 4、6 条；M8-history 第 2 条（版本的增长）；13.4 第 9、12 条的旧版本下的通用计划。它们进已知限制、部署文档的数与 v0.2 的移交。

**量的范围缩小**（设计审查 A 的过度设计 3）：先做有预算的 B1–B10 与部署文档要的数（内存、磁盘、WAL、硬件下限）。只量的各项，能用 I2 第 7 节已有的数推的就推，记录写明"推算"与依据的数。推不出、又不判任何预算的（事件流同时重连、按凭证的并发上传、版本的增长），不写专门的驱动或生成器：版本的增长照每次保存的版本行的大小推算；另两项写明不量与理由。

#### 4.7.2 页面树的界线（量之前写死）

主持者的决定 305：第 14 节的"上万页"定为 1 万页。下面的界线是设计者提的（主持者的决定 329，设计审查 A 改）；P4/S2 的设计照抄，量之前不再改。

- **规模与形状**：1 万页，三种形状：
  - 平：1 万页都在根下；
  - 深：10 个根、各 10 个文件夹、各 100 页，另有一个 1,000 个孩子的文件夹（共 11,111 页；那个文件夹在根下排在最前；T3 只在深的两种形状上判，P4 设计 4.3）；
  - 带附件：深的形状另加一成附件（1,000 个）。
- **环境**：本机，Playwright 自带的 Chromium，生产构建（`make build`），默认视口。不降 CPU，不加 CDP 的网络降速：走回环。
- **三条界线**：
  - **T1 冷打开**：从 `goto` 笔记本首页（已登录、HTTP 缓存清空）到树的答复已到、并且左栏视口里的行已画出，≤ 1 秒。终点用 `PerformanceObserver` 的 element timing，或视口内行数的 `waitForFunction`。
  - **T2 别人改一次树**：另一个账户经 API 新建一页，以 `after_id: null` 新建、排在同级的最前（树按 `sort_order` 排，不按名字），落在视口里。从答复到这个标签页画出新的一行 ≤ 1.5 秒（含 refresher 的 500 ms，`web/…/events/handlers.ts:36`）；其间主线程最长的一个长任务 ≤ 200 ms。新页放不进视口时，量"store 已更新加视口重画"。
  - **T3 展开**：展开那个 1,000 个孩子的文件夹，到视口里的孩子都画出 ≤ 500 ms。
- **为什么是这些终点**（设计审查 A 的 M5）：初稿的终点是"根下的行全部进入 DOM""新的一行画出"。"平"的形状没过时，候选的改法是只渲染看得见的行；改了之后，那两个终点永远到不了（新页按次序在一万行的末尾），同一条界线就证明不了改好了。视口里的终点在改法前后都量得了。T3 照同样的道理改为视口里的孩子。
- **为什么 T1 是 1 秒**：照 I2-D1 的推荐。量在回环上，网络不在被量的东西里；初稿放宽到 1.5 秒，理由是"给网络留余地"，与量法不合，而且盖不住真实网络上不压缩的 2.7 MiB（20 Mbps 上单传输约 1.1 秒）。主持者取不加网络降速、用 1 秒在回环上判这一种（设计审查 A 改）。
- **只记的**：树的答复的 JSON 字节与它在 gzip 1 级、6 级与 zstd 3 级下的字节（离线算，P4 设计 2.1 第 6 条）与 JS 堆（CDP 的 `JSHeapUsedSize`）两样；5 万、10 万页同样只记这两样。CPU 降速与持续改树不改任何结论，不量（设计审查 A 的过度设计 4）。
- **怎样判**：每条界线在每种形状上判。两轮，每轮 5 次取中位数，两轮的较大中位数对照界线。
- **结论**：
  - 1 万页三种形状都过：保留整棵树。5 万、10 万页的数记成已知限制，分层加载进 v0.2。
  - 有一条没过：在 v0.1 改，先写设计。改法看没过的是什么：数据一侧（读、比较、索引）多半是分层加载；只有"平"的形状在渲染上没过，多半是只渲染看得见的行（虚拟列表或分批）。改法定了之后，P2/S6 照它排；改完照同样的终点再量。
- **代价最低的缓解**：部署文档的两份代理样例给 JSON 开压缩（I2 第 2.5 节）。它不改结论，只写进文档。
- 依据：I2 第 1.2 节的小样里，数据一侧在 1 万页约 20 ms，渲染没有量（I2 第 2.3 节）；I2-D1 的推荐。

#### 4.7.3 方法：机器、规模、种子、两轮

- **机器**（I2-D2；主持者的决定 318，设计审查 A 改）：以本机为参照（aarch64，8 vCPU，30 GiB，Parallels 虚拟机；I2 第 1.1 节）。CI 的 EPYC 7763 作"慢的主机"的代表：它上面有守卫的实数时用实数，没有时按本机对 7763 的实测之比推算（4.7.1 的"判在"一列；P4 设计 3.5.4）。部署文档写"以下数字在 X 上量得"，最低配置是 7763 的那一档。没有真的生产机，不另租。
- **之前**：`free -h`（交换已满，噪声会大）、`df -h /`（搜索一批约 20 GB，先修剪 Go 的构建缓存，06-environment 第 3 节）；其间不跑别的构建与测试；全部经 `heavy.sh`。
- **被量的**：生产构建的 `nervewiki serve`（`NWIKI_ENV=prod`，默认配置，另写的除外），PostgreSQL 18.7 的容器，建库参数同测试（`--locale-provider=builtin --locale=C.UTF-8`）；进程内的基准经 `pgtest`（testcontainers 加了 `fsync=off`，同 M8），对外的容器不加（P4 设计 2.1 第 10 条）。
- **种子**：每批一个固定的种子，写进记录；生成器同一个种子出同样的数据。
- **两轮**：每批两轮，各自重启服务进程与 PostgreSQL 的容器。时延报两轮的 p50、p99，对照预算取较差的一轮；时长取两轮的较大中位数；内存取两轮的较大峰值。
- **内存**（I2-D14）：外部每 100 ms 读进程的 `VmRSS`、`VmHWM`，与容器的 cgroup `memory.current`（取最大）；容器每轮重启时也读 `memory.peak`。`memory.peak` 从容器起来就一直累积，清零要往里写、要 root（设计审查 A 的 N6）。Go 的堆用 `GODEBUG=gctrace=1`。产品里不加 pprof 或 metrics 端点。
- **浏览器**：Playwright 与 CDP（`Performance.getMetrics`、`PerformanceObserver` 的 `longtask` 与 element timing）。
- **批次**（I2 第 6 节）：A 页面树与浏览器各项、B 搜索、C lint 与 `index` 与升级、D 回收站与撤销与后台、E 并发与内存、F GB 级的导入导出、G 版本的增长（推算）、H 迁移在大库上的停机。P4 的设计重估：S3 约 16–20 小时、有条件的另约 4.5–6 小时，S2 约 3–3.5 小时，比初稿估的 10–16 小时多（搜索两种分布各两轮、对照照整轮算，导入之后每次删除都等搜索的队列，本机对 7763 之比要在每个作业最近的至多三次 7763 的提交上量；P4 设计 3.8）。H 只在负责人说有 v0.1.0 之前的实例要升级时才量（4.8）。
- **10 万页的树的重读**（设计审查 A 的 Q1）：批次 E 量一项，只记。10 万页的 LLM Wiki，agent 每秒建一页、60 秒，k = 1、4、8 个开着这本笔记本的标签页；量服务进程的 CPU 与出口字节，写进部署文档。每个标签页每 0.5 秒整棵重读一次（I2 第 2.3 节），碍着的是同一实例上的别人，不只是那个标签页慢。
- **CI 不跑**（I2-D5）：CI 不加 10 万、30 万页的守卫。只有修了东西的 Step 照负责人 2026-11-06 的话加缩小规模的守卫。

#### 4.7.4 设施进仓库

照 I2-D4（主持者的决定 319，设计审查 A 改）。可复现、可审查；"Codex 的证据不在 git 里"的教训。只写有预算的几项与部署文档要的设施。

- **Go 的生成器与基准**：在组合根写 `_test.go`，没有环境变量就跳过（语料的两个不连库的测试与 1,000 页的载入测试除外，P4/S1，决定 577），同 `BenchmarkSearch`（`bootstrap/search_bench_test.go:66-69`）：
  - 对外量的大笔记本经导入生成的 vault 建起（`server/tools/perf/vault`，10 万页导两次）；30 万页只经现有守卫的规模开关在进程内量（P4 设计 2.1 第 5 条）；
  - 合成的搜索语料：写成 `BenchmarkSearch` 读的三个 TSV，200 本与 5,000 本两种分布；词表取仓库里的 Markdown（中文的设计文档与英文的代码注释），按 Zipf 抽样，不下载语料（`04-owner-decisions-and-preferences.md` 第 4 节）；
  - GB 级的 vault 也是这一个生成器（批次 F）。版本的增长不写生成器（4.7.1）。
- **规模开关**：现有的守卫（升级、lint 等）加一个只在手跑时用的环境变量（例如 `NW_SCALE_PAGES`），默认仍是今天的常量（`upgrade_start_cost_test.go:41-44` 等）。
- **外部的驱动与采样**：放在 `server/tools/perf/`，只经 HTTP 打一个在跑的 `serve`。只做有预算的几项要的：k 个 5 MiB 的阅读与保存（B2）、MCP 的大请求体（B3）、N 个编辑者自动保存加树写（M4-performance 第 1 条与部署文档的内存；B5 的满负载是 M8 的口径，即数据的形状，用现成的基准量，P4 设计 2.1 第 1 条）、k 个标签页的树重读（4.7.3）。事件流同时重连与并发上传不写驱动（4.7.1）。采样的脚本同目录。
- **浏览器的计时脚本**：`e2e/perf/` 与它自己的 Playwright 配置，不进 `make e2e`；tsc、knip、lint 照常看它们。knip 要读到 `e2e/perf/` 的配置：P4/S2 经 `e2e/package.json` 的 `perf` 脚本（knip 从脚本解析 `-c`；配置写字符串的 `testMatch` 与绝对路径的 `globalSetup`）；`knip.jsonc` 只给 e2e 加一节，忽略 B9 的解析页按路径载入的 web 的 `vite`（P4 设计 2.1 第 7 条、4.4）（审查 C 未核实的怀疑 U4）。
- 全部合并之后，CI 多跑语料的测试（三个在 `test-go-costs-3` 一行，其中两个不连库的也在 `short`，几秒）、`server/tools/perf` 的单元测试与三个毫秒级的短测试（规模开关与档位，前两个也在竞态的第四份），驱动不进 CI；别的只多编译（P4/S1 的审查 C 的 M3，决定 577）。

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
     - 硬件分两档，默认配置与小机器（`page.parse_budget_bytes` 5 MiB 加 `GOMEMLIMIT`），数取自 P4；小机器一档是 P4/S3 批次 E 另量的一组（P4 设计的 P4-27：`page.parse_budget_bytes` 5 MiB 加 `GOMEMLIMIT=1536MiB`），不是推算（P5 设计 2.1 第 21 条）。最低配置是 EPYC 7763 的 4 核一类，那一档的数用 CI 守卫在那里记下的实数（4.7.1）；写明最大合法的 `batch`、回收站与撤销在慢的 4 核上离 15 秒的期限不远（移交 M4-performance 第 22 条之后的补记 `:490`、`:492`）。
     - PostgreSQL 18.7 或更新：18.7 的 GIN 修复正对着 `page_search`，还有一批 CVE（设计审查 B 的 N7）。以 builtin provider 的 `C.UTF-8` 建库、带 LZ4（`platform/postgres/check.go:53-66` 不满足就拒绝启动），托管的 PostgreSQL 要能这样建。
     - 磁盘：正文约 5 倍给搜索、版本的增长、导出与导入的临时文件、WAL。
     - v0.1 只支持单实例。
  2. **安装**：拉镜像并校验签名；建库；两个角色与 `deploy/runtime-grants.sql`；签名私钥；配置（`NWIKI_ENV=prod`、`server.public_url`、`server.trusted_proxies`）；`migrate up`；第一个管理员（`nervewiki users`，链到 README 的命令）；`serve`；`deploy/compose.yaml` 样例。
  3. **反向代理与 HTTPS**：nginx 与 Caddy 的整站样例，从根 README 移过来（`README.md:554-616`）；JSON 开压缩；几个 GB 的导出下载的超时与缓冲（移交 M7-transfer 第 3 条）；大的 MCP 请求体与正文写较多时推荐 nginx，经 Caddy 时早答的拒绝成 502 的代价写明（I1-D15、I2-D13；服务端的改法进 v0.2）。
  4. **备份与恢复**：命令（`pg_dump -Fc`、`pg_dumpall --roles-only`、先库后 `blobs/`）；恢复到 builtin 的库；恢复之后先 `ANALYZE` 再开放服务；私钥与配置也要备份；一次演练的步骤。
  5. **升级与回退**：通用的升级（拉新镜像并校验、`docker stop -t 40`、备份、`migrate up`、补授权、有说明时 `reindex`、启动、看 `/readyz` 与版本号）；`nervewiki reindex` 在 30 万页约 26 分钟、约 290 GB 的 WAL，之后表约是原来的 1.7 倍（M8 的数，`README.md:534`，`86565dd6` 上是 `:535`，P4 不重量；P5 设计 2.1 第 21 条）；回退靠恢复备份，`migrate down` 不保证数据；PostgreSQL 的小版本（18.7 的 GIN 修复：怀疑损坏时 REINDEX；18.7 说明里 TOAST 索引的 REINDEX 提示）与大版本（`pg_upgrade` 或 dump/restore、builtin locale、复核"一个事务里几条 `NOTIFY` 只取一次全局锁"，移交 M5-performance 第 5 条）。
  6. **运行与监控**：探针；日志；磁盘与 507；停机的次序与 `docker stop -t 40`；`server.shutdown_timeout` 20 秒短于一次 `/mcp` 调用（至多约 45 秒），停机时还在传大请求体的调用被切断、整个回滚，要它们做完就把两者一起调长（移交 M7-transfer 第 6 条）；连接数；`GOMEMLIMIT`；连接池代理（PgBouncer）只能用会话模式（移交 M4-performance 第 10 条）；校时：下载地址的到期按客户端的时钟比较（移交 M7-transfer 第 4 条）。
  7. **容量**：P4 的数（内存、磁盘、WAL、版本的增长的推算、10 万页的树被几个标签页重读时的 CPU 与出口、每 1,000 页的库的大小、树的 JSON 的压缩比、`GOMEMLIMIT` 的建议值；对照 P4 记录的"给部署文档"一节，P5 设计 3.18）；搜索的索引约是正文的 5 倍。
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
  - 样例用一个不是超级用户的库角色：官方镜像的 `POSTGRES_USER` 总是超级用户，样例的初始化脚本另建 `NOSUPERUSER` 的 `nervewiki` 作库与表的所有者；两个角色的写法与 `deploy/runtime-grants.sql` 在哪一步执行写在 `deploy/README.md`。附件目录绑定到 compose 文件旁的 `./data`（属主 65532），`blobs/` 在宿主机上打包备份、放回；默认网络钉一个子网，`server.trusted_proxies` 信任它：宿主机上的代理经端口转发进来，服务看到的对端是这个网络的网关，不设时所有客户端共用按 IP 的限流（P5 设计 2.1 第 22 条，P5 设计审查 C 的 I1、M3、M4）。
  - `make deploy-check` 跑 `docker compose -f deploy/compose.yaml config -q`，CI 的 `image` 一份跑它（13.5 第 1 条：新门禁同时进 Makefile 与 CI）。它只查语法与变量（插值与 schema 本来就覆盖整份文件，`--profile migrate` 另让 `migrate` 里的引用也核对，P5 设计 3.17），上面那一条由演练核对。
  - 它的 PostgreSQL 镜像与 initdb 参数进 `pgtest/image_test.go` 的守卫（4.3）。
- **演练**：P5/S3 照 `deploy/README.md`，用 `deploy/compose.yaml` 本身在本机做三遍：从零装一遍；改镜像的 tag 升一次级（备份、`run --rm migrate`、启动）；备份再恢复一遍。用本地构建的镜像（`make image` 要在普通的克隆里跑，`Makefile:89`）。包设为公开之后，用 GHCR 的镜像再装一遍（4.11 第 5 步）。

### 4.9 版本号

照 I1-D8（主持者的决定 321，设计审查 B 改）。

- **唯一的注入点**是 ldflags 写 `buildinfo.version`。默认值 `0.1.0-dev` 有四份：`Makefile:13`、`platform/buildinfo/buildinfo.go:9`、`deploy/Dockerfile:29`、`:42`（`86565dd6` 上是 `:32`、`:45`），没有守卫（I1 第 1 节）；`README.md:43` 写着这个默认值。
- **守卫**：一个短测试核对四份与 README 写的一致，带反向对照（P5/S1；README 一项是 决定 432）。不去掉 Dockerfile 的 `ARG` 默认值：直接 `docker build` 的人仍得到一个能用的镜像。
- **MCP 的版本**：`serverInfo.version` 取 `buildinfo.Get().Version`（`bootstrap/mcp.go:90`）。image-smoke 只核对 `nervewiki version`（`deploy/image-smoke.sh:76-78`）与 `/api/v0/instance`（`:131-136`），不碰 `/mcp`；加一个短测试断言 MCP 的 `serverInfo.Version` 等于 `buildinfo.Get().Version`，来源相同，image-smoke 照旧（P5/S1；设计审查 B 的 M9、C 的 M2）。测试的二进制不带 ldflags 时两边都是默认值，写死默认值的错抓不到：`make test-go-short` 以 `-ldflags -X …buildinfo.version=0.0.0-short` 构建测试并设 `NWIKI_TEST_VERSION`，测试先核对盖上了；mcpserver 另有一个握手测试读 `serverInfo`（P5 设计 2.1 第 6 条）。
- **打 tag 时不改版本号**：发布的版本由 tag 经 ldflags 注入（4.10）。
- **发布之后**，`main` 的四份默认值改为 `0.1.1-dev`（负责人第 4 问：验收发现的问题出 v0.1.1）。随之改的：`api/modules/instance.yaml:80` 的示例、`README.md:43` 写的默认值、`instance/domain/info.go:15` 的注释；`api/dist/openapi.yaml:3293`、`instance/adapter/http/gen/server.gen.go:63` 与 `web/packages/api-client/src/schema.gen.ts:2211` 经 `make gen` 随之（最后一个是 P5 设计 2.1 第 11 条补的）。四份默认值与 README 漏一处，P5/S1 的守卫就红；示例与注释照这张清单改，生成物由 `make gen-check` 核对；`platform/httpserver/apitest/apitest_test.go:19`、`:110` 是独立的假数据，不改。提交的范围写 `build, api`（设计审查 B 的 N2）。
- **Go 的模块**：`server/go.mod` 是子目录的模块，仓库根的 `v0.1.0` 不给它定版本；`go install` 出来的二进制没有内嵌前端（`webui/dist` 只提交了 `.gitkeep`）。不打 `server/v0.1.0`，发布说明不宣传 `go install`（I1 第 1 节）。
- **接口的版本**：`api/openapi.yaml` 的 `info.version: v0` 是接口的版本，与产品版本无关，不改。

### 4.10 发布流水线

照 I1 第 4 节与 D1–D7、D14（主持者的决定 322，设计审查 B 改）。

- **镜像名与标签**（I1-D1；设计审查 B 的 Q1，主持者定）：`ghcr.io/open-nerve/nervewiki`（仓库名有大写，镜像名必须小写）。只打确切的版本号：`v0.1.0` 打 `0.1.0`，rc 打 `0.1.0-rc.1`。**不打 `0.1`，也不打 `latest`**：升级要先停机、备份、`migrate up`，prod 不自动迁移；浮动的标签会诱使人拉了直接重启，新的二进制碰上没迁移的库，`/readyz` 一直 503。补丁版本同样可能带迁移（v0.1.1 很可能带验收的修复），所以 `0.1` 与 `latest` 一样。部署文档要求生产写确切的版本号，照升级的步骤换版本。
- **触发**（I1-D2）：
  - `release.yml`：`on: push: tags: ['v[0-9]+.[0-9]+.[0-9]+*']` 走发布；分支与 `main` 的推送只走演练（见下）。两种都不加路径条件，写在同一个 `on.push` 里。
  - `ci.yml` 改为 `on.push.branches: ['**']`，tag 不再触发（今天 `on: push` 不带过滤，`.github/workflows/ci.yml:3-5`，推 tag 会多跑一遍完整的 19 项）。
  - 不加 GitHub 的 environment 审批：负责人已要求推 tag 之前问他一次。
- **`ci-wait.sh` 按工作流取**：P1 之前已改好（`b54cd679`，4.2）。读 `release.yml` 的演练时第二个参数写它。读 tag 的那次运行用 `gh run list -R open-nerve/NerveWiki --workflow release.yml --branch <tag>`：tag 指的提交在 `main` 上已有一次演练，`ci-wait.sh` 取最后建的那一次，可能取到演练（P5 设计 2.1 第 13 条）。
- **闸门**（第一个作业）：
  - tag 名是 `vX.Y.Z` 或 `vX.Y.Z-rc.N`（数字不带前导的 0，别的预发布写法不收；决定 434），`VERSION=${GITHUB_REF_NAME#v}`；
  - tag 指的提交是 `origin/main` 的祖先；
  - 这个 SHA 在 `main` 上 push 的 `ci.yml` 的运行已成功：只查 `ci.yml`（`/actions/workflows/ci.yml/runs?head_sha=$SHA&branch=main&event=push`），看最后一次尝试的结论；`release.yml` 的演练绿了不算（经 `gh api`，要 `actions: read`）；
  - `make vuln` 为绿（npm 一侧照 `--audit-level=high`，4.3）；
  - `CHANGELOG.md` 里有这个版本的一节、不是空的（rc 取去掉 `-rc.N` 之后那个版本的一节；正式版本的标题要写发布的日期），在构建之前就核对：闸门照说明脚本取一次这一节（决定 434、3.6）。
  - 闸门写成一个脚本（`deploy/release-gate.sh`），可以在本机与分支上对着假的输入跑。
- **两种架构**（I1-D3）：矩阵 `ubuntu-24.04` 与 `ubuntu-24.04-arm`，各自原生构建。Dockerfile 不必改成交叉编译；smoke 在真的 arm64 上跑；公开仓库的 arm64 runner 免费。每个架构：
  1. 检出，`persist-credentials: false`；断言 `git status --porcelain` 为空：`image-smoke.sh:17-20` 照工作区算期望的 `modified`，脏的工作区也能过，发布的二进制要一定是 `modified=false`（设计审查 B 的 N5）；
  2. `docker/setup-buildx-action`、`docker/login-action`；
  3. `docker/build-push-action`，`context: .`（**路径上下文**：它默认的 Git 上下文会忽略之前对文件的改动，也不带 `.git`，要设 `BUILDKIT_CONTEXT_KEEP_GIT_DIR` 才保留；VCS 信息因此缺失、`image-smoke.sh` 会失败；在演练里核实），按摘要推送、不打 tag，`provenance: false`、`sbom: false`（用 GitHub 的证明，免得两套）；
  4. 以 `deploy/image-smoke.sh ghcr.io/open-nerve/nervewiki@<digest> $VERSION` 拉回刚推的摘要来 smoke。smoke 过的就是推上去的；失败时 GHCR 里只留一个没有 tag 的摘要。
- **合并**：`docker buildx imagetools create` 用两个摘要建出多架构的索引，先只打一个不是版本号的 tag（`sha-<提交>`）。给人用的版本号最后打（见"次序与重跑"）。
- **来源证明与 SBOM**（I1-D4、D5）：`actions/attest`，`subject-name: ghcr.io/open-nerve/nervewiki`、`subject-digest: <索引的摘要>`、`push-to-registry: true`。
  - 镜像的 SBOM 由 syft（`anchore/sbom-action`）从镜像生成，覆盖二进制里的 Go 模块。两份 SBOM 与取许可证在一个只读的作业里（`contents: read`、`packages: read`）：`pnpm install` 的安装脚本与 syft 不在能签名、能写 GHCR 的作业里跑（P5 设计 2.1 第 2 条）。
  - 前端的生产依赖另出一份：打包进二进制的 npm 包在镜像里看不出来（vite 的产物里没有 `package.json`）。用 pnpm 自带的 `pnpm --filter @nervewiki/web sbom --sbom-format spdx --prod`（SPDX 2.3；决定 438 的实验：要先装依赖，只读锁文件时许可证都是 NOASSERTION）。
  - 两份都作 attestation，也作 Release 的附件。
- **签名**（I1-D4）：`sigstore/cosign-installer` 加 `cosign sign --yes ghcr.io/open-nerve/nervewiki@<digest>`，keyless。负责人原话列了"签名、SBOM 与来源证明"三样，这样做一一对应。
- **第三方许可证**（I1-D14）：镜像里放一个 `/licenses`（Go 模块与前端生产依赖的许可证文本），同一份作 Release 的附件。理由（推断，非法律意见）：MIT、BSD、Apache-2.0 都要求分发时附上版权声明；SBOM 只列标识，不含文本。怎样收集在 P5/S2 的设计里定，只读本机的模块缓存与 `node_modules`，不往外发。
- **Release**（I1-D6；P5 设计 2.1 第 3 条改）：用 runner 自带的 `gh`，经脚本 `deploy/github-release.sh`。先建草稿：没有就建（`gh release create --draft --verify-tag`），是草稿就改说明；附件用 `gh release upload --clobber` 传；发布之后的核对过了，最后一个作业才公开它（`gh release edit --draft=false`）。已经公开的不再改，只核对附件都在；`gh release view` 的失败只有"release not found"当作没有，别的让这一步失败。原来是"没有就建、有就改，附件 `upload --clobber`"：GitHub 的 immutable releases 打开时，公开之后不能再传附件、tag 也不能再动，那样写整个失败；先草稿后公开在两种设置下都能用，核对失败时只留下一个没公开过的草稿（主持者定，P5 设计审查 B 的 Q1）。rc 加 `--prerelease`。
  - 说明由脚本 `deploy/release-notes.sh` 拼：取 `CHANGELOG.md` 里这个版本的一节；rc 取去掉 `-rc.N` 之后那个版本的一节，前面加一句"预发布"；后面接工作流生成的一段：索引的摘要与两条校验命令。脚本像闸门脚本一样在本机对着假的输入跑，缺这一节就拒（设计审查 B 的 M6）。
  - 附件：两份 SBOM、许可证。
  - 不附独立的二进制：部署文档只覆盖容器；附二进制要另做签名、证明与运行说明。
  - `v0.1.0` 不标 pre-release。
  - 不用第三方的 release action。
- **次序与重跑**（设计审查 B 的 M7）：
  1. 每个架构构建、smoke、按摘要推送；
  2. 合成索引，打 `sha-<提交>`；
  3. 按索引的摘要做来源证明、SBOM 与签名；
  4. 用单个来源 `imagetools create` 复制出 `0.1.0`（只给一个来源时原样复制，摘要不变）；已有的 `0.1.0` 指着别的摘要时这一步失败、不改指，指着同一个时不动：整次运行重跑时构建得到新的摘要（P5 设计 2.1 第 4 条）；
  5. 建或改 Release 的草稿，传附件；
  6. 两种架构上按版本号的 tag 核对（见"发布之后的核对"）；
  7. 公开 Release（P5 设计 2.1 第 3 条；原来第 5 步是最后一步，核对在它之后）。
  - 这样 `0.1.0` 出现时已经签好、证明好。包已设为公开时，不会有一段公开却没签名的 `0.1.0`。
  - 一时的失败（Sigstore 或 GHCR 一时出错）在同一次运行上"Re-run failed jobs"，不重推 tag。每个作业都能重跑：`imagetools` 对同样的来源大概得到同一个摘要（推断；不同时版本号的 tag 照第 4 步不改指），证明与签名多一份无害，Release 的草稿没有就建、有就改，已公开的不动（P5 设计 2.1 第 3 条）。产品的问题出下一个 rc，`v0.1.0` 之后出 v0.1.1（4.11）；流水线的问题出下一个 rc（决定 324），`v0.1.0` 上流水线自己的错不出 v0.1.1（4.11；P5 设计 2.1 第 3、16 条）。
- **发布之后的核对**：公开 Release 之前的那个作业是矩阵（P5 设计 2.1 第 3 条；原来是最后一个作业），在 `ubuntu-24.04` 与 `ubuntu-24.04-arm` 上各按 tag 拉索引，跑 `gh attestation verify oci://ghcr.io/open-nerve/nervewiki:<版本> -R open-nerve/NerveWiki --cert-identity <同 cosign 的身份>`（来源证明；加 `--predicate-type https://spdx.dev/Document/v2.3` 是 SBOM 的证明，镜像的与前端的都要在；P5 设计 2.1 第 5 条）与 `cosign verify …`（`--certificate-identity` 指 `release.yml` 在这个 tag 上，`--certificate-oidc-issuer=https://token.actions.githubusercontent.com`），再 smoke 一次。初稿让本机核对 arm64：rc 还是私有包时，本机拉取要负责人的令牌，本机也没装 cosign（设计审查 B 的 M8）。本机只在包设为公开之后不登录地拉一次（4.11）。版本号的 tag 在核对之前就已出现（"次序与重跑"第 4 步）；核对失败时 Release 还是没公开过的草稿。
- **演练**（I1-D7；设计审查 B 的 M4、M5，修订核对第 1 轮 B 的 M14）：
  - 单独的作业，只有 `contents: read`，用 `if` 只在不是 tag 的推送上跑。`jobs.<id>.permissions` 只能写字面值，不能随事件变；与 tag 共用一个作业定义的话，分支推送也拿到能写 GHCR 的令牌，与移交 M0-P6 第 1 条"写权限只在发布的触发条件下授予"相反。
  - 两种原生 runner 各自 `push: false`、`load: true` 构建，对本地的镜像跑 `image-smoke.sh`；核实路径上下文带上了 `.git`、VCS 信息在。
  - 在所有分支与 `main` 的每次推送上都跑，不加路径条件：`ci.yml` 的 `image` 一份只构建 amd64（`ci.yml:310-326`），Dockerfile 的 web 阶段在目标架构上 `pnpm install` 与构建，有按架构选的原生可选依赖（`deploy/Dockerfile:16-18`）。P2–P4 改 `web/`、`server/`、锁文件时，只在 arm64 上坏的构建在合并时就看得到，不等到 rc.1 才发现、又因改了产品代码要再问负责人。
  - 第一次修订写的"`main` 每次跑、分支只在改了工作流或 `deploy/` 时跑"，一个 `on.push` 写不出来：同时有 `branches` 与 `paths` 时两个都满足才跑，`paths` 对 `main` 与分支一样有效（GitHub 的 workflow syntax）。取最简单的一种（修订核对第 1 轮 B 的 M14）。
  - 代价：每次推送（分支与 `main`）多两个并行的作业，推断 amd64 约 2–3 分钟、arm64 约 2–5 分钟（`ci.yml` 的 `image` 一项构建加 smoke 是 68–80 秒，arm 的 runner 没有量过；P5/S2 在 amd64 上另加取许可证与两份 SBOM，约 1–2 分钟；P5 设计 2.1 第 1 条，原来推断各 10–15 分钟），与 `ci.yml` 并行，不进组合根；公开仓库不收钱（第 10 节）。
  - P5/S1 加一个读 `release.yml` 的检查：演练作业不许有任何 write，带反向对照。
- **权限**：只用 `GITHUB_TOKEN`，不要别的机密。顶层 `permissions: {}`，各作业按需开：

  | 作业 | 触发 | 权限 |
  |---|---|---|
  | 演练（两种架构的构建与 smoke，不推送） | 分支与 `main` | `contents: read` |
  | 闸门 | tag | `contents: read`、`actions: read` |
  | 每个架构的构建、smoke 与推送 | tag | `contents: read`、`packages: write` |
  | 取许可证、生成两份 SBOM（P5 设计 2.1 第 2 条） | tag | `contents: read`、`packages: read` |
  | 合并、证明、签名、复制出版本号的 tag | tag | `contents: read`、`packages: write`、`id-token: write`、`attestations: write`、`artifact-metadata: write` |
  | 建或改 Release 的草稿、传附件 | tag | `contents: write` |
  | 发布之后的核对（两种架构） | tag | `contents: read`、`packages: read`、`attestations: read` |
  | 公开 Release（P5 设计 2.1 第 3 条） | tag | `contents: write` |

- **新加的 action**，按 SHA 引用、注释写 tag（13.5 第 2 条；版本与 SHA 见 I1 第 4 节的表与 `survey-i1/actions-new-shas.txt`）：`docker/setup-buildx-action` v4.4.1、`docker/build-push-action` v7.4.0、`docker/login-action` v4.8.0、`actions/attest` v4.2.2、`sigstore/cosign-installer` v4.1.3、`anchore/sbom-action` v0.24.3（自带 syft v1.54.0，不是 I1 写的 v1.57.1；工作流里显式写 `syft-version`，P5 设计 2.1 第 12 条）、`actions/download-artifact` v8.0.2。实施时再核一次最新版：这些 SHA 只与 I1 的记录、现有的工作流对照过，P5 的设计审查没能以公开仓库复核，P5/S1 实施时核对（P5 设计 3.21）。`docker/setup-buildx-action` 起的 BuildKit 以 `driver-opts: image=moby/buildkit:<版本>@sha256:…` 钉住，演练与构建是同一个；能签名的作业不用 `docker/login-action`，以 shell 的 `docker login` 登录（P5 设计 2.1 第 17 条）。初稿列的 `docker/metadata-action` 没有哪一步用它（label 由构建参数传，tag 由 `imagetools` 打），删掉（设计审查 B 的过度设计）。第三方的几个要负责人在"允许的 actions"里放行（第 11 节）。
- **actionlint**：`make lint-actions`，只在 GitHub 的 runner 上跑：CI 的 `costs-1` 一份在 `make lint-go` 之后跑它，本机的 `make lint` 与逐个提交的门禁不含它，出错写成 GitHub 的注解（主持者照决定 308 的先例定：设计时在本机跑它被权限分类器拒过；P5 设计 2.1 第 10 条）。核对三份工作流，不调 shellcheck（`-shellcheck=`：runner 装了版本不由我们定的 shellcheck，actionlint 看到它才查 `run:`）。至少 1.7.10：`artifact-metadata` 权限到 1.7.10 才认，`ubuntu-24.04-arm` 到 1.7.7。同 `govulncheck`，用 `go run …@版本`，不进 `server/tools/go.mod`（设计审查 B 的 N4）。
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
| 6 | 在 `m12-release` 上写 `CHANGELOG.md` 的日期，合并（合并提交单独推上 `main`：闸门只认 tag 指的提交自己的 `ci.yml` 运行，P5 设计 3.23），`main` 的 CI 绿；推 `v0.1.0`，同第 4 步核对，本机不登录再拉一次 | 主持者 | **对外**（第 2 步已点头） |
| 7 | 在 `m12-release` 上把 `main` 的默认值改为 `0.1.1-dev`，合并，`main` 的 CI 绿 | 主持者 | 否 |
| 8 | 收尾记录写"发布"一节（负责人对两个 tag 的回答与日期、run 号、摘要、校验的输出、`m12-release` 上的每个改动）；12.6 写"实现完成，待人工验收"；推送；停下汇报 | 主持者 | 否 |

- **第 2 步的点头盖住什么**（主持者的决定 324）：rc 有问题时，修了再推 `rc.2`、`rc.3`。只改发布流水线、部署文档或发布说明的，同一次点头盖住；改了产品代码的，再问一次。收尾合并之后由闸门的 `make vuln` 发现、要升 Go 或依赖的，算改了产品代码；为此往 `audit.ignore` 加的忽略，算改了产品的依赖配置；都再问（4.3）。
- **发布期的改动走 `m12-release` 分支**（主持者的决定 332，设计审查 C 的 M6）：收尾合并之后为发布做的改动（rc 的修复、`CHANGELOG.md` 的日期、`0.1.1-dev`）都在分支 `m12-release` 上做。主持者读 diff、看分支的 CI，`git merge --no-ff` 合并进 `main`，`main` 的 CI 绿之后才推 tag。每一轮从 `main` 开分支，合并之后删。收尾记录的"发布"一节逐个列出。不直接提交到 `main`：`docs/README.md` 的 Git 一节要代码在分支上开发、门禁全绿再合并，这些改动又不在任何收尾审查之内。
- **一时的失败**：在同一次运行上"Re-run failed jobs"，不重推 tag，不算重推（4.10）。
- **rc 不过时**：不删 tag；已公开的 Release 不删、标明作废，没公开过的草稿删掉（P5 设计 3.23）；修了推下一个 rc。GHCR 里留着的 rc 镜像不影响 `0.1.0`。
- **`v0.1.0` 推了之后发现产品的问题**：不重推同一个 tag，出 `v0.1.1`。流水线自己的错（例如校验的命令写错、一个 action 换了行为）而产品没有问题时，不出 `v0.1.1`：在 `m12-release` 上修流水线，照 Release 说明里的命令手动校验，记进收尾记录；手动公开留下的草稿是对外的一步，先问负责人（主持者定，P5 设计 2.1 第 16 条）。
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
  - 4.6.6 的 Shift-JIS、Big5、EUC-KR 会被误读，与 CP437；决定 376 的两条：只有恰好合法 UTF-8 的 GBK 名字的包认不出，GBK 的包里没有标志的真 UTF-8 名字被读成 GBK（相对今天的回归）；
  - 4.6.3 的切罗基文：升级之后要跑一次 `nervewiki reindex`，已有只差切罗基文大小写的兄弟页时它报撞键、要先改名；
  - 4.6.4 的 `batch` 没有单独的期限；
  - P4 只量的与超出而经负责人改判的；
  - 运维相关的（4.8 的第 8 项）；
  - e2e 揭出的那一条：续期途中刷新会被登出（4.4）。
  - 第三方许可证里没有文本的包（决定 439，今天三个），进 `M12-release.md`。
- **v0.2 的移交**：M12 收尾时在 `docs/v0.2/handoffs/` 写（`status: open`、`from: M12`、`to: v0.2`），按主题分：
  - `M12-polish.md`：C 类 87 项与 3、12.2、11.6 的其余（附录 A）、D 类其余、Web Locks 的登记（第 7 条）；
  - `M12-performance.md`：只量的各项与它们的数、已知的改法（页面树的分层加载若没做、搜索尾巴的三种改法、按页日志的迁移、共用的参与者界线、MCP 名额的配置项、慢的发送者与 Caddy 早答的服务端改法、附件列表的重读、`batch` 单独的期限）；
  - `M12-history.md`：H1、H3、H4 的功能、版本的保留策略（带 P4 推算的增长）；
  - `M12-transfer.md`：24.1（网页的任务列表在网络结果未知时可能指到别处的任务，负责人 2026-11-14 第 5 问）；T1、T5、T8；CP437 与别的编码的识别、导入的编码选项；
  - `M12-release.md`：Node 26、功能性的大版本依赖、Dependabot（看负责人的决定）、多实例、独立的二进制、actionlint 查 `run:` 的脚本、构建的缓存、`outdated.sh` 不列的工具（actionlint、syft、cosign、BuildKit）、第三方许可证里没有文本的包、rc 与 `v0.1.0` 留在 GHCR 里的 `sha-` 与没有 tag 的摘要清不清（P5 设计 第 9 节）；续期途中刷新会被登出（重看 M1 第 10 节的取舍，设计审查 C 的 N6）。
- **v0.1.1 的移交**：`docs/v0.1.1/handoffs/M12-after-release.md`：发布之后的人工验收（4.12 的各份清单）与读屏验收要确认的八项；`vuln.yml` 的两件（定时运行的通知发给最后改 cron 的人；公开仓库 60 天没有活动时定时工作流会被停用，4.3）。
- **逐项核对去处**（修订核对第 1 轮 C 的 M9）：上面"已知限制"的每一项都在某一份移交里，P5/S3 的对照表写明是哪一份：H1 在 `M12-history.md`；24.1、T1、T5、T8、CP437 与别的编码、决定 376 的两条在 `M12-transfer.md`；`batch` 的期限与 P4 只量的在 `M12-performance.md`；续期途中刷新被登出、单实例、切罗基文标题升级之后的 `reindex` 在 `M12-release.md`；C 类在 `M12-polish.md`；H4 的功能在 `M12-history.md`。T2、T4 在 v0.1 修，不进移交。
- `docs/README.md` 的目录结构加 `v0.1.1/` 与 `v0.2/handoffs/` 两行。v0.2 的总体设计开工时先处理它们。

## 5. 接口

M12 不加 REST 操作、MCP 工具与 prompt，也不加配置项。改的是几处行为、几个 Makefile 目标与工作流、发布物。

### 5.1 REST 与 MCP 的行为

| 处 | 改动 | Step |
|---|---|---|
| 节点的标题键（`shared.TitleKey`，三处） | 切罗基文的小写在折叠之后换成大写；只差切罗基文大小写的两个标题从此冲突（`page.title_taken` 照旧） | P3/S1 |
| 阅读视图的标题 id | 含自动链接与原始 HTML 的文字；这类标题的锚点 id 变了 | P3/S1 |
| 标签列表（linking 的 `listTags`） | 不再列 `/`；`api/modules/linking.yaml` 的说明随之 | P3/S1 |
| 显示名与客户端名 | 去掉默认可忽略的字符与空白之后什么也不剩的新的显示名答 `required`，这样的客户端名当作空的；标题与库的 CHECK 不变 | P3/S1 |
| MCP 的 `log` | `path` 省略或 `"/"` 是整本；`""` 答字段错误，文字写"省略它就是整本"；`index` 不变（`parent:"/"` 仍是根）；`api/mcp/tools.json` 的说明随之 | P3/S1 |
| MCP 的工具调用超过请求的期限 | 文字说明期限已到、什么也没有写、`batch` 拆小再试；码照旧是 `internal_error`（或照 P3/S1 的设计） | P3/S1 |
| 日志与变更集的详情（`listChangesets`、`getChangeset`、MCP 的 `log`） | 各自的五条读在一个只读快照里；答复的形状不变 | P3/S1 |
| 事件流（`GET /api/v0/events`） | 一个凭证至多 4 条，超出时以 `reset`（新的原因 `superseded`）结束同一凭证最老的那条；先按凭证判，只有净增一条时才看账户的上限：一个账户至多 64 条，超出答 429 带 `Retry-After`；`api/modules/events.yaml` 的枚举与说明随之 | P3/S1 |
| 撤销（`revertChangeset`、`startRevert`） | 被这个变更集删了、本身是回收站根的节点自成一顶：回滚的变更集与"恢复之后先删里面、再删外面"的变更集照常撤销，不再答 `page.revert_conflict` | P3/S2 |
| 导入的报告 | 按 Info-ZIP 的 Unicode 路径或 GBK 解读的名字另列，带新的原因；两种语言的文字随之 | P3/S3 |
| 开始任务（导出、导入、后台的恢复与撤销） | 一个人排队与运行中的任务到上限（常量 5）时答一个新的码（`transfer.too_many_jobs`，409，不带 `Retry-After`；决定 373），两种语言的文字说"你自己已有几个任务在排队或运行"；全局的 503 `server_busy` 照旧；`api/modules/transfer.yaml` 的 `x-problem-codes` 与说明随之 | P3/S3 |
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
| `make lint-actions` | actionlint 核对三份工作流；至少 1.7.10，`go run …@版本`，`-shellcheck=` | CI 的 `costs-1` 一份；**本机不跑**（P5 设计 2.1 第 10 条） | P5/S1 |
| 工作流与发布脚本的守卫（P5 设计 2.1 第 7 条） | Go 的短测试（`platform/buildinfo/release_test.go`）：读三份工作流，演练作业不许有 write，tag 的作业的权限、`if` 与调的脚本照表；闸门、说明与发布的脚本对着假的 `gh`、`docker` 与临时的仓库跑；都带反向对照 | `make test-go-short`、`make test`（CI 的 `short` 与 `race`）；本机与 CI | P5/S1、S2 |
| 性能的目标（名字在 P4 的设计里定） | 跑 4.7 的生成器、驱动与浏览器脚本 | 只在本机手跑 | P4/S1、S2 |

- **工作流**：`ci.yml` 只在分支上跑（4.10）；`release.yml` 是新的，tag 走发布、分支与 `main` 走只读的演练（4.10）；`vuln.yml` 是新的，每天、手动与改了它自己或依赖清单的推送时跑 `make vuln`，手动触发与改了依赖清单的推送时另跑 `make outdated`（4.3）。
- **`ci-wait.sh`**（`docs/v0.1/claude_to_codex/tools/`）：按工作流取运行，默认 `ci.yml`，第二个参数可改（已在 `b54cd679`，4.2）。

### 5.4 发布物

- **镜像**：`ghcr.io/open-nerve/nervewiki:0.1.0`，`linux/amd64` 与 `linux/arm64` 的索引；只有确切的版本号，没有 `0.1` 与 `latest`；label 加 `revision`、`created`、`description`；索引带 `description`、`source`、`licenses` 三个注解，值取自 amd64 那一份的 label（GHCR 的多架构镜像从索引的注解读说明；P5 设计 2.1 第 15 条）。
- **证明与签名**：GitHub 的来源证明与 SBOM（推进 GHCR）、cosign 的 keyless 签名。校验：
  - `gh attestation verify oci://ghcr.io/open-nerve/nervewiki:0.1.0 -R open-nerve/NerveWiki --cert-identity https://github.com/open-nerve/NerveWiki/.github/workflows/release.yml@refs/tags/v0.1.0`（来源证明；加 `--predicate-type https://spdx.dev/Document/v2.3` 是两份 SBOM 的证明；要先 `gh auth login`；P5 设计 2.1 第 5 条）
  - `cosign verify ghcr.io/open-nerve/nervewiki:0.1.0 --certificate-identity=https://github.com/open-nerve/NerveWiki/.github/workflows/release.yml@refs/tags/v0.1.0 --certificate-oidc-issuer=https://token.actions.githubusercontent.com`（cosign v3，流水线用 v3.1.4）
- **Release**：说明取 `CHANGELOG.md` 的一节，加摘要与两条校验命令；附件是 `nervewiki-<版本>-image.spdx.json`（镜像的 SBOM）、`nervewiki-<版本>-web.spdx.json`（前端的 SBOM）、`nervewiki-<版本>-licenses.tar.gz`（第三方许可证，与镜像里的 `/licenses` 相同）（P5 设计 2.1 第 15 条）。
- **仓库里的文件**：`deploy/README.md`、`deploy/compose.yaml`、`deploy/release-gate.sh`、`deploy/release-notes.sh`、`deploy/release-version-tag.sh`、`deploy/github-release.sh`、`deploy/compose-initdb.sh`、`tools/licenses/`、`CHANGELOG.md`、`SECURITY.md`、`docs/v0.1/M12-release/manual/` 的两份。

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
| S1 e2e 的帮手 | 4.4：夹具里的在途记录与帮手（`goto`、`reload`、显式的逃生口）；`pageWatch` 认出 `/sign-in`；109 个故事文件与 `e2e/fixtures/wiki-health.ts:157` 换上帮手；门禁（含 `e2e/fixtures/`，数逃生口）与它的反向对照；延后续期的对照测试；等续期的锁（或租约）与这个文档的令牌（第一次续期，或在它里面登录、注册的答复）；`global-setup.ts` 的一行日志（第 50 条）；总体设计 13.4 第 3 条 | e2e、tools、build（`Makefile`、`knip.jsonc`）、docs | 能：只改 e2e，CI 的 e2e 一份证明；逃生口只有 A15 的两格（帮手的旁支加第 11 格，三格，P1 设计 3.17） | 组合根不变。e2e 作业预计多 0–30 秒（推断：多数 `goto` 之前没有在途的读；有任务时每秒轮询的设置页可能要多等几次，没有量；今天 6:52–9:23，时限 20 分钟） |
| S2 工具链与安全更新 | 4.3：Go 1.27.4（两份 `go.mod`；Dockerfile 的 tag、摘要与构建时的断言；README；文档里的五处：总体设计 13.1 第 32 条、06-environment 两处、05-lessons、03-project-background）；PostgreSQL 18.7（五处、守卫加 README）；三个镜像的摘要；两个 action；`make vuln`（Go 只扫 `server/`，npm 的 `pnpm audit --prod`）、新的 `vuln.yml` 与它报出的模块；`make outdated`；Dockerfile 的 Go 版本的守卫；两处注释的核对；总体设计 7.1 与 13.5 第 2 条 | server、deploy、e2e、CI、Makefile、README、docs | 能：一个分支、一次完整的 CI，`vuln.yml` 在同一个分支上由 `push` 触发跑一次；PostgreSQL 的五处由守卫一起改，漏一处就红 | 组合根各份照新的工具链与数据库重量：合并之后两次 `main`，过 450 秒照 06-environment 第 4 节挪份。`ci.yml` 不多跑 `make vuln`。`image` 一份要拉新的基础镜像。S2 实施：分支上多出的约 50 秒是编译（竞态的份没取回竞态的编译结果），`ci.yml` 只让竞态的九份取存 Go 的缓存（决定 355）；第三、第五份各挪一个给第八份（决定 354）；见"分份" |

- S1 先做：S2 要一次干净的完整 CI，不该被 `goto` 的偶发打红。
- **每个 Step 合并之前**（02-workflow 第 3.6 节）：主持者看一眼 `vuln.yml` 最近一次的结论（`gh run list --workflow vuln.yml --branch main --limit 1`）；定时运行的通知不发给主持者（4.3）。
- **`vuln.yml` 在 M12 期间红了**（4.3）：npm 一侧修不了或与我们无关的写进 `audit.ignore`，其余主持者开旁支 `m12-toolchain-<版本>`，照 S2 的做法升，提交写当时所在的 Phase，照常审查与合并。收尾合并之后由闸门发现的，照决定 324 再问负责人。

### P2 网页的打磨

每个 Step 的逐项清单在附录 A。每个 Step 自带它加的文字的两种语言（13.2 第 4 条）、假服务端的字段与 knip 的干净。

| Step | 内容 | 改的部件 | 单独合并为绿 | 组合根与 CI 的用时 |
|---|---|---|---|---|
| S1 焦点与共用组件 | `ConfirmDialog` 发送中改用 `aria-disabled`，被拒之后焦点留在确认按钮上（丢了就还给它）（17 个文件、25 处 `<ConfirmDialog`）；`useForm` 的拒绝只有横幅、而焦点已丢时，焦点给横幅（`web/…/components/form-field.tsx:89-97`）；`button.tsx` 加 `aria-disabled:` 的样式（23.17、28.2）；到达的焦点（12.3、23.14、26.4、28.4、28.5）；其余的焦点项；撤销对话框的两句说明进 `aria-describedby`（24.3、49）；第 7 条"仍要退出吗"；总体设计 13.2 第 12、26、1 条改写 | web、e2e、docs | 能 | 组合根不变；vitest 多几十个测试；e2e 改到的故事照需要进 `threeBrowsers`，推断多 1 分钟以内 |
| S2 文字 | 单复数（35.1、41.x，每个单数一个 `*One` 键）；根下的叫法（22.11、23.3）；撤销的用词（23.1、23.4、23.11、24.7）；其余误导的文字；第 40 条两份 `AGENTS` 模板；H4 的界面一句（不写版本号） | web、server（`wiki/domain/template-*-agents.md`） | 能 | 组合根不变（模板的测试在 wiki 模块里）；vitest 多一些 |
| S3 375 px、`bdi` 与对比度 | flex 项补 `min-w-0` 或 `[overflow-wrap:anywhere]`（26.17、32.x、35.8、38.10–38.12）；从右到左的名字进 `bdi`（26.12、32.4）；焦点环与补全说明、暗色高亮的对比度（12.4、26.9）；32.3 MC3 的 375 检查；33.1 在 webkit 上排查（复现了，在 S3 修，决定 589） | web、e2e | 能 | 组合根不变；e2e 加几处 375 px 的检查，33.1 的排查不进套件 |
| S4 阅读视图（**先写 spec**） | `reading-view.tsx:133-232` 那个 layout effect 的重写：更新不整段重写 `innerHTML`、不失焦（3、8.2、12.2、29）；每小时的整篇替换与全屏退出（17.1、17.8）；`editable` 与 `role` 不进那个 effect（48，改法见移交第 48 条）；后退前进到锚点（8.1）；mermaid 的 id（11.1）；失去角色后的勾选（6.2）；换语言时的失焦（11.6）；按职责拆开（14d） | web、e2e、server（只加 Go 测试与 `testdata/m12` 的样例） | 能；要对照 M6 以来依赖 `editable` 的增强逐一核对（移交第 48 条） | 组合根不变；e2e 加阅读视图的焦点与滚动检查，三种引擎（05-lessons 第 6 节；新故事 PG15，UP1 (page) 加第 48 条的一段） |
| S5 编辑器、附件、上传、粘贴与导入导出 | 附件一节、阅读视图里的附件、编辑器的上传与粘贴、导入对话框与任务列表的 A、B（附录 A）；上传的超时取多长在 S5 的设计里定（16.3） | web、e2e、server（`platform/markdown/obsidian` 的一个属性） | 能；可按"附件与上传""编辑器""导入导出与来源""阅读视图里的附件"分四组提交 | 组合根不变 |
| S6 版本、回收站、历史、目录、后台任务与搜索的网页 | 22.14 与 23.10 的"在途的写进 store"；12.12 的 `Math.max(...)` 与 `push(...)` 的参数上限（`web/…/stores/page-tree.ts:116`、`:124`）；27.2；删除与新建之后的导航（2）；其余 A、B；24.8、24.10（TB2 的偶发）、26.15 | web、e2e | 能；**排在 P4/S2 的页面树结论之后**；可按版本与回收站、历史与目录、搜索分组 | 组合根不变 |

`en.ts`、`zh-CN.ts` 几乎每个 Step 都改：一次合并一个，后合并的先 `git merge main` 解决冲突（02-workflow 第 3.6 节）。

### P3 服务端的正确性与历史

| Step | 内容 | 改的部件 | 单独合并为绿 | 组合根与 CI 的用时 |
|---|---|---|---|---|
| S1 小的正确性 | 25.1（三处折叠，删 `searchFold` 的映射）、10.2、10.1、30.2（整个名字的判断）、`log` 的 `path` 约定（30.4、30.5）（4.6.3、4.6.4）；MCP 调用超时的文字（4.6.4）；H5 的快照，日志与变更集的详情（4.6.2）；每个凭证与账户的流数（4.6.5）；13.1 第 28、30 条 | server、api（说明）、web（若有新的文字；事件流的 429） | 能 | 单元与适配器的测试为主；流数的整个程序的测试点名进第七或第八份，名字避开第二份的模式（"分份"），推断多 5 秒以内 |
| S2 第 302 条（**先写 spec**） | 4.6.1：`readReverted`/`topOf` 把本身是回收站根、被这个变更集删的节点当作它自己的顶，几处传 `Nodes`；后台撤销的 `revertSteps`/`needed`；两个性质测试扩开；第三种形状的一格；nested 一格翻过来（两本笔记本，第七、第八份的模式一并改）；总体设计 5.8 第 6 步与 13.1 第 2 条 | server（page 的 domain 与 app） | 能；nested 一格在同一个提交里翻过来 | 两个性质测试在 page 的 app。`make test-go-race` 对组合根之外的包不带 `-short`（`Makefile:179`），代价在 `race` 一份：各多至多约 15 秒，以同类为参照（`TestTheJobsStepsReplayToStar` 竞态下 13.99 秒、`TestTheRollbacksStepsReplayToTheBaseline` 1.85 秒）；`-short` 的次数照 `TestTheJobsStepsReplayToStar` 的 2,000 对 400（设计审查 A 的 M8）。组合根只改第八份已有的测试，推断不变 |
| S3 导入导出的任务 | 4.6.6：名称的编码（先读 0x7075，再 GBK 回退，整包的判断）、报告的新原因与两种语言的文字、差分的测试；按人的队列上限（T2）；启动时收拾上一个进程取到而没开始的任务（T4）；`README.md:193` 那一句 | server（transfer）、api、web | 能 | 适配器与单元的测试；T4 并进第四份已有的 `TestServeFailsTheJobsLeftRunning`，已有的种子带 River 的行（P3 设计 3.14.3；P3 设计修订核对 C 的 M6） |

P3 的 S1、S2 与 P2 除翻译文件外不重叠，可以并行；翻译文件照上面一次合并一个。S3 的网页部分改的文件（`problem-messages.ts`、`transfer-report.tsx`、`trash-row.tsx`，导出、导入、后台撤销与回收站的测试）P2 的 S1–S6 也改：S3 的网页部分在 P2/S1 合并之后写，谁后合并谁先 `git merge main`、重跑 `make test-web`（P3 设计第 0 节；P3 设计审查 C 的 M5）。S1 与 S2 都改 page 模块，S2 在 S1 之后开，或在合并时 `git merge main`。S3 只改 transfer，与 S1、S2 不重叠。

### P4 性能与内存实测

| Step | 内容 | 改的部件 | 单独合并为绿 | 组合根与 CI 的用时 |
|---|---|---|---|---|
| S1 设施（Go） | 4.7.4：搜索语料（三个 TSV）、导入用的 vault 的生成器（含 GB 级）；现有守卫的规模开关；lint 守卫的 worker 期限提到界之上（决定 359 的做法，M10 决定 160 留下的）；回收站两个基准的档位开关；`server/tools/perf/` 的驱动（有预算的几项要的）与采样 | server（`_test.go` 与 `server/tools/perf/`）、Makefile | 能：没有环境变量就跳过；语料的三个测试在 `!race` 的文件里，`test-go-costs-3` 单独一行；lint 要过 | 竞态的八份不变（只多两个毫秒级的短测试，在第四份）；`test-go-costs-3` 多一行，7763 上推断约 15 秒（P4 设计 2.1 第 8 条、7.5） |
| S2 浏览器侧的设施与实测（**页面树先**） | `e2e/perf/` 的脚本与配置，knip 读到它的配置；先量页面树（4.7.2，量之前把界线写进 P4 的设计），结论先报主持者；再量 M4-performance 第 7、9、11、12 条与 M7-transfer 第 4 条；记录 | e2e（perf；`package.json` 的脚本与两个 devDependencies；`fixtures/auth.ts` 导出引导的步骤表）、knip（`e2e` 一节忽略 `vite`）、docs | 能：脚本不进 `make e2e`；记录只改文档 | 不变 |
| S3 服务端实测（P3 合并之后） | 4.7.3 的批次 B–G（H 看负责人的回答）；有预算的每一行对照 4.7.1，写明判在哪台机器上；部署文档要的数（含硬件下限、10 万页的树被几个标签页重读）；只量的推算；已知限制与 v0.2 的清单；总体设计第 14 节与 13.4 第 8、9 条写着 M12 的各句照结论改 | docs | 能：只改文档 | 不变；本机的机器时间约 16–20 小时、有条件的另约 4.5–6 小时（推断，P4 设计 3.8；比初稿估的 10–16 小时多），分几夜独占 heavy 的锁 |
| S4 起：条件的修复 | 只在超出 4.7.1 或 4.7.2 的判据时开，各自先写设计、带缩小规模的守卫。可能的：页面树的改法、参与者的共用界线（B4）、附件列表的重读（I2-D10 判为 A 时）、搜索的回归、B5 的规则一层没过时经主持者与负责人定下的做法（降常量，或写进部署要求） | 看改法 | 每个都要能 | 守卫照 13.4 第 8 条进成本或守卫的一份 |

S2、S3 的"实测"Step 照常有分支、审查与合并：审查者核对方法、种子与脚本，抽一两项在自己的副本上重跑，不重跑全部。S2 与 S3 可以写在同一时段，跑要串行（heavy 的锁）。

### P5 部署文档与发布流水线

| Step | 内容 | 改的部件 | 单独合并为绿 | 组合根与 CI 的用时 |
|---|---|---|---|---|
| S1 版本号与流水线的构建部分 | 四份默认值的守卫与 MCP 版本的短测试（4.9）；Dockerfile 的三个 label 与 `make image` 传它们；`ci.yml` 只在分支上跑；`release.yml` 的闸门（`deploy/release-gate.sh`，只认 `ci.yml`，带"`ci.yml` 红、`release.yml` 绿"被拒的反向对照，核对 `CHANGELOG.md` 的一节）、说明脚本（`deploy/release-notes.sh`）、两种架构的构建与 smoke；只读的演练作业（每次推送）与读 `release.yml` 的权限检查；actionlint 在 CI 上（`make lint-actions`，本机不跑） | deploy、CI、Makefile、server（守卫在 buildinfo 与 bootstrap 的测试） | 能：演练作业在分支上证明两种架构为绿；不推送 | 组合根不变；每次推送（分支与 `main`）多两个并行的演练作业（推断 amd64 约 2–3 分钟、arm64 约 2–5 分钟，与 `ci.yml` 并行；P5 设计 2.1 第 1 条） |
| S2 推送、签名、证明、SBOM、许可证与 Release | 合成索引（先打 `sha-<提交>`）；`actions/attest`；cosign；镜像与前端两份 SBOM；`/licenses`；最后复制出版本号的 tag；Release 先建草稿、传附件；发布之后的核对作业（两种架构的矩阵）；核对过了才公开 Release（P5 设计 2.1 第 3 条） | CI、deploy | 合并时为绿（分支上不触发这部分）。这条路径**只有推 tag 才证明得了**，所以要 4.11 的 rc | 不变 |
| S3 部署文档与发布说明 | `deploy/README.md`（含硬件下限、从 `README.md:534` 挪来的数）；`deploy/compose.yaml`（`migrate` 在 profile 里）与 `make deploy-check`；PostgreSQL 镜像的守卫把 README 那一项换成 `deploy/README.md`、加 `compose.yaml`；根 README 缩写与改地址；第 7 节"移交的落实"的每条部署项；开发版的附录（看负责人的回答）；`CHANGELOG.md` 的 v0.1.0 一节（已知限制取自 P2–P4，逐条对照 4.13）；`SECURITY.md`；`manual/` 的读屏清单与汇总入口；用 `compose.yaml` 安装、升级、备份恢复的演练；总体设计第 11 节 | docs、deploy、README、CI、server（PostgreSQL 镜像的守卫） | 能：只改文档、一个检查与一个守卫 | `image` 一份多 `make deploy-check`，几秒 |

P5/S1 在 P1 之后就能开，与 P2–P4 并行。S3 等 P4/S3 的数。

### 收尾与发布

照 02-workflow 第 4 节与 4.11：

1. 三位审查者并行：A 服务端（跨 Phase 的一致性、302 的修法与撤销的其余路径、任务的入口与启动）；B 网页与 e2e（打磨的回归、三种引擎的焦点）；C 完成标准、文档、移交的落实与发布流水线（对照第 3 节逐条，对照代码核对每份移交）。
2. 修复在分支 `m12-closeout`，提交写 `(M12 closeout)`；核对一轮一轮做到干净；合并，`main` 的 CI 绿。
3. 处理移交：六份改为 `done`；写 `docs/v0.2/handoffs/` 的五份与 `docs/v0.1.1/handoffs/` 的一份（4.13）；P5/S3 之后才合并的 P2 的已知限制补进 `CHANGELOG.md`（P5 设计 2.1 第 9 条）。
4. 4.11 的第 2–7 步；发布期的改动在 `m12-release` 分支上（主持者的决定 332）。
5. 收尾记录 `reviews/M12-closeout-review.md`，加"发布"一节：负责人对两个 tag 的回答与日期；`m12-release` 上的每个改动；run 号、摘要与校验的输出；P1/S1 之后 `main` 的 e2e 红过的每一次与总次数（主持者的决定 334）。12.6 写"实现完成，待人工验收"；推送；停下，给负责人写中文汇报（02-workflow 第 6 节）。

### 次序与并行

逐条列出依赖（初稿的图有两处歧义，设计审查 C 的 N4）：

- P1/S1 在设计定稿之后开；P1/S2 在 P1/S1 合并之后开。
- P1/S2 合并之后，可以开 P2/S1–S5、P3/S1、P4/S1、P4/S2、P5/S1。
- P2 的 Step 一次合并一个，后合并的先 `git merge main`，不按编号；只守真的硬依赖，见 [P2 设计](02-P2-web-polish.md) 0.1。
- P2/S6 等 P4/S2 的页面树结论；结论要改树时，等改法的 Step。结论是要改（平的形状超），P2/S6 排在 P4/S4 之后（决定 510）。
- P3/S2 在 P3/S1 之后开，或在合并时 `git merge main`；P3/S3 不等 S1、S2。
- P4/S2 不等 P4/S1（只用 TypeScript 与 e2e 的帮手，P4 设计第 0 节、2.1 第 12 条）；P4/S3 在 P3 全部合并、P4/S1 与 P4/S2 之后；P4/S4 起的条件修复在它们的结论之后。
- P5/S2 在 P5/S1 之后；P5/S3 在 P4/S3（与它引出的条件修复）、P5/S2 之后，不等 P2 全部合并：P2 合并晚于 P5/S3 的部分（推后的 C 类等），收尾时补进 `CHANGELOG.md`（主持者定，P5 设计 2.1 第 9 条）。
- 收尾在 P2–P5 全部合并之后；发布在收尾合并之后。

其余：

- 同时在实施的 Step 不多于两个：本机 8 核，重的运行串行，两个 Step 的门禁与审查已经排队（06-environment 第 3 节）。等页面树结论的 P2/S6 在结论之前不开，不占名额（P2 设计 0.1）。
- P4/S3 的实测要独占机器：安排在没有别的 Step 在实施与审查的时段，例如夜里。
- 页面树的结论若要改树，改法的 Step 排在 P2/S6 之前，P2/S6 随之等。

### 分份

- 组合根八份。收尾之后两次 `main`（`05dac827` 的 run 43339247136、`a057b6ad` 的 run 43344369124）的平均：325、326.5、402.5、312、425、326、257、267 秒（[M11 收尾审查](../M11-upgrade/reviews/M11-closeout-review.md)"守卫与分份的数"一节）。
  - 第五份两次都在 EPYC 7763 上（427、423），平均 425 秒，余量 25 秒。
  - 第四份这两次都不在 7763 上（339、285），平均 312 秒；再前两次（`07abd0f7`、`9e787bab`）都在 7763 上，平均 413.5 秒。
  - 第七、第八份的 257、267 秒是两次都不在 7763 上的数；在 7763 上是 315、318 秒（`9e787bab` 那一次）。
- M12 新的整个程序的测试**逐个点名进第七或第八份的模式**（`Makefile:296`、`:328`；`194ca350` 上是 `:321`、`:365`），照收尾时 `TestRevertingARollback…` 的先例（主持者的决定 328）。
  - 第四份是补集，第五份收 `TestThe(MCP|Lint|WikiTools)`、`TestRebuild` 等（`Makefile:263`；`194ca350` 上是 `:288`），新测试的名字要避开它们。
  - 第二份是 `-run SECOND -skip FIFTH`（`Makefile:342`；`194ca350` 上是 `:379`），不跳过第七份：名字以 `E`、`Source`、`TheMCP[C-Z]`、`TheContents` 开头的新测试点名进第七份之后，会在第二份再跑一遍。事件流现有的整个程序测试叫 `TestEvery…`（`bootstrap/events*_test.go`），P3/S1 的流数测试不照这个习惯起名。避不开时，第二份的 `-skip` 加第七份的模式，同 M11 收尾给第四份加的（设计审查 A 的 N5）。
- P1/S2 合并之后两次 `main` 重看各份；过 450 秒照 06-environment 第 4 节先挪给邻份，挪不下再分一份，不提高 `-timeout`。
- P1/S2（决定 354、355）：分支 CI 上组合根在 7763 上 417–458 秒，多出的约 50 秒是编译：竞态的份取回的缓存里没有竞态的编译结果（第一、二次没有缓存，第三到五次取回 `vuln.yml` 先存下的 3 MB），包的时间没变。S2 之前 `main` 的键是 `bootstrap-1` 存的；改了 Go 与 `go.sum` 之后，不改的话由最先跑完、不带竞态的 `short` 存。`ci.yml` 从此只让竞态的九份取存 Go 的缓存，`vuln.yml` 不取不存。照包的时间折算，第三、第五份各挪一个给第八份，暖的时候 7763 上第三、四、五份约 396、396、382 秒（[P1 设计](01-P1-foundation.md) 3.16、10.2）。合并之后第一次 `main` 是冷的。

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
| 9 编辑器解析的上限 | P4/S2，照 B9 判：超，改法在条件的 Step P4/S5（决定 511） |
| 10 ANALYZE、通用计划、连接池代理 | P4/S3 只量；恢复之后先 `ANALYZE`、只能会话模式，写进部署文档，P5/S3 |
| 11 阅读视图的大图 | P4/S3 只量（P2/S4 合并之后；S2 时没合并，决定 518）；上传的像素上限进 v0.2 |
| 12 附件列表的重读 | P4/S2 复测（I2-D10）：能稳定让用户的保存失败就是 A，开条件的 Step 修；否则已知限制。S2：第一次超、复核过，记成已知限制（决定 513） |
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
| M 类 14 项与读屏推断的八项 | 发布之后的人工验收（4.12），读屏推断的八项照负责人 2026-11-14 第 6 问，确认有问题的在 v0.1.1 修；33.1 在 P2/S3 复现、修好（Playwright 的 WebKit），真的 Safari 上仍在发布之后看（决定 589、590） |

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
6. **13.5** 加第 6 条，随 P5/S1、S2 改（S1 写到"合并之后才推 tag。"为止，S2 在末尾加后一句；P5 设计 2.1 第 18 条改，比定稿时多了 README、`CHANGELOG.md`、actionlint、写权限只在 tag 的作业里、版本号的 tag 不改指与 Release 先草稿）：`6. **版本号与发布**（M12）：版本号只由 ldflags 注入，\`Makefile\`、\`buildinfo\`、Dockerfile 的两个 \`ARG\` 与 README 写的默认值相同，由测试核对；发布之后改为下一个补丁版本加 \`-dev\`。发布由推 \`vX.Y.Z\` 或 \`vX.Y.Z-rc.N\` 的 tag 触发 \`release.yml\`，闸门只放过 \`main\` 上 \`ci.yml\` 已绿、\`make vuln\` 为绿、\`CHANGELOG.md\` 有这一节的提交；依赖的漏洞扫描另在每天的 \`vuln.yml\` 里跑，actionlint 在 \`ci.yml\` 里，都只在 runner 上。写权限只在 tag 触发的作业里，演练只读，由测试读工作流核对。推 tag 之前问负责人；收尾之后为发布做的改动在 \`m12-release\` 这样的分支上，合并之后才推 tag。 镜像两种架构，带来源证明、两份 SBOM 的证明与签名，镜像里有第三方许可证；只打确切的版本号，不打 \`latest\` 与浮动的"大版本.小版本"，版本号的 tag 在证明与签名之后才打、已有的不改指；Release 先草稿，两种架构上核对过才公开。`P5/S1 的文档提交落了前一半（到"合并之后才推 tag。"，新文字照 P5 设计第 8 节第 5 条）；P5/S2 的文档提交落了后一句（P5 设计 10.2）。
7. **第 11 节**，随 P5/S3 改：
   - 第一段，从 `**M10/P3/S1升级补充**` 到 `不以暖样本承诺生产停机。`（各恰好出现一次）整段换成 `部署、备份、升级与回退的步骤在 [deploy/README.md](../../deploy/README.md)（M12）；这里只列要点。`
   - 最后一段，从 `- **M10/P3来源表示升级（决定62）**` 到 `§3.0、§8.1a）。`（各恰好出现一次）整段删掉；开发版的升级若要写，在 `deploy/README.md` 的附录。
   - `**单实例**：v0.1 官方只支持单实例（附件在本地磁盘）。` → `**单实例**：v0.1 只支持单实例：附件在本地磁盘，限流、大请求体的名额与正在上传的导入只在进程内计数（M12）。`
   - `不可写时拒绝启动（M7）。` → `不可写时拒绝启动（M7）。M12 起发布到 \`ghcr.io/open-nerve/nervewiki\`：amd64 与 arm64，标签只是确切的版本号，不打 \`latest\`；带签名、SBOM 与来源证明，校验的命令在 deploy/README。`
   - `恢复时先恢复数据库，再放回附件目录。` → `恢复时先恢复数据库，再放回附件目录；恢复之后先 \`ANALYZE\` 再开放服务，命令与演练见 deploy/README（M12）。`
   - 另有四处写着 `README 的部署一节`（`86565dd6` 上第 11 节的 `:923` 两处、`:929`，第 14 节的 `:1311`）：S3 把那一节缩成入口之后它们落空，改为 deploy/README（第 11 节里的写成链接 `[deploy/README.md](../../deploy/README.md)`；第 14 节的那一处不单独改，由第 9 条的 SSE 一行整句替换完成）；S3 时核对第 11 节里恰好三处、第 14 节里恰好一处（P5 设计 2.1 第 19 条，P5 设计审查 C 的 M8）。
8. **5.8 第 6 步与 13.1 第 2 条**，随 P3/S2 改：
   - `回滚自己的变更集在"先删的子树落在收着基线节点的新建节点下"时也答冲突（M11 第 302 条，M12 修）。` → `回滚自己的变更集照常可以撤销（M11 第 302 条的冲突由 M12/P3/S2 修好，[M12 总设计](M12-release/00-M12-design.md) 4.6.1）。`
   - 13.1 第 2 条里从 `回滚的变更集由几遍、几个单元在不同时刻写下` 起的那一句（到 `第 6 条）。` 为止）换成照 P3/S2 实情写的一句：被这个变更集删了、现在本身是回收站根的节点，在撤销计划里是它自己的顶；后台撤销的分段随之。P3/S2 的文档提交落了（新文字照 P3 设计第 8 节第 3、4 条）。
9. **第 14 节**，随 P4/S3、P5/S3 改：写着 M12 的十一行（搜索、渲染的内存、`FOR SHARE`、解析预算先到先得、lint 与 `index.md`、`NOTIFY`、保存串行与改名、附件与导入导出、页面树、版本的增长、升级的冻结时长）照实测的结论改写："M12 实测"与"M12 压测"改为量得的数、结论与已知限制。旧文字的锚点：`M12 压测`、`M12 实测；不达标时改为分层加载`、`v0.1 全部保留；M12 实测后在 v0.2 设计保留策略`、`配额与负载留给 M12（M7 写的移交）`、`10 万页与 30 万页由 M12 实测`，各恰好出现一次；其余几行在 P4/S3 的文档提交里照同样的办法先核对。13.4 第 8 条里的 `进 M12 的性能移交（00-M11 第 232 条）`、`检查的十万页在 M12` 与第 9 条里的 `旧generic返回2000却扫52000的失败和未成功实验仍移交M12`（各恰好出现一次）一并照结论改成量得的数或已知限制（设计审查 C 的 M7）。`NOTIFY` 与 SSE 两行不是实测的事，归 P5/S3 改、P4/S3 不改它们（主持者定，P5 设计审查 C 的 M7 与修订核对 C 的 N14，同 P4 设计的 P4-26）：锚点 `升级 PostgreSQL 大版本时复核（论证见` 与 `两份配置写在 README 的部署一节，M12 的部署文档沿用。`（各恰好出现一次），新文字写复核的步骤在 deploy/README 的"升级与回退"、两份配置在 deploy/README 的"反向代理与 HTTPS"（P5 设计 2.1 第 20 条、第 8 节第 5 条）。页面树一行（锚点 `M12 实测；不达标时改为分层加载`）P4/S2 的文档提交落了（P4 设计第 8 节第 5 条，数照 P4 设计 10.2）；其余几行仍随 P4/S3、P5/S3。P4/S3 的文档提交落了（九行：搜索、渲染的内存、`FOR SHARE`、解析预算先到先得、lint 与 `index.md`、保存串行与改名、附件与导入导出、版本的增长、升级的冻结时长，各写量得的数、结论与已知限制，行末指向 P4 设计 10.3；页面树一行在 P4/S2、S4 落了；13.4 第 8 条另改了"十万个来源在 M12"；P4 设计第 8 节第 5 条）。
10. **13.1 第 28 条（标题键）与第 30 条（实时推送）**，随 P3/S1 改：标题键加切罗基文那一步；实时推送加每个凭证的上限与重置、每个账户的硬上限与 429。P3/S1 的文档提交落了（新文字照 P3 设计第 8 节第 1、2 条）。
11. **13.2 第 12 条**，随 P2/S1 改：从 `` `ConfirmDialog` 发送中仍用 `disabled` `` 起、到它之后的第一个 `第 23 条）。` 为止的那一句（起头恰好出现一次；中间是指向 `M12-release/handoffs/M5-polish.md` 的链接）→ `` `ConfirmDialog` 在 M12 照此（M12/P2/S1）：发送中"确认"与"取消"用 `aria-disabled`（"确认"另带 `aria-busy`），忙的守卫让一次按下至多一个请求，Esc 与"取消"在发送中不关闭对话框；焦点不动：按下"确认"时它拿着焦点的（键盘，Playwright 的三种引擎用鼠标也是），发送中与被拒之后仍在它上面（丢了就还给它）；浏览器不给点过的按钮焦点的（Safari），焦点留在原处；拒绝由对话框里的 `role=alert` 说出。`useForm` 的表单发送中仍禁用按钮：失败而没有 `aria-invalid` 的字段（只有横幅说的拒绝），且拒绝到达时焦点已丢（在 `body`，或在发送中禁用的元素上）时，焦点给横幅；焦点没丢的（撤销的确认框）、读者放到别处的不动（第 26 条）。`aria-disabled` 的按钮与禁用的一样变暗（`components/ui/button.tsx`）。 ``（P2 设计 2.1 第 2 条、8.2）。P2/S1 的文档提交落了，"焦点不动"一句照决定 509（新文字照 P2 设计 8.2 的 S1 第 1 条）。
12. **13.4 第 3 条（端到端测试）**，随 P1/S1 改：加一句（P1 设计第 8 节第 5 条；S1 照实施改了几处：锁先于令牌（没有 Web Locks 时是租约）、令牌的答复、门禁核对读到了帮手；A15 一句多了锁里写记录的登录、租约与自己注册的文档不必等，P1 设计 3.15）："M12/P1/S1：e2e 换文档只经 `e2e/fixtures/navigation.ts` 的 `goto` 与 `reload`：它们先等页面没有在途的接口请求、续期的锁（没有 Web Locks 时是租约）空着、这个文档有了给它令牌的答复（第一次续期，或在这个文档里登录、注册），再换；有意在请求途中离开的写显式的逃生口 `{ inFlight: "leave" }`。门禁 `tools/e2e-navigation/check.mjs`（`make lint-web`）不许 `e2e/` 里别处直接调 `page.goto`、`page.reload`，按表核对每个文件的逃生口，并核对它读到了帮手的模块；同一个文档里的后退与前进不经帮手。A15 演示续期途中离开会被登出（M1 第 10 节），并核对帮手等的每一样（含在锁里写记录的登录、没有 Web Locks 时的租约）、自己注册的文档不必等；只有页面版本，接口那一面是 A5 (API)；在三种浏览器里跑，第一次续期发出之前离开的那一格只在 Chromium 与 Firefox（WebKit 在请求下一个文档之后旧文档发不出请求）（M12/P1）。"
13. **1.2** 的末句，收尾时：`以下几项明确留给 v0.2：同步 CLI` 起的那一句末尾，加 `；M12 的已知限制与推后的事项在 [v0.2 的移交](../v0.2/handoffs/)`。
14. **第 15 节**：设计定稿时加一行，写这些修订与依据；之后每次修订各加一行。
15. **00-M11 第 302 条**（不是总体设计，同一个提交里改）：`负责人若要放弃也输入名称，在 M12 改。` → `负责人若要放弃也输入名称，在 M12 改。（M12 更正：这里的"v0.1 之内不修"本意是"M11 之内不修"；照负责人 2026-11-13 第 1 问（它点名了 302）在 M12 修，[00-M12](../M12-release/00-M12-design.md) 4.6.1。）`P3/S2 的文档提交落了。
16. **`docs/README.md`**：设计定稿时，Git 一节 `- M11 同 M10（…）` 之后加 `- M12 同 M11（[M12 总设计](v0.1/M12-release/00-M12-design.md)第 7 节），另有发布的 \`(M12 release)\`。`；收尾时目录结构加 `v0.1.1/` 与 `v0.2/handoffs/` 两行。
17. **13.2 第 26 条与第 1 条**，随 P2/S1 改（设计审查 C 的 M7）：第 26 条从 `无主笔记本接管之后的焦点（M3）还没有照此` 起的那一句（恰好出现一次）照 22.17 修好之后的实情改写，不再写"v0.1 之后"；第 1 条里 `M12 的体验移交第 7 项`（恰好出现一次）照第 7 条"仍要退出吗"的结论改写。P2/S1 的文档提交落了（新文字照 P2 设计 8.2 的 S1 第 2、3 条）。
18. **13.1 第 32 条**，随 P1/S2 改（设计审查 B 的 M1）：从 `go1.27.1 里它在默认打开的 GOEXPERIMENT` 起的那一句（恰好出现一次）照 1.27.4 里 `jsonv2` 的核对结果写。
19. **13.2 第 21 条、第 17 条与第 26 条到达的那一句**，随 P2/S1 改：退出登录先问；链接随它所在的页一起离开的带 `arrived`、树打开的页带 `opened`；迟挂上的标题与代替这次导航已显示的页的"页面不存在"只从无人处取。新文字在 [P2 设计](02-P2-web-polish.md) 8.2 的 S1 第 4–6 条，旧文字各恰好出现一次。P2/S1 的文档提交落了（第 17 条补了"连同'目录''历史'两行"，第 26 条那一句照决定 507 补了换查询或片段的 `replace`）。
20. **13.2 第 17 条**，随 P2/S2 改：`根下的页面带笔记本名（` 起的那半句加根下的页的写法（`page.nameAtTop`，9.5）。新文字在 P2 设计 8.2 的 S2。P2/S2 的文档提交落了（示例照决定 584 写"Install（笔记本的顶层）"，不带笔记本名）。
21. **13.2 加第 31 条（名字、窄屏与对比度）与 13.4 第 3 条**，随 P2/S3 改：第 31 条插在第 30 条末句之后（设计时写第 30 条、插在第 29 条之后；P4/S4 先合并、取了第 30 条，决定 505、536、593）；13.4 第 3 条加 `webkit-narrow` 与项目配置的检查。新文字在 P2 设计 8.2 的 S3 与 13.4。P2/S3 的文档提交落了（第 31 条另写了原生选择的 `contain-paint`、WebKit 的下拉、destructive 按钮的白字与祖先的过渡，P2 设计 10.3）。
22. **13.2 第 23 条与 13.4 第 3 条**，随 P2/S4 改：`HTML 换掉时倒序清理、再重跑；` 与开始了的音视频的那半句照阅读视图的就地更新改写；13.4 第 3 条加 PG15。新文字在 P2 设计 8.2 的 S4 与 13.4。P2/S4 的文档提交落了（第 23 条另写了焦点在映射到的块与同一处插进的近旁几块里找、滚动区不拿走给它的 `tabindex`、写入之前就在的音视频留在原处；另改第 26 条：锚点的再看与 `view-anchors.ts`，主持者定；P2 设计 10.4）。
23. **13.2 第 7、12、21、27 条**，随 P2/S5 改：第 27 条加下载链接在点击时比到期与上传的超时；第 7 条加睡醒之后的重读；第 12 条加粘贴对话框的先问；第 21 条加上传在途时离开先问。新文字在 P2 设计 8.2 的 S5。P2/S5 的文档提交落了（第 27 条的起点照决定 620 写"请求体开始发出之后"；第 12 条另写问的时候焦点在"继续编辑"上；第 21 条另一处照 S5 审查 C 的建议改；P2 设计 8.2 的 S5、10.5）。
24. **13.2 第 12 条**，随 P2/S6 改：`不在选中时发出（方向键移动就会选中）。` 之后加关着的原生选择等读者停下再用（26.5）。新文字在 P2 设计 8.2 的 S6。P2/S6 的文档提交落了（括号另写了一项项换的键、macOS 上只有打字找选项的字符算一步（决定 627）、用等着的选择的回车不再打开列表（决定 630）与只在 Linux 上核实过；P2 设计 10.6）。
25. **13.4 第 8 条**，随 P4/S1 改（P4 设计 2.1 第 15 条）：`例外是十万页摘要的成本用例，请求 6 s、worker 3 s（决定 160）` → `例外是十万页摘要的成本用例，请求 12 s、worker 6 s（决定 160；M12/P4/S1 起，期限由界推出、断言界低于期限，M12 决定 475）`；第 15 节随之加一行，同决定 359 的先例。P4/S1 的文档提交落了（新文字照 P4 设计第 8 节第 5 条，逐字）。
26. **13.2 加第 30 条（页的列表列出全部，分批画；P2/S3 未合并、P4/S4 先合并，号照决定 505，决定 536）与第 14 节"大笔记本（上万页）"一行补 P4/S4 之后的一句**，随 P4/S4 改：第 30 条插在第 29 条末句之后；第 14 节那一句接在 P4/S2 写下的那一句之后、行末的后缀之前。新文字在 P4/S4 设计第 8 节第 1、2 条。P4/S4 的文档提交落了（数照 P4/S4 设计第 10 节）；P2/S3 的新条合并时取第 31 条，在它自己的文档提交里改。
27. **13.2 第 25 条与 13.5 第 5 条**，随 P4/S5 改：第 25 条加三处线性的扫描与 `referenceLimit`，"每次解析至多约 80 ms"之后写出上限处最贵的形状，"两个上限兜不住"改成"上限兜不住"；13.5 第 5 条在"才改依赖的一行。"之后补一句。新文字在 P4/S5 设计第 8 节第 1、3 条。P4/S5 的文档提交落了（第 25 条的数写成约 62–67 ms，决定 539；13.5 第 5 条那一句照代码写成两处照抄，P4/S5 设计第 8 节第 3 条）。
28. **13.1 第 4 条与第 23 条**，随 P3/S3 改：第 4 条的码的次序加 409 `transfer.too_many_jobs` 与导出、导入的次序；第 23 条加 transfer 的五种任务的 `DiscardRunning` 与启动时的收拾。P3/S3 的文档提交落了（新文字照 P3 设计第 8 节第 12、13 条，第 13 条末句的后半照 S3 审查 A 的 N3 补；随 Step 的文档提交、不随代码，决定 531）。
29. **13.4 第 9、10、13 条**，随 P4/S7 改：第 9 条加宽读与它的计划测试，第 10 条加 B 与数法，第 13 条加两条读的逐行对照。新文字在 P4/S7 设计第 8 节第 1–3 条。
30. **13.1 第 31 条**，随 P4/S6 改：`MaxJobItems` 60,000 → 45,000，依据改成 M12/P4 的实测（P4 设计 3.12、10.3、10.4；M8 在另一台机器上取的 60,000 留作历史）；第 14 节导入一行的出处加 10.4。P4/S6 的文档提交落了。

**文档与代码不符之处**（M12 里改文档，不改代码，除非另写）：

- 5.8 第 6 步写"M12 修"、00-M11 第 302 条写"v0.1 之内不修"：第 15 条与 4.6.1。
- 13.2 第 12 条、第 26 条与打磨移交的"v0.1 之后"：4.5，第 11、17 条；P2/S1 的文档提交落了。
- 总体设计 13.1 第 32 条、`06-environment.md`、`05-lessons.md`、`03-project-background.md` 写着 Go 1.27.1：P1/S2（4.3，第 18 条）。
- `ci-wait.sh` 是 06-environment 第 4 节规定的读 CI 的工具，它原取这个提交的第一个运行、不分工作流：已在 `b54cd679` 改好（4.2）。
- 移交 M4-performance 第 14、15 条的守卫时限写 12 秒，代码是 14、14、8 秒（`bootstrap/trash_cost_test.go:539`、`revert_cost_test.go:623`、`:670`；M10 第 153 条）；第 16 条写 1 秒、2.1 秒，代码是 1.3 秒、3 秒（`bootstrap/background_cost_test.go:936-937`；M11 第 249 条）；第 15–18 条的份名是旧的（I2 第 8 节第 1–3 条）。P4 照代码量，移交收尾时注明。
- 移交 M8-history 第 5 条写"两条语句"，现在是五条读（I3 第 5 节第 4 条）。
- 移交 M5-polish 第 25 条写"分词的版本随之提升"：不需要（4.6.3）。
- 移交 M5-polish 已修或部分修了的：12.7 的一半、24.10 的注释、22.18 的大半、21.12 的一半、第 3 条的公式与图、`NativeSelect` 的宽度（I3 第 5 节第 5 条）。附录 A 只列剩下的。
- `Makefile:12` 的注释"默认值与 buildinfo 中的相同"：Dockerfile 另有两份，P5/S1 的守卫之后注释照实改。P5/S1 改了（`68777b97`，P5 设计 3.2）。
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
- **网页（M4–M11）**：附录 A 的各文件（P2）；页面树的行、菜单与对话框的挂法，首页、页与标签页的列表的分批（P4/S4）。
- **网页（M4、M6）**：编辑器的 Markdown 配置加三处线性的扫描（`web/…/editor/markdown.ts` 的 `linearScans`），跨行的引用式定义另设上限 `referenceLimit`（P4/S5）。
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
  - Chromium 与 WebKit 两种：375 的几格带标签 `@narrow`，由新的 Playwright 项目 `webkit-narrow` 在 WebKit 上再跑一遍，不把整份故事交给 `threeBrowsers`（P2 设计 4.3）；33.1 的排查只在 webkit 上跑 SE1 的 375 一段。实际复现了，SE1 拆出 `@narrow` 的一格，`webkit-narrow` 跑 MC3、PG2、SE1 三格（决定 413、589）。
- **`bdi`**：vitest 断言名字在 `<bdi>` 里。
- **对比度**：e2e 取计算之后的颜色，算对比度；文字 ≥ 4.5:1，焦点环与非文字的 ≥ 3:1；亮与暗两种主题。
- **只从 ARIA 推断的八项**：不写自动测试，进读屏的人工清单（4.12）。
- **阅读视图（P2/S4）**：更新之后，留着的块里的选区、展开的 callout、焦点与正在播放的媒体都还在（同一个节点；公式与图由增强重写，其中的选区除外）；只差附件地址的重读不换任何节点；换了新的块里的焦点找回同键的元素，或它原来所在的那一块；任务勾选框找不回就不给（P2 设计 4.4）；17.1 的整点替换由 vitest 的假定时器推进触发，不真的等一小时。

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
  - 有标志的名字与没有候选的包里的名字（除了有效的 0x7075）不变；差分照此（13.4 第 11 条；决定 376）。
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
- 判在哪台机器上（本机、CI 的实数，或乘了哪个比并写"推断"），对照的预算与结论（过、超）；
- 只量的写数，或写"推算"与依据的数；
- 超出的去处（修的 Step，或负责人的改判）。

### 9.5 发布流水线怎样在不推送的情况下验证

推送、签名、证明与 Release 只有真的 tag 才走得到（I1-D7）。之前分两层核对，rc 是第三层：

1. **静态**：
   - actionlint 核对三份工作流（`make lint-actions`，只在 runner 上：CI 的 `costs-1` 一份；P5 设计 2.1 第 10 条）。
   - 读三份工作流的守卫（Go 的短测试，P5 设计 2.1 第 7 条）：演练作业没有 write，tag 的作业照表。反向对照：给它加 `packages: write`，守卫失败。
   - 闸门脚本 `deploy/release-gate.sh` 在本机对着假的输入跑。tag 名不是 semver、提交不在 `main` 上、提交的 `ci.yml` 不绿、"`ci.yml` 红而 `release.yml` 绿"、`CHANGELOG.md` 没有这一节或它是空的，各被拒或放过一次（反向对照）。`make vuln` 不在脚本里，是闸门作业的下一步（它只在 runner 上跑）：守卫核对那一步是 `make vuln VULN_AUDIT_LEVEL=high`；级别传到 `pnpm audit`，由 P1/S2 的变异 15 那样的 `go`、`pnpm` 两个桩核对一次；"`moderate` 不挡、`high` 挡"是 `pnpm audit --audit-level` 的行为，不是我们的代码（P5 设计 2.1 第 8 条）。
   - 说明脚本 `deploy/release-notes.sh` 对着假的 `CHANGELOG.md` 跑：rc 取到去掉 `-rc.N` 的那一节、前面有"预发布"；缺这一节就拒。
2. **演练**（P5/S1）：单独的只读作业，两种原生 runner 各自 `push: false`、`load: true` 构建，对本地的镜像跑 `image-smoke.sh`；核实路径上下文带上了 `.git`、VCS 信息在。分支与 `main` 的每次推送都跑。
3. **rc**：推 `v0.1.0-rc.1`，走完整条路，照 4.11 第 4、5 步核对。keyless 签名、`actions/attest`、GHCR、Release、复制出版本号的 tag 与重跑，只在这一层第一次真的跑。只改流水线的 rc.N 不再问负责人（决定 324），这一层因此便宜。

初稿的另一层（本机起 `registry:2`、cosign 用临时的 key、syft）删掉（设计审查 B 的 M10 与过度设计）。它与 GHCR 的差别正落在要验证的地方：本机的 buildx 只有 `docker` 驱动与 `linux/arm64`，合成的索引只有一种架构；`registry:2` 没有 OCI 1.1 的 referrers 接口，cosign v3 经它存签名；cosign v3 不进透明日志的签名，校验要另带 `--insecure-ignore-tlog=true`；还要装 syft、cosign。

### 9.6 变异（每个 Phase 设计列全，这里是要列进去的）

- P1/S1：帮手什么也不等（A15 第 2、4、5、6 格抓）；帮手不等这个文档的第一次续期（第 4 格，Chromium 与 Firefox 落到 `/sign-in`、WebKit 报旧文档没发续期）；帮手不看续期的锁（第 6 格）；帮手不等在途的读（第 5 格）；`reload` 不先等（第 2 格）；门禁放过直接的 `goto` 或 `reload`；门禁不看 `e2e/fixtures/`；门禁不核对逃生口的表；门禁把帮手模块里的逃生口文字也算进去（P1 设计 7.2）。
- P1/S2：Dockerfile 的 Go 版本的 tag 与 `toolchain` 不同（守卫抓）；摘要换成别的版本的（构建时的断言抓）；README 的 PostgreSQL 版本不同（守卫抓）。
- P2：9.1 列的几种，每个 Step 照它的项补全。
- P3/S1：`TitleKey` 不做切罗基文一步；三处只改一处；`PlainText` 仍跳过自动链接；`listTags` 列 `/`；日志或变更集详情的读不在快照里；流数按账户而不按凭证数、重置最新的那条、账户的硬上限不拒、先看账户再看凭证；整个名字的判断不认只由 U+200B、U+2060、U+3164 组成的名字；`index` 的 `parent:"/"` 被拒；`log` 的 `""` 被当作整本；超时的文字照旧；`Blank` 不减 Prepended_Concatenation_Mark；`Find` 在快照之外；`alivePaths` 在快照之后；顶掉的流算到它关掉为止；提交的错误不带标记；只读的快照的提交错误也带标记；MCP 不看标记（P3 设计 7.3 的 S1-1–S1-34）。
- P3/S2：去掉新规则；`topOf` 走过回收站根；`needed` 不看段内后面的放回；某一处构造计划不传 `Nodes`（三处非等价，两处等价；P3 设计 7.3 的 S2-1–S2-9）。
- P3/S3：不看 UTF-8 标志就按 GBK；读 `NonUTF8` 而不读 `Flags`；没有候选的包里合法的 UTF-8 名字也按 GBK 读；结果里有 U+FFFD 也回退；一个名字解不开时其余的照样回退；0x7075 的 CRC 不核；报告不写；`room` 不按人数；按人的拒绝答成通用的 `server_busy`；`AtStart` 不收拾排队的行；按解读之前的名字判文件夹；解读过的名字的记录排在别的问题之前；回滚也被按人的上限拒绝；先判全局、再判按人；收拾的任务排在五种任务之前（P3 设计 7.3 的 S3-1–S3-18）。
- P5/S1：照 P5 设计 7.2，其中：四份默认值与 README 写的有一处不同（守卫抓）；MCP 的版本写死成默认值（盖了版本号的 `make test-go-short` 抓，直接 `go test` 抓不到）；闸门的各条各去掉一条、闸门不分工作流、闸门放过空的一节（闸门的测试抓）；演练作业加了 write、tag 的作业的 `if` 放宽、闸门的漏洞一步 `continue-on-error`（守卫抓）（P5 设计 2.1 第 6、7 条）。
- P5/S2、S3：照 P5 设计 7.2，其中：版本号的 tag 改指、Release 已公开时照样上传（发布脚本的测试抓）；`/licenses` 缺 Go 本身的许可证（image-smoke 抓）；`compose.yaml` 引用错的 secret（`make deploy-check` 抓）；它的 PostgreSQL 镜像不同（守卫抓）。

### 9.7 门禁与 CI

- 每个 Step 照 02-workflow 第 3.1 节：`make lint gen-check knip build`、`make test`、`make test-go-short`、`make e2e`，都经 `heavy.sh`；每个提交单独为绿。2026-11-19 起照负责人第 12 问（第 11 节）：推送之前与每个提交只跑与改动相关的，全量交给分支 CI；M12 收官时在 `main` 上整体跑一次全量。
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
| **e2e 的偶发挡发布**：Playwright 不重试，`main` 的 CI 红了就不能打 tag | P1/S1 最先做；门禁数逃生口；`ERR_NETWORK_CHANGED` 加一行日志看它在 CI 上触发了没有；其余的偶发照 M11 第 277 条的做法只重跑 e2e，并在记录里写明；L1 已改测法（决定 541）；装浏览器卡在 apt 的镜像不再取消 E2E：这一步自己 30 分钟，系统包有缓存（决定 599、600） |
| **演练作业的 CI 用时**：每次推送（分支与 `main`）多两个作业，推断 amd64 约 2–3 分钟、arm64 约 2–5 分钟（P5 设计 2.1 第 1 条；原来推断各 10–15 分钟） | 与 `ci.yml` 并行，不进组合根；`ci-wait.sh` 按工作流取，不被它挡住也不被它冒充 |
| **推 tag 之后才走得到的路径**：keyless 签名、`actions/attest`、GHCR 的推送与权限、Release、复制版本号的 tag；`build-push-action` 的上下文 | 9.5 的三层；rc 先走一遍；一时的失败在同一次运行上重跑，不重推 tag（4.10）；允许的第三方 action 与组织的 Actions 设置由负责人在推 rc 之前核对（第 11 节） |
| **GHCR 设为公开不能撤回** | 只在 rc 核对之后做；之前核对镜像里没有机密：最后一层只有二进制与 `/licenses`（`deploy/Dockerfile:41-47`），构建参数只有版本与 label，`.git` 只在构建的阶段；Release 的说明与附件在 rc 上先看一遍；版本号的 tag 在签名与证明之后才打 |
| **先发布、后验收**：v0.1.0 发出去之后，人工验收可能在输入法、真实的 MCP 客户端或读屏软件上找到问题 | 发布说明写明人工验收在发布之后、还没逐项核对过；v0.1.1 的移交与目录先定好（第 7 节"写给后面的 M"）；部署文档要求生产写确切的版本号，照升级的步骤换版本 |
| **打磨的面宽**：约 113 项，焦点的修复常波及别处（M5–M7 的修复核对一再找到回归，05-lessons 第 8 节） | 按组件分六个 Step；焦点的修复在三种引擎里探查；阅读视图先写 spec；修复的核对一轮一轮做到干净 |
| **第 302 条的修法改的是所有撤销都走的计划** | 先写 spec；改法小（`topOf` 在本身是回收站根的节点处停，审查 A 的原型约 10 行）；认出的三种形状写进 spec；两个现成的性质测试扩开；后台撤销的 `needed` 随之改（批的核对挡不住，只有 `needed` 挡，P3 设计 3.10.6）；M8 原有的撤销测试照过；收尾审查 A 专看撤销的其余路径 |
| **标题键与标题 id 改了已有的键** | 发布之前做；发布说明写明试用库要 reindex、这类锚点变了；不升分词与提取规则的版本，免得白白重建 |
| **同一凭证上 5 个以上守契约的客户端轮流互顶** | 按凭证计数，不同设备与令牌不互顶；同一凭证的轮转受它自己的限流约束；被重置的一行是 `debug` 或限频；hub 的单元测试有"17 条活流在有限步之内停下" |
| **GBK 回退把别的编码误读成中文** | 先读 0x7075；回退的条件写明"结果里没有 U+FFFD"，一个名字解不开整包不回退；Shift-JIS、Big5、EUC-KR 仍会被误读，写进已知限制；报告列出解读的名字 |
| **T2、T4 改了任务的入口与启动** | 只改 `room` 与 `AtStart`，另给五种任务设 `DiscardRunning`、改已有测试的种子（带一行 River 持有的任务，P3 设计 3.14.3）；P3/S3 先写设计；T4 假定单实例，与部署文档一致；整个程序的测试 |
| **B5 的规则在本机上可能过不了**：本机比 M5 Max 慢，界线处的撤销单次记过约 6.2 秒（移交 M4-performance 第 15 条，哪台机器没写明） | B5 只用 M8 的规则判，不再乘倍数；最低配置的硬期限用 7763 上的实数加 `parse_max_wait` 判；规则一层没过不自动降常量，照移交第 14 条的两种做法（降常量，或写进部署要求）列给主持者与负责人定（4.7.5） |
| **实测的时长与机器**：搜索一批约 20 GB 的磁盘；交换已满，噪声大 | 范围先缩到有预算的与部署文档要的；独占时段跑、经 heavy 的锁；之前修剪 Go 的构建缓存；两轮取较差的；本机为参照，部署文档写明 |
| **页面树 1 万页没过** | 4.7.2 先量它，结论先于 P2/S6；改法要设计，是一个大的 Step，M12 随之变长；负责人可以改判为已知限制 |
| **并行的 Step 争机器与翻译文件** | 同时实施的不多于两个；`en.ts`、`zh-CN.ts` 一次合并一个、后合并的先 `git merge main` |
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

### 2026-11-17：L1 的改测法与 P4/S3、P5 之前的四问

主持者在 L1 的旁支审查之后问了一问（审查 A 的 L1A-Q1、C 的 L1C-Q1：M11 第 277 条把改测法留给负责人），又把"需要负责人确认"里 P4/S3、P5 开工之前要的几项一起问了。原问与所选，逐字：

7. 问："e2e 的 L1（Ctrl 点链接开新标签页）约 4% 偶发超时：标签页其实开了，是 Chromium 偶尔漏发一个事件，Playwright 因此等不到。M11 第 277 条把"改测法、削弱测试"留给你定。旁支已备好一个改法：不再等浏览器开出的标签页，改为在点击那一刻核对"应用没拦下这次点击、地址是真的"，再由测试自己开标签页打开这个地址。改后 200 次 0 红；代价是 e2e 不再证"浏览器真的开了标签页"。怎么处理？"
   选："采用改法 (Recommended)"，即"合并旁支 m12-e2e-l1-new-tab，记录里写明取代 M11 第 277 条。发布前 main 的 CI 不再被它偶发挡住。"
8. 问："有没有 v0.1.0 之前就在跑、要升级到 v0.1.0 的 Nerve Wiki 实例（你自己的开发库之外）？没有的话，P4/S3 不量批次 H、部署文档不写开发版升级的附录，根 README 的六节开发版升级说明删掉（留在 git 历史里）。"
   选："没有 (Recommended)"，即"只有我自己的开发库，不写开发版附录、不量批次 H。"
9. 问："输入法清单（M4–M8 的 21 步）是否也在发布之后验收，和读屏、清单 A–K 一样？"
   选："发布后验收 (Recommended)"，即"同 2026-10-13 的原话：发布说明写明未验收，发现的问题出 v0.1.1。"
10. 问："SECURITY.md 要指向 GitHub 的私密漏洞报告。你会在仓库设置里开启它吗？顺带 Dependabot alerts（只管 Go 与 actions）要不要开？（这是你在 GitHub 上的操作，我的令牌读不到这些设置。）"
    选："两个都开 (Recommended)"，即"私密漏洞报告在 P5/S3 写 SECURITY.md 之前开；Dependabot alerts 随时开。"
11. 问："发布流水线从 P5/S1 起每次推送都会用 docker/* 这些第三方 action 做只读演练，P5/S2 起还用 anchore/sbom-action；推 tag 时还要 sigstore/cosign-installer 与 packages/id-token/attestations/artifact-metadata 的写权限。组织或仓库的"允许的 actions"有没有限制？"
    选："先跑分支 CI 看 (Recommended)"，即"不确定就先做 P5/S1，分支 CI 红在权限上时我再告诉你去设。"

照此：L1 的旁支照常审查、合并，它的决定与记录写明取代第 277 条（第 7 问）；P4/S3 不量批次 H，P5/S3 不写开发版的附录，根 README 的六节开发版升级说明照 4.8 那一条删掉、删之前把对新装也成立的数挪走（第 8 问）；输入法清单并进发布之后的验收（第 9 问）；`SECURITY.md` 照 P5 设计指向私密的漏洞报告（第 10 问）；P5/S1 的实施者在分支 CI 红在 actions 的权限上时停下，主持者转告负责人（第 11 问）。主持者的令牌读不到仓库的 Actions 设置（`gh api …/actions/permissions` 答 403），所以第 11 问问的是先怎样做，不是设置本身；"需要负责人确认"第 2 项仍要在推 rc 之前照实核对。

### 2026-11-19：门禁与核对的两条提速

主持者为缩短验证的排队提了两条提速。所问与负责人的原话：

12. 问（主持者的两条提速）：推送之前只跑与改动相关的门禁，全量交给分支 CI；修复之后只剩 Nit 的，不再开一轮核对。
    答，原话："两条都采用，从下一轮起照此执行，全部收官之后再整体跑全量 Gate"。

照此（负责人 2026-11-19 第 12 问）：

1. **推送之前不跑完整的 `make check`、`make test`**：本机只跑与改动相关的，经 `heavy.sh`：改了网页的 `make lint-web`、`make knip`、`make test-web`，改了行为、故事或夹具的加 `make e2e`（或只跑受影响的故事，三种浏览器照 `threeBrowsers`）与两个 `check.mjs`；改了 Go 的 `make lint-go`、改动的包与依赖它们的包的 `go test`、`make test-go-short`；改了工作流、部署脚本或 Dockerfile 的 `make lint`、`make test-go-short`，改了 Dockerfile 或 `image-smoke.sh` 的加普通克隆里的 `make image-smoke`；只改文档的 `check_links.py`、`git diff --check`。全量交给分支 CI（19 项并行）。每个提交单独为绿照旧，用同样的相关子集。
2. **只剩 Nit 不再开一轮核对**：修复之后只剩 Nit 的，Nit 并进文档提交，由主持者核对（02-workflow 3.4）。
3. **M12 全部收官时**，主持者在 `main` 上经 `heavy.sh` 整体跑一次全量门禁（`make check`、`make e2e`、两个 `check.mjs`、`make image-smoke`），写进收尾记录。

从那以后派的修复与核对照此写，正在跑的不打断：第一轮照它执行的是 P2/S5 的核对第 2 轮（只跑与对象相关的），推送之前的门禁第一次照它跑的是 P2/S5 的修复第 3 轮。第 1 点取代 9.7 第一条与各 Phase 设计门禁一节里"推送之前 `make check`"的写法。三点的同一段文字写进 [02-workflow](../claude_to_codex/02-workflow.md) 3.1 的 M12 注。

### 2026-11-21：P4/S3 的四件（第 13 问）

主持者在 P4/S3 的批次 F 之后，把超出与推断过期限的四件一次问了负责人（`AskUserQuestion`；每题的第一个选项带"(Recommended)"，负责人四题都选了它）。原问与所选，逐字（选项的说明照工具的原样；没选的只列标签）：

13. 四个问题：
    1. 问："P4 实测 B4（树写，15 秒请求期限，过了就答错误、什么也不写）：导入上限的 vault 约 6 万个节点，整个删除、移动、改名，在最低配置 EPYC 7763 上推断要约 17、30、29 秒，都过期限。本机快约 2 倍，移动也撞过一次期限。降导入上限只管得住删除（4.5 万时约 13 秒），移动和改名还要约 22 秒。怎么处理？"
       选："开修复 Step (Recommended)"，即"按 00-M12 4.7.5 在 v0.1 修。先写设计，查清移动、改名的时间花在哪。目标：6 万个节点的删除、移动、改名在 7763 上不超过 15 秒。设计若要大改（例如改成后台任务），先回来问你。发布要推后，估计一天以上。"（另一个选项："改判为已知限制"。）
    2. 问："B5 规则（每个单元不超过 5 秒，在本机判）：后台撤销一次深的导入时，最上层那一批 6 万个节点在本机要 6.1–7.4 秒，没过。这个常量 MaxJobItems 同时就是导入的节点上限。怎么处理？"
       选："降到 45,000 (Recommended)"，即"两轮都在 5 秒内（至多 4.45 秒），是按 5,000 一档在 5 秒内的最大一档。导入上限随之变成 4.5 万个节点，更大的 vault 要分开导；条目多于它的变更集不能整组后台撤销。B4 的删除在 7763 上降到约 13 秒。"（另三个选项："降到 40,000""改判为已知限制""写进部署要求"。）
    3. 问："B5 硬期限（请求一路，15 秒）：在最大规模的界线处恢复、撤销一次删除，CI 上 EPYC 7763 的实测最大值加 2 秒解析等待，是 13.79 和 14.54 秒，过了。按"慢的 7763 再慢 25%"推断则是 16.47 和 16.88 秒，超过 15 秒（另一种推断约 12.2 秒，过）。部署文档怎么写？"
       选："最低配置仍是 7763，写明余量 (Recommended)"，即"不改代码。写明在满负载界线处，一般的 7763 只余约 0.5 秒，慢的实例可能超时（答错误、什么也不写）。这与 P5 部署文档原计划的写法一致。"（另两个选项："提高最低配置""降常量"。）
    4. 问："B2 解析预算的内存（界线 2,400 MiB，即 8 MiB × 300；README 写的是"约 2.4 GB"）：16 个并发读写 5 MiB 的病态正文，堆增量 2,519 MiB；复核两轮 2,385、2,474，仍超约 1–8%。多出来的是解析之外的答复和正文。8 个并发那一格的复核过了。怎么处理？"
       选："改判，文档改成约 2.6 GB (Recommended)"，即"不改代码。README 与部署文档写最坏约 2.6 GB 堆（进程峰值约 2.8 GiB），部署时按约 3 GiB 留内存。"（另一个选项："开修复 Step"。）

照此（负责人 2026-11-21 第 13 问）：

1. **B4 开条件的 P4/S7**（第 1 条）：照 4.7.5 先写设计，查清移动、改名的时间花在哪；目标是 6 万个节点的删除、移动、改名在 7763 上不超过 15 秒；设计若要大改先问负责人。发布随之推后。
2. **`MaxJobItems` 降到 45,000，开条件的 P4/S6**（第 2 条）：导入的节点上限随之；设计是 [P4 设计](04-P4-performance.md) 3.12；S6 先于 S7 合并。
3. **B5 硬期限不改代码**（第 3 条）：最低配置仍是 EPYC 7763，部署文档的硬件一节写明余量（P5/S3，4.8 原计划的写法）。
4. **B2 改判**（第 4 条）：不改代码；README（`README.md:141`、`:630`，`f1317bc3` 的行号）的"约 2.4 GB"由 P5/S3 改为最坏约 2.6 GiB 堆（2,662 MiB，写明单位；最坏实测 2,592 MiB），部署文档按约 3 GiB 留内存（只覆盖 B2，同时到的见 P4 设计 10.3 的"给部署文档"）。

两个条件 Step 照 4.7.5、决定 642；后两条落在 P5/S3（决定 643）。

注（P4/S3 的修复第 1、2 轮，审查 A-I1、A-M5、B-I1、B-M1；原文不改）：

1. 第 2 题 40,000 那一项写的"约 14.5 秒，在 15 秒内"是 B4 删除的 × 1.25 一行照记录的式子的数；同口径约 15.2–15.7 秒，过 15 秒（[P4 设计](04-P4-performance.md) 10.3 的 B4）。这不改变负责人选的 45,000（45,000 时主行同口径约 13.6–13.9 秒）；第 1 题的"约 17 秒"同口径约 17.5–18.1 秒，仍超。主持者另当面告诉负责人。
2. 第 3 题"另一种推断约 12.2 秒，过"那一行，在本机低于判据行（基准在 `VACUUM` 之后量，比守卫量的状态快），不提供余量的信息。同一处另有一件：守卫的状态下本机的单元约 5.4–5.9 秒，过 5 秒，主持者记为已知限制、与这一题同一处（决定 644），负责人可以改判。
3. 第 4 题写的"堆增量 2,519 MiB"漏了第一次的第一轮的 k = 16，2,592 MiB；8 个并发那一格照修复第 2 轮的窗口是 2,525 MiB，也过 2,400（复核仍过）。"进程峰值约 2.8 GiB"出自把字节 ÷ 10⁶ 当成 MiB 的 `VmHWM`，实为至多 2,718 MiB（约 2.65 GiB）。文档的约数取 GiB、写明单位：约 2.6 GiB 即 2,662 MiB，比最坏实测余约 3%。这些不改变负责人的选择；3 GiB 只覆盖 B2，B2、B3 与树的重读同时到的推算写进部署文档（决定 651）。主持者另告诉负责人。4.7.1 的 B2、B5 两行"超出时"一格与 [P5 设计](05-P5-deploy-release.md) 3.18 加了这一问的注。

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
- **318 "目标硬件"**以本机为参照、EPYC 7763 为慢的主机的代表（I2-D2）；4.7.1 逐行写判在哪台机器上：有硬期限的，有 CI 上的实数时由实数定，没有时才乘本机对 7763 的实测之比、写"推断"（这一半由 P4 的设计审查之后的决定 454 改，原来是乘记下的最大倍数 2.33 或 4.6）；B5 只用 M8 写下的规则在本机判，最低配置的硬期限用 7763 上的实数加 `parse_max_wait` 判，规则没过不自动降常量，两种做法列给主持者与负责人定；部署文档的最低配置是 7763 的那一档（4.7.1、4.7.3、4.7.5；设计审查 A 改，修订核对第 1 轮 A 的 I4 再改）。
- **319 测量的设施进仓库、不进 CI**；内存从外部采样，不加 pprof 或 metrics 端点；只写有预算的几项要的驱动（I2-D4、D5、D14，4.7.4；设计审查 A 改）。
- **320 部署文档**：`deploy/README.md` 作运维手册，加 `deploy/compose.yaml` 与它的检查；`compose.yaml` 的 `migrate` 在 profile 里、备份之后显式跑；要求写 PostgreSQL 18.7 或更新与硬件下限；根 README 缩成入口，删开发版的说明之前先挪走对新装也成立的数；开发版的升级说明看负责人的回答；单实例写明原因、时钟一条不适用；Caddy 的 502 只写代价、推荐 nginx；小项都写进去（I1-D13、D15–D18，4.8；设计审查 B、C 改）。
- **321 版本号**：tag 经 ldflags 注入，四份默认值有守卫，MCP 的版本由短测试守住，发布之后 `main` 是 `0.1.1-dev`；不打 `server/v0.1.0`，不宣传 `go install`（I1-D8，4.9；设计审查 B 补）。
- **322 发布流水线**（I1-D1–D7、D14，4.10；设计审查 B 改）：`ghcr.io/open-nerve/nervewiki`，只打确切的版本号，不打 `0.1` 与 `latest`；tag 触发；闸门只认 `ci.yml`、加 `make vuln` 与 `CHANGELOG.md` 的一节；两种原生 runner；演练是只读的单独作业，分支与 `main` 的每次推送都跑、不加路径条件；先打 `sha-<提交>`，证明与签名之后才复制出版本号的 tag，已有的不改指（P5 设计 2.1 第 4 条）；一时的失败在同一次运行上重跑；发布之后的核对在两种架构上、在公开 Release 之前；`actions/attest` 的来源证明与 SBOM 加 cosign keyless；前端另出一份 SBOM，SBOM 的生成在只读的作业里（P5 设计 2.1 第 2 条）；镜像与 Release 带第三方许可证；Release 不附二进制、先草稿后公开（P5 设计 2.1 第 3 条改，原来是"没有就建有就改、附件 `upload --clobber`"），`v0.1.0` 不标 pre-release；先推 rc；不用 `docker/metadata-action`。`ci-wait.sh` 按工作流取已在 `b54cd679`。
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
- **336 T2、T4 在 v0.1 修**（负责人 2026-11-14 第 5 问）：T2 是按人的常量上限，答一个新的码（`transfer.too_many_jobs`，409，P3/S3 定；初稿推荐 429），有自己的两种语言的文字，网页照它显示，全局的 503 `server_busy` 照旧；T4 是启动时把上一个进程在 River 取到之后、开始之前停掉的任务记成失败，River 那一边照 `Job.DiscardRunning` 的先例、注意它的次序。都在 P3/S3（4.6.6；修订核对第 1 轮 A 的 M9、N12，C 的 M10 改，第一次修订是"同样答 503"）。

### 2026-11-15：P1 的设计审查之后的取舍 337–352（主持者的决定，负责人可以改判）

337–350 是 P1 初稿的十四条（P1-1 到 P1-14，P1-k 是 336 + k），设计审查之后照次序编号，344、347、349 照审查改写；351、352 是审查之后新定的（P1-15、P1-16）。每条在 [P1 设计](01-P1-foundation.md) 2.3，审查里的出处在 [P1 审查记录](reviews/P1-design-review.md)"主持者的决定"一节。

- **337 e2e 换文档的帮手是函数 `goto(page, url, options)`、`reload(page, options)`，在 `e2e/fixtures/navigation.ts`，不是 `page` 的方法**（P1-1；P1 设计 3.2）：`anotherPage`、`anotherTab` 与故事自己开的标签页也要它，包一层 `Page` 会让 Playwright 的类型与 `page.context()` 的页面对不上。
- **338 在途的请求只算 `/api/v0/` 之下的 fetch 与 XHR，事件流除外**（P1-2；P1 设计 3.2）：图片、媒体与附件的内容打断不了续期，媒体的请求还可能一直开着。
- **339 帮手等三样：在途的接口请求答完、续期的锁空着（没有 Web Locks 时看租约）、这个文档有了给它令牌的答复（文档的 `performance` 里有续期、登录或注册的一条）**（P1-3；设计审查 A-I1、C-I1 之后每一样各有 A15 的一格只在它去掉时失败；P1 设计 3.2、3.6。P1/S1 实施改，原来是"这个文档的第一次续期已答（文档的 `performance` 里有 `/api/v0/auth/refresh` 一条）、续期的锁空着"：在登录页登录的文档照原来的写法等满 10 秒，A3、S4 失败；登录、注册在锁里写记录，先看记录会在写之前放行，A15 第 8 格；P1 设计 3.15 第 2、9 条）。
- **340 帮手等 10 秒，然后失败并写出在等什么；在页面里读的那一次与剩下的时间赛跑**（P1-4；设计审查 C-N2；P1 设计 3.2）：比续期自己的 8 秒长、比测试的 30 秒短；Chromium 在下一个文档还没提交时 `page.evaluate` 不返回。
- **341 没被监视的页面，帮手直接失败**（P1-5；P1 设计 3.2）：在第一次调用时才开始监视，那时已在途的请求看不到。
- **342 落到 `/sign-in` 只写进失败时的页面状态，不让通过的测试失败**（P1-6；P1 设计 3.3）：有意走到登录页的故事很多；收尾照 CI 注解里 `sent back to sign in:` 一行数：数这一行带 ` after a refresh answered 401` 的失败，再除去有意造重用的故事（A5）（决定 334；P1/S1 实施改：这一行只在页面落到过 `/sign-in` 时写，故事自己去登录页的也有，原来写"照这一行数"；P1 设计 3.15 第 10 条）。
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

### 2026-11-15：S2 的决定 353–356（主持者的决定，负责人可以改判）

353、354 在 S2 的实施中定，355 在审查之后定，356 在合并之前定。每条在 [P1 设计](01-P1-foundation.md) 3.16，理由与审查里的出处在 [S2 的审查记录](reviews/P1-S2-toolchain-review.md)"主持者的决定"一节。

- **353 katex 的 GHSA-238p-pmpm-9mq7（low，修在 0.18.2）写进 `pnpm-workspace.yaml` 的 `audit.ignore`**（`c654db5f`；P1 设计 3.13 第 5 步、3.16 第 9 条）：`math.ts:44` 的 `trust: false` 是选项对象自己的属性；`:123` 把 mermaid 给的选项放在前、我们的在后（`{ ...given, ...options }`）；`:318` 只有我们的选项与 `displayMode`（`{ ...options, displayMode: displayed }`）。继承来的 `trust` 盖不过它，公告剩下的风险要先有别的原型污染。0.16 线没有修，升 0.18 是 4.3 不升的功能性大版本、补丁要重做；发布的闸门用 `high`，本来不挡它。katex 0.18/0.19 与补丁重做，收尾时写进 v0.2 的 `M12-release.md`。
- **354 审查之前就在 S2 里挪份：第三份的 TestTheSearchIsWhatEachReadAllows、第五份的 TestTheWikiToolsAnswerEveryCode 挪到第八份（也进第七份的模式），不分第九份**（`28980104`；P1 设计 3.16 第 3 条，第 7 节"分份"）：照 M11 决定 272、276 的先例；分支上慢的是编译，不是测试，照 7763 上包的时间折算只有这两份过约 400。八份、19 项不变。
- **355 `ci.yml` 的 server 矩阵只让竞态构建的九份（`race`、`bootstrap-1`…`8`）取存 Go 的缓存，另外七份不取不存；`vuln.yml` 照旧 `cache: false`**（`0844464d`；S2 审查 B 的 I1；P1 设计 3.16 第 2 条）：setup-go 的键归先存下的那一份，竞态与不带竞态的编译结果不共用；不改的话合并之后 `main` 的新键由最先跑完、不带竞态的 `short` 存下，组合根每份从头编译。七份只少约 2 秒的模块下载。
- **356 S2 的文档提交的分支 CI 红在 S1 的 e2e 帮手的竞态上：只重跑 e2e、照常合并 S2；帮手开旁支 `m12-p1-navigation-settle` 修，在 `docs: M12/P1 done` 之前合并，照常审查**（run 43686031128；照 M11 决定 249 的先例）：服务端每个那样的请求都很快答了；trace 里那一个只到响应头，随后帮手换了文档。帮手的 `settle` 看过在途的请求为空之后才去读页面里的锁与令牌，读完不再看一次；换了文档之后，只到响应头的请求在 Chromium 里既不报完成也不报失败，一直算在途，下一次 `reload` 等它满 10 秒。S2 没有改 e2e 与网页，红的原因在 S1。S2 合并之后第一次 `main` 的 e2e 又红在 S1 的 A15 第 3 格（Firefox），一并交给这个旁支。旁支照实（P1 设计 3.17）：两样一起出了错，各自改：`settle` 读完页面为空时隔一个 `pollMs` 再看在途的请求，同一轮两样都空才放行（`d8cbb216`、`b48e8c26`，A15 第 10 格）；`followRequests` 在主框架提交它最后请求的那个文档时清掉留下的请求，同文档的移动不清（`860bf289`、`5db08ef1`，第 11–13 格）。第 3 格：闸门每 25 ms 各问一次，离开之前等"Loading…"（`cb13cc9a`），Firefox 8 个 worker 80/80。

### 2026-11-15：旁支 `m12-p1-prepared-alloc` 的取舍 357（主持者的决定，负责人可以改判）

- **357 M10 的守卫 `TestPreparedSourceDoesNotCopyTextForComparison` 不提高 2 MiB 的界，留在竞态一项。每次调用从 `TotalAlloc` 里按 pgx 的分配形状减去后台读取器的缓冲（`min(8 KiB 块数, 24 字节对象数)` 块），取至多三次调用里最小的一次**
  - 提交 `2b65d4b9`、`c82cc3b6`、`19160e7a`，合并见第 12 节；审查与修复见[旁支的审查记录](reviews/P1-prepared-alloc-review.md)。
  - 不再优化：照负责人 2026-11-06 的话（调用例的量级或时限），同 M11 决定 249。

  **起因**：旁支 `m12-p1-navigation-settle` 的 `cb13cc9a`，run 43694312336 三次 attempt 都红在竞态一项：2,462,776、4,631,752、2,561,368 字节。此前自 2026-11-06 以来竞态一项 155 次绿。

  **原因**：
  - 本机两个版本（`main`；`0da2fb5d` 的 Go 1.27.1、PG 18.6）竞态与否都在 269,280–269,576 之内。
  - pgconn 在每次 flush 前后挂一个 15 ms 的计时器（`enterPotentialWriteReadDeadlock`）。发查询的 goroutine 在那一段被挂起，pgx 就起后台读取器（`bgreader` 的 `bgRead`），把结果读进新的 8 KiB 缓冲，多则整个 6.4 MB。一次复制是 6.33–6.36 MB，一样大，调界分不开。
  - 注入复现：写完之后停 17–20 ms 复现出 1.2–6.8 MB（20 ms 也会读完整个结果），盖住 CI 的三个数；停 16 ms 也起，读得少（0.28–1.4 MB）；停 5 ms 不变。剖面里多出来的全在 `bgRead`。
  - 本机少见：16 个 `yes` 占满 8 核时，新测试 240 次里 3 次第一次调用里出现 8 KiB 块，形状同注入的停顿（没有剖面，推断是后台读取器），其中两次原测试会红；原测试 200 次没碰上（审查 B）。钉核、`GOMAXPROCS=1`、`nice` 都没有出现。
  - CI 上的 N 从未打印过，"CI 上是后台读取器"是按大小与机制的推断。

  **第一层：配对减去。**
  - pgx 的 `iobufpool` 每新建一块 8 KiB 缓冲，都带一个 24 字节的 `*[]byte`。后台读取器起来时，两者一起涨（24 字节的总比块数多 6–7 个，是平常的那几个），减去两者中少的那个块数。
  - 只造 8 KiB 块的复制（例如每行一个 `bytes.Buffer` 先 `Grow(8 << 10)`）只被减去平常的 6–7 块。`2b65d4b9` 整个减去 8192 一类，这样的复制藏得住 67 MB（审查 B 的 ALLB-I1），`c82cc3b6` 改为配对。注入的停顿下，两种算法每次调用逐字节相同。
  - 减去之后，剩下后台读取器每次读约 90–160 字节的簿记：读满 8 KiB 时整个结果约 7 万字节，按 1448 字节读约 0.5 MB。
  - 平均每次读不到约 500 字节、且整段都由后台读时会过界，这时只有第二层兜，三次都碰上就误报：每次读 256 字节、三次都停 300 ms，红在 3.7–4.4 MB。所以"与碰上的频率无关"只在读得不碎时成立。

  **第二层：取至多三次里最小。**
  - 先例是 M8 收尾审查 A-1（总体设计 13.4 第 8 条"取几次里最快的一次"）。
  - 这一层是防备性的：没有注入时，审查与门禁几百次没有走到第二次调用。

  **盲区**：
  - 每块 8 KiB 之外再分配一个 24 字节对象的复制；
  - 不是三次调用里每次都有的复制。

  其余的复制每次调用都过界：
  - 两个复制的变异与审查 B 的 `row-clone`、`big-object`、`two-columns`，之前、之后都次次红，它们不在 8 KiB 一类里，减法一字节不少；
  - B 的四个 8 KiB 形状（`row-buffer-grow-8k`、`row-scratch-8k`、`row-scratch-7000`、`chunks-8k`）在 `2b65d4b9` 上绿，在 `c82cc3b6` 上红，每次调用只被减去平常的 6–7 块。

  **留在竞态一项**，不照 13.4 第 8 条改成 `!race` 加 costs 一行：
  - 那一条的理由是"竞态检测让分配多数倍"，这个守卫量到的数竞态 269,280–269,576、不带竞态 269,280–269,360；
  - 借用连接缓冲的检查在竞态构建下跑也有价值；
  - 代价是通过的运行不打印数，看不到两层有没有起作用。接受；失败信息逐次写出 `TotalAlloc`、8 KiB 块数、24 字节对象数与减去之后的数。

  **其他**：
  - 测试里不核 pgx 的版本，06-environment 记一句：升 pgx 时核对这个守卫的两个常量。
  - 合并先后：这个旁支先合进 `main`，`m12-p1-navigation-settle` 再把 `main` 合进去跑 CI。

### 2026-11-15：帮手的旁支的决定 358（主持者的决定，负责人可以改判）

在旁支 `m12-p1-navigation-settle` 的修复核对之前定；在 [P1 设计](01-P1-foundation.md) 3.17，审查里的出处在[旁支的审查记录](reviews/P1-navigation-settle-review.md)"主持者的决定"一节。

- **358 下一个文档请求之后、提交之前，旧文档以同一地址 `replaceState` 时，帮手把这次同文档的移动当成提交，提前清空、忘掉地址，之后旧文档再发的读留下（Chromium、Firefox）：不改帮手，作为已知的窄口写进 P1 设计 3.17**（旁支审查 A 的怀疑，修复一轮的探针 G 核实）：应用经 React Router 的 replace 调 `replaceState`（17 处），但都是渲染时换到别的地址、或由用户的选择触发，没有在请求下一个文档之后、以同一地址 replace 的（修复核对 A-N1 = C-N4）；补法"文档请求结束时才忘地址"要先证三种浏览器里文档请求的结束一定晚于提交，否则漏清；下载在 Firefox 报 `requestfinished`、不提交，补不全。

### 2026-11-15：`main` 上来源的恢复与撤销的守卫 359（主持者的决定，负责人可以改判）

- **359 分配守卫旁支合并之后第一次 `main` 的 guards-sources 在 7763 上红：只重跑失败的一项；守卫开旁支 `m12-p1-source-revert-deadline`（M10 的故事，在 M12 里发现）修，在 `docs: M12/P1 done` 之前合并，照常审查**（run 43721644553；照 356 的先例）。
  - **做法**：照负责人 2026-11-06 的话（调用例的量级或时限，不再优化）与 M11 决定 249，只调守卫的量级或时限，不改产品。先定因：7763 上 S2（Go 1.27.4、PostgreSQL 18.7）前后有没有台阶。若是产品变慢、超出设计预算（4.7.1 的 B5），停下另定。
  - **背景**：
    - `TestSourceRevertAtTenThousandTakesItsTime`（`server/internal/bootstrap/source_cost_test.go:177`）是 M10 的守卫，界 7.6 秒（`:26`），但请求 4 秒、提交 2 秒的期限断言不提高（`:21`），实际起作用的是 4 秒；生产的请求期限 15 秒（行号是 `ad0bf40c` 的）。
    - M10 出过同样的失败，那次改了撤销规划的读（[M10 的审查记录](../M10-llm-wiki/reviews/P3-revert-tree-review.md)）。
    - 7763 上 revert 的 sample2：run 43693474739 3.31–3.50 秒，run 43711581539 3.71–3.86 秒；这次三个状态的 sample0、1 是 3.21–3.65 秒，sample2 都撞上 4 秒。同一棵树在 EPYC 9V45 上（分支的 run 43719110918）sample2 2.31–2.50 秒。
  - **旁支的做法**（来源守卫的旁支，提交 `83437b79`、`b0c80b74`、`80de075a`，合并见第 12 节；[审查记录](reviews/P1-source-revert-deadline-review.md)）：
    - 恢复、撤销两个守卫改在生产的请求期限下跑：`server.request_timeout` 15 秒，读 30 秒、写 60 秒。由 7.6 秒的界来判，界不变。
    - 提交期限 2 秒不变；新建、移动、遗忘的界都在 4 秒之下，仍用 4 秒。
    - 改的是 M10 写进测试、Makefile 与总体设计 13.4 第 8 条的"请求 4 s、提交 2 s 不变"：对这两个用例，请求期限不再是 4 秒。13.4 第 8 条随之改。
  - **定因：不是变慢，而是一直贴着 4 秒**：
    - **失败本身**：失败的是 run 43721644553 的 attempt 1（job 132834719102）。撤销三个状态的 sample2 都在 4.0 秒的期限被取消，所以真实用时 ≥4.0 秒；照 sample2/sample1 之比推算，至多约 4.1–4.5 秒。
      - 那次运行重跑过。读 attempt 1 的日志要写 `gh run view 43721644553 --attempt 1 --log-failed`，或 `gh api --allow-escape-sequences …/actions/jobs/132834719102/logs`；`gh run view --job 132834719102` 给的是 attempt 2 的（审查 A 的 Q1）。
    - **同一棵树的重跑**（attempt 2，job 132841888377）也落在 7763 上：撤销的 sample2 最慢 3.54 秒，18 个都是 200，各项都在 S2 之前的范围之内。这是失败出在那一台 runner 上最直接的证据。
    - **7763 上的历史**：M11 起共 38 次。撤销每次最慢的 sample2，S2 之前 33 次是 3.30–3.78 秒（中位 3.47）；S2 之后是 3.50、3.59、3.86、≥4.0（500）与重跑的 3.54。它一直只比 4 秒少 3–18%。
    - **M10 也撞过 4 秒**：M10/P2/S2 与 P3 在同一组守卫上几次撞到 4 秒，见下面"反向的先例"。
    - **失败的那一台**在 7763 的 38 次里，每一项都是最慢的：
      - 计时的新建、移动、恢复、撤销，比中位慢 9–25%；
      - 不计时的种子与准备，慢约 18–20%；
      - 编译慢 16%。
    - **看不出与代码有关的台阶**：
      - 9V74：S2 之后 6 次与之前 38 次相比，各项中位相差 −2% 到 +1%；第 7 次也落在中间。
      - 本机：`0da2fb5d`（Go 1.27.1、PG 18.6）对 `ad0bf40c`（Go 1.27.4、PG 18.7），撤销 27 个样本的平均是 1.533 对 1.530 秒；审查 A 另跑的两轮（撤销各 18 个样本）是 1.507 对 1.515 秒。
      - 7763：S2 之后没有失败的 4 次，各项比之前的中位高约 2–4%。这 4 次在同一天，多数仍在之前的范围之内；次数太少，分不出是 runner 还是 Go、PostgreSQL。不改做法；合并之后 `main` 碰上 7763 的几次，顺手看撤销的 sample0 是否回到 2.65 秒左右。
  - **这个用例没有越 B5 的迹象**（4.7.1）：
    - **规则一层**：本机单跑，撤销的 sample2 最慢 1.96 秒。这是来源的形状（1 万个 64 字节、没有链接的节点，空载单跑），不是 B5 的满负载；B5 的判定仍在 P4/S3 的批次 D。
    - **硬期限一层**：4.7.1 点名的是页的恢复、撤销守卫，来源的这两个不在其中，这里是延伸。同一棵树在 7763 上的实数是 3.54 秒，失败那次推算约 4.0–4.5 秒，加 `parse_max_wait` 的 2 秒，都在 15 秒之下。
    - 改后这道界只守硬期限一层（7.6 + 2 < 15），不再守规则一层的 5 秒。
    - 以后 guards-sources 落在 7763 上，记下的是不被截断的实数（`MaxUnitNodes` 的规模），P4 判 B5 的硬期限时可以用。
  - **测试怎样改**：
    - 恢复、撤销的种子、副本、每次的重置与样本，都在生产的请求配置下跑。取生产的值，依据是 M8/P2A 起垃圾箱的 bench（`99dca8aa`；今天在 `newTrashBenchOf`）。
    - 期限由界推出：界在 4 秒之下用 4 秒，否则用 15 秒。推出的期限要与起应用的表一致，每个样本还要断言界低于期限（审查 B 的 M1）。
    - `sourceHTTPBoundary` 仍断言 4 秒，留给 lint 的两个计时保存测试：4 秒是它们唯一的时间界。来源代价的种子、副本与样本改调 `sourceHTTPBoundaryWithin`（审查 B 的 M2）。
    - 计时的请求经 `sendRequestBy`，用 30 秒的客户端：`client()` 5 秒就放弃，期限提到 15 秒之后，它会先于界来判。
  - **为什么提高期限、界留 7.6 秒，不照 249 压低**：
    - 只有提高期限修得了这次的红：期限留在 4 秒，界取多少都是期限先判。
    - **先例**：
      - M10 决定 150：两个 ingest 用例的请求提到 30 秒，由界来判；
      - M10 决定 160：十万页摘要的成本用例，请求 4 s、worker 2 s 提到 6 s、3 s，校准的上限不动。
      - 13.4 第 8 条"request 4 s 不提高"的例外由此加第三个。
      - M10 决定 153 是"界必须低于期限"的先例：那次请求期限不动，界留在 bench 15 秒之下。新加的"界低于期限"的断言正是它的意思。
    - **249 不适用**：249 处理的是界本身在 7763 上被越过；这次的界在 42 个 7763 作业里一次也没碰到。界没有动，13.4 第 8 条的"不随当轮抬高"没有被碰。
    - **反向的先例**：M10/P2/S2 与 P3 在同一组守卫上几次撞 4 秒，当时都改了产品，并明写不提高 4 秒：
      - 恢复两次 4 秒失败，改了 `TrashChildren`、`TrashHistory`；
      - 撤销在 CI 40740783718、40768488401 上，三个状态的 sample2 都是 500（[P2 设计](../M10-llm-wiki/02-P2-source-area.md) 10.2）；
      - P3 的 CI 41247708831 是 4.002 秒的 500，改了撤销规划的读（[P3 的记录](../M10-llm-wiki/reviews/P3-revert-tree-review.md)）。
      - 那些都在负责人 2026-11-06 的话之前；此后照那句话只调时限。
    - **不减样本或规模**：sample2 正是历史增长最多的那一格；1 万个节点就是 `MaxUnitNodes`。
  - **守卫少证了的**：
    - 不再证明恢复、撤销能在测试配置的 4 秒之内完成。
    - 7763 上到 7.6 秒都照过，约是今天最大值的 1.7–1.9 倍；本机约 3.9 倍。
    - 本机的变异：
      - 撤销里加 3 秒的 sleep：改前红，改后绿（4.33–4.78 秒）；
      - 检查多做三遍：前后都绿；
      - 加 6 秒：红（9 个里 4 个过界）；
      - 加 7 秒：前后都红。
    - 不计时的请求换了判它的东西：每次样本之前删掉 1 万个节点的组，种子里的移动，原来受 4 秒的期限，现在只受测试客户端的 5 秒，到时报 "Client.Timeout exceeded"，不再是 500。它们今天约 1 秒。
  - **可以改判**：
    - 把两个界压到 5.6–6.3 秒（推算的 4.0–4.5 秒的 1.4 倍），取约 6.0，只改一个常量。
    - 代价：对推算的最大值只余 1.33–1.5 倍，比 249 自己的 1.4 倍还紧。失败那次比中位慢 25%，7763 上这样的尾巴会把偶发的红请回来。
    - 换来的：界离 7763 的常态更近。6.0 约是撤销 sample2 中位 3.47 秒的 1.7 倍，现在的 7.6 约是 2.2 倍；变慢在 1.7 倍到 2.2 倍之间的，压低之后才红。

### 2026-11-16：`main` 上 pgtest 的一次读超时 360（主持者的决定，负责人可以改判）

- **360 来源守卫的旁支合并之后第一次 `main` 的竞态一项红在 M1 的 `TestWaitForLockWaitsOnCountsTheRowsOfItsTable`：只重跑失败的项，不开旁支，记成观察的偶发；M12 收尾之前再出现一次就开旁支查**（run 43745380462）。
  - 失败：`server/internal/platform/postgres/pgtest/lockwait_test.go:98`，测试 0.87 秒时"timeout: read tcp 127.0.0.1:…->127.0.0.1:…: i/o timeout"。那一行以 10 秒的期限等 `users` 的一行锁；它之前的一行以 300 毫秒的期限故意等不到（`advisory`）。
  - 同一棵树在分支上的三次 CI 都绿；只重跑失败的项之后为绿。最近 80 次失败的 `ci.yml` 的注解里没有这个测试，也没有别的 `i/o timeout`。
  - 推断（没有复现）：300 毫秒那一次的查询被期限打断时，连接上留着过期的读期限，又回到池里给了下一行。要查时先在本机竞态构建下大量重跑这个测试，看 pgx 在期限打断之后是否把连接关掉再还池。
  - 理由：只出现一次，在测试帮手的测试里，不在产品的路径上；照 356、359 的先例，偶发只重跑失败的项；开旁支的代价大于一次重跑。收尾记录核对 P1 之后 `main` 的红时一并核对它（决定 334）。

### 2026-11-16：P3 的设计审查之后的取舍 361–377（主持者的决定，负责人可以改判）

361–377 是 P3 设计的十七条（P3-1 到 P3-17，P3-k 是 360 + k），设计审查之后照审查改写。每条在 [P3 设计](03-P3-server-correctness.md) 2.3，审查里的出处在 [P3 审查记录](reviews/P3-design-review.md)。P3-16 与 T4 的做法是主持者在第一次修订说明里定的（设计审查 C 的 Q1、Q2）。

- **361 `readReverted` 只有一个签名，收 `Nodes`**（P3-1）：五处构造计划都传，`RevertWrites` 传 `nil`；`CheckRevertSteps`、`RevertActionFacts` 不读顶，它们的"不传"是等价的变异，证据是调用链上的代码事实与一个会被抓到的对照（设计审查 A 的 M3）。
- **362 `needed` 也看段内后面的放回**（P3-2）：不改 `CheckRevertSteps` 的模拟；挡这一种的是 `needed` 与一格确定的测试。
- **363 domain 的 `world` 按根的时刻读组**（P3-3），同 `TrashChildrenOfAll`。
- **364 回滚的性质测试撤销回滚的变更集，请求与后台轮流，只数嵌套的根**（P3-4；设计审查 A 的 M2）：中间节点下的与两个成员以上的嵌套组交给后台撤销的性质测试。
- **365 标题键的切罗基文一步用标准库的 `strings.Map`，`shared.CherokeeCapital` 三处共用，`IndexKeyer` 原地换；`searchFold` 删掉，不升分词与 `Extractor` 的版本**（P3-5）。（P3/S1 实施改：不用 `strings.Map`，它把不是 UTF-8 的字节写成 U+FFFD；不导出的 `cherokeeCapitals` 逐 rune 原地换、别的字节原样留着；P3 设计 3.21 第 2 条。）
- **366 标题 id 带上自动链接的标签与原始 HTML 的源文本**（P3-6），与 `AnchorID` 相遇；不升提取规则的版本。
- **367 名字为空的判断放在 `CheckName` 与 `CleanClientName` 里，管显示名、工作区名与 PAT 的名字**（P3-7）：默认可忽略的字符照 Unicode 的推导精确地算（设计审查 B 的 M1），U+2800 不算空，标签块不去掉；先判空，只由 Bidi_Control 组成的名字从 `invalid_format` 变成 `required`。
- **368 MCP `log` 的 `"/"` 是整本，`""` 答 `too_short`**（P3-8）。
- **369 MCP 超时的文字说"没有写、拆小再试"，码照旧 `internal_error`；写的事务在提交时跨过期限的，`TxManager` 给提交的错误加标记，MCP 另用一句"结局未知，先读笔记本"**（P3-9；P3 设计修订核对 B 的 N14）。（P3/S1 第一次修复改：提交在调用的期限之前超过 `database.commit_timeout` 失败的，`FaultUnknown` 一支也认标记，答另一句"保存时失败、可能保存了、先读笔记本再重试"；决定 483，P3 设计 3.21 第 11 条。）
- **370 快照放在四个用例里，从各自第一条属于答复的读起，到 `alivePaths` 为止**（P3-10；设计审查 B 的 N3、N4）。
- **371 事件流按凭证（会话、PAT、任务）计数，账户满了答 429 `rate_limited`、`Retry-After` 30 秒，顶掉的一条立刻不算、别的原因重置的算到处理器结束，顶掉记 `debug`**（P3-11；设计审查 B 的 M5、N10）。
- **372 条目名的解读在 archive 适配器里，报告的两种新原因在任务结束时加、排在别的问题之后**（P3-12）。
- **373 按人的上限答 409 `transfer.too_many_jobs`，不带 `Retry-After`，常量 5；回滚算进计数但从不被拒；码的次序照今天的检查**（P3-13；设计审查 C 的 I2）。
- **374 T4：五种任务各设 `DiscardRunning`，`AtStart` 之后对每一种跑一次 `lost`；以 SQL 种排队行的测试同时种它的 River 行**（P3-14；设计审查 C 的 I3、Q2，主持者定）。
- **375 新的整个程序的测试都点名进第七份；T4 并进已有的 `TestServeFailsTheJobsLeftRunning`**（P3-15）。
- **376 GBK 的包里，没有标志、解得开的非 ASCII 名字都按 GBK 读，恰好合法 UTF-8 的也读**（P3-16；设计审查 C 的 Q1，主持者定）：偏离 4.6.6 第 2 步的"不是合法的 UTF-8"与末条"解读只改原本不是合法 UTF-8 的名字"，也偏离 05-lessons 第 5 节"宁可不做"。理由：中文 Windows 的包里 `图片`、`目录` 这类常用文件夹被拆成两个，是主要读者都会碰到的错；混合的包少（7-Zip、WinRAR、Python、Java、Go 写非 ASCII 名字都带标志，macOS 的包与本程序的导出没有候选）；误读的名字在报告里逐个列成 `name_gbk`。对"有 GBK 候选、又有没标志的真 UTF-8 名字"的包，这是相对今天的回归，已知限制照实写。
- **377 回收站一行在后台开始答 `transfer.too_many_jobs` 时留着"在后台恢复"的按钮**（P3-17）。

### 2026-11-16：P2 的设计审查之后的取舍 378–429（主持者的决定，负责人可以改判）

378–429 是 [P2 设计](02-P2-web-polish.md) 2.3 的52条（P2-1 到 P2-52，P2-k 是 377 + k）；每条照 P2 设计 2.3 抄，括号里的节号是 P2 设计的。其中 P2-1、P2-43、P2-46、P2-47 与 P2-42 的"先探、再定"是主持者在审查之后定的；审查里的出处在 [P2 审查记录](reviews/P2-design-review.md)"主持者的决定"一节。

- **378 次序与并行**（审查之后主持者定，C 的 Q1、Q2；0.1）：P2 的 Step 一次合并一个，后合并的先 `git merge main`，不按编号；只守真的硬依赖：S4 在 S1 之后开；S5 的 17.4、17.7 在 S4 合并之后；S6 的整句断言在 S2 之后，24.2 的焦点与 21.3、22.2 的接续在 S1 之后。S2 与 S3 可以并行，先做完的先合并。S6 在 P4/S2 的页面树结论之后才开（结论要改树时，在改法的 Step 之后），结论之前不占"同时在实施的不多于两个"的名额。（P2-1，P2 设计 2.3）
- **379 `ConfirmDialog` 被拒之后焦点留在"确认"上**，不给拒绝的说明：同 M8 的三个确认框（HI3 钉着），拒绝由 `role=alert` 说出；发送中 Esc 与"取消"不关闭对话框（4.1 的 23.16）。（P2-2，P2 设计 2.3）
- **380 `useForm` 的横幅只在焦点已丢时取焦点**（在 `body`，或在发送中禁用的元素上）：不改 M8 撤销框"被拒之后焦点在 Undo 上"；横幅经一层 `data-form-banner` 找到，不改各处的调用；取得焦点的横幅用全局的 `:focus-visible`，不加 `outline-none`（4.1 的 9.1）。（P2-3，P2 设计 2.3）
- **381 到达的焦点的四个新规则**：28.4 从导航提交起 `watchReader`，读者动过就只从无人处取；28.5 树打开的页带路由状态 `opened`，后退、前进回到它时标题从无人处取；`NotFoundPage` 的 `replacing`（23.14、18.6）只从无人处取（4.1）；同一路径与查询、只换 state 的 `replace` 不算新的导航，看守不重新起算，`offered` 接续（S6 的 21.3、22.2 用它，所以它们在 S1 之后合并）。（P2-4，P2 设计 2.3）
- **382 12.3 的范围加工作区首页的笔记本卡片**（`W/pages/workspace/workspace-home.tsx:41`）：同一种问题，一个属性（4.1 的 12.3）。（P2-5，P2 设计 2.3）
- **383 第 7 条**：用户菜单、邀请页与会话不可用时的三处"退出登录"共用 `useSignOut`；"有未保存的修改"取 `PageEditing.unsaved`，自动保存的 2 秒窗口里也问；别的标签页经 `nwiki.edits` 一问一答、等 100 毫秒（4.1.2）。（P2-6，P2 设计 2.3）
- **384 11.7 留在 S1，用 S1 的做法**（不依赖 S4）；S4 的就地更新让多数情形不再发生，S4 的测试照过它的格（4.1 的 11.7、4.4.2）。（P2-7，P2 设计 2.3）
- **385 单复数**：`en.ts` 里带计数、会显示 1 的十个键各加一个 `*One`，其余写明不改的理由（4.2.2）。（P2-8，P2 设计 2.3）
- **386 根下统一叫"笔记本的顶层"/"the notebook's top level"**，`index.root` 一并；LLM Wiki 与升级里说库根位置的"根下"不改（4.2 的 22.11）。（P2-9，P2 设计 2.3）
- **387 撤销的用词**：网页的英文一律 undo；服务端 `page/domain/errors.go` 的文字是 API 的词，网页不显示它，不改（4.2 的 23.1）。（P2-10，P2 设计 2.3）
- **388 38.6 的单位改成 KiB、MiB**（`W/i18n/format.ts:50`）：与 README、粘贴那一句一致，名副其实；改 10 个测试文件、19 行（4.2 的 38.6）。（P2-11，P2 设计 2.3）
- **389 15.3 用中性的说法**"没有这个名称的页面或附件"，不按扩展名猜附件：服务端只在有同名附件时才把 `x.png` 读成附件（`linking/domain/target.go:109-126`），按写法猜会与服务端不一致（4.2 的 15.3；移交原话是"按写法一开始就说没有这个附件"）。（P2-12，P2 设计 2.3）
- **390 22.16 只改说法，23.4 只改英文**：重开失锁的会话、冲突链到上级那一项（要服务端点名上级）进 v0.2（4.2）。（P2-13，P2 设计 2.3）
- **391 H4 的两句**：回收站"过了可以恢复的期限之后由系统永久删除"，不写天数、不写"立即"；版本列表"版本不能单独删除，系统也不会"，不提时间与清理（4.2 的 H4）。（P2-14，P2 设计 2.3）
- **392 只加 WebKit 的 375 检查用新项目 `webkit-narrow`**（`grep: /@narrow/`、`testIgnore: threeBrowsers`），不把 SE1、MC3、PG2 整份交给 `threeBrowsers`（4.3.1.2）。（P2-15，P2 设计 2.3）
- **393 焦点环与次要文字的颜色**：全应用的 `:focus-visible` 是 2 px 实色外框；亮色 `--ring` 改为 `oklch(0.556 0 0)`，八处 `ring-ring/50` 去掉 `/50`，destructive 的环改实色；亮色 `--muted-foreground` 从 0.556 调到 0.53；顺带找到的暗色 destructive 按钮的白字 2.89:1，加 `dark:bg-destructive/60`（4.3 的 12.4）。（P2-16，P2 设计 2.3）
- **394 26.9 只把命中加粗到 700**，颜色不动：命中的文字对高亮已达标，不达标的是颜色之外的提示；把高亮的底色调到对页 3:1 要改阅读视图的样子（4.3 的 26.9）。（P2-17，P2 设计 2.3）
- **395 `bdi` 用一个共用件 `W/i18n/isolated.tsx`**，范围是点名的几处加"名字后面紧跟数字、日期、另一个名字或箭头"的高风险几处（含变更集页"Renamed: “A” → “B”"）；其余约二十处进 v0.2（4.3 的 26.12、附录 A）。（P2-18，P2 设计 2.3）
- **396 13.2 加第 30 条**：名字与别的文字在同一行时进 `bdi`；flex 项里的长文字要 `min-w-0` 或 `[overflow-wrap:anywhere]`；焦点指示对相邻颜色 ≥ 3:1（8.2）。（P2-19，P2 设计 2.3）实际取第 31 条：P4/S4 先合并、取了第 30 条（决定 505、536、593）。
- **397 S4 的 spec 写在本文 4.4**，不另写 `specs/P2-S4-…`：spec 与它的测试、变异同在一处，审查一次。（P2-20，P2 设计 2.3）
- **398 S4 按文章的顶层块就地更新：尾先比、再头比，中间再按键贪心配一次，不做 LCS**：线性；留着的节点不移动；同名标题插在前面时留住原来那一节；链接的落点变了、本页相隔很远的几处同时变时，其间没变的块也留着（审查 A 的 M4；4.4.2）。（P2-21，P2 设计 2.3）
- **399 `ReadingContext` 删掉 `role`，加 `writes()` 与 `onWritesChange`，`revision`、`assetsExpire` 读最新的**（4.4.2）。（P2-22，P2 设计 2.3）
- **400 6.2 的重读也给"编辑"的 `forbidden`**：同一个根因，同一行（4.4.2 末条）。（P2-23，P2 设计 2.3）
- **401 焦点找不到同一个节点与同键的元素时，给它原来所在的那一块**（经配对映射到的顶层块，`tabindex=-1`，不展开 callout、不滚动），不再落到 `body`；任务勾选框照 M5/P6 的规则，找不回就不给（`page-tasks.test.tsx:103`、`:146`、`:217` 照过）（审查 A 的 I1、M3，修订核对 A 的 M11；4.4.3）。（P2-24，P2 设计 2.3）
- **402 8.1 先探查、复现了才改**；没复现就移到发布之后的人工清单（4.4.4）。（P2-25，P2 设计 2.3）
- **403 上传的超时**：请求体在发时 60 秒没有进度、发完之后 180 秒没有答复就放弃，两种说明，离开页面的提醒随之关掉；不设整体的超时。M 比 N 长：中止会让服务端的事务回滚，误判一次就毁掉一个健康的上传，而发完之后发送缓冲还要排空（审查 C 的 M2；4.5.1）。（P2-26，P2 设计 2.3）
- **404 15.1 在 S5 里给服务端加一个属性**（写下的尺寸 `data-nw-sized`），CSS 照它；只改 CSS 分不开写下的与图片自己的宽高（4.5 的 15.1）。（P2-27，P2 设计 2.3）
- **405 17.2b 只给提示的 `span` 加 `select-none`，空格留在外面**：空格放进去（移交的写法）在 jsdom 里少一个空格，约 30 处可读名字的断言要改成与真浏览器不符的样子（05-lessons 第 6 节）。三种引擎里复制都不带提示，审查 C 的探针（`m12/p2-review-c/probe1.txt`）证实（4.5 的 17.2b）。（P2-28，P2 设计 2.3）
- **406 17.4 下载的链接在点击时比到期，过期就再签再下**：只对下载的链接，新标签页的照旧靠到期之前的重读（13.2 第 27 条随之改）；放行的标记在这次派发之后立即清掉，S4 的就地换地址之后第二次过期也能重签（审查 C 的 M1；4.5 的 17.4）。（P2-29，P2 设计 2.3）
- **407 只有 PG15 进三种浏览器**：PG15 查阅读视图的就地更新（4.4.6）。AS6 只在 Chromium：17.2b、17.4 的三引擎行为审查 C 已探过，相同；15.2 只在 Chromium 复现（4.5、P2-42）。（P2-30，P2 设计 2.3）
- **408 27.2 在笔记本的外壳卸下时把树标为未确认**（照 P4/S2 的结论写：保留整棵树时照此，改成分层加载时照 4.6.6），重新挂上时页的外壳先读一次树再显示：比移交写的"点搜索结果时标记"宽（后退、前进、任何从笔记本外进来的路都算），不改搜索结果与根 store 的查找（4.6 的 27.2）。（P2-31，P2 设计 2.3）
- **409 24.8 的 `links` 事件的读排在同一个单元的 `pages` 事件要的那次树的读之后**：移交写的"跳过树里没有的页"不够（事件先于删除的答复到时树里还有那一页，4.6 的 24.8）。（P2-32，P2 设计 2.3）
- **410 21.10 只改"说的话"，不改标签页**：换版本时留着的标签页对新版本仍对，`version-page.test.tsx:227` 有意钉着它（4.6 的 21.10）。（P2-33，P2 设计 2.3）
- **411 26.5 连同历史的三个筛选与目录的"类型"**：同一个写法（4.6 的 26.5）。（P2-34，P2 设计 2.3）
- **412 23.19 连同回收站的预览**：预览同样不转小写，大写地址下恢复过的节点会被说成"页面不存在"（推断，4.6 的 23.19）。（P2-35，P2 设计 2.3）
- **413 33.1 没复现时套件不变**（照 00 号文档 4.5"排查不进套件"），复现了才把 SE1 (page) 的 375 一段拆成 `@narrow` 的一格（4.3 的 33.1）。（P2-36，P2 设计 2.3）实际复现了，拆了（决定 589）。
- **414 S3 不并进移交没点名的两处同类**：变更集页面包屑的 `current`（显示名加日期，要让 `BreadcrumbTrail` 收 `ReactNode`）、回收站预览 `h1` 的长名（没有现成的 375 检查）；记进 v0.2（4.3 的 32.4、38.10）。（P2-37，P2 设计 2.3）
- **415 "仍要退出登录吗"不算在途的上传**：S1 只看未保存的文字（P2-6）；上传在途时退出登录，行随会话失效照常说失败，记进 v0.2（4.5 的 18.1）。（P2-38，P2 设计 2.3）
- **416 26.18a 命中的两端都对齐到字素簇**：开头也往前移到簇的起点（移交只说末尾），命中盖住整个簇（4.6 的 26.18a）。（P2-39，P2 设计 2.3）
- **417 16.4c 读作"清掉已不成立的那一句"**：没有在途的上传、这一轮也没话说时清空播报区域；仍成立的（"已复制嵌入"）不清，不做定时清空（4.5 的 16.4c）。（P2-40，P2 设计 2.3）
- **418 S6 不做的两处**：回收站预览"在后台恢复"的开始也有在途的窗口（`trash-preview.tsx:124`），移交没写，进 v0.2；21.3、22.2 的"刷新之后不再说"只用 vitest 断言历史条目的 state，不在 HI2 加一次真的刷新（4.6 的 21.3、22.14）。（P2-41，P2 设计 2.3）
- **419 15.2 在媒体的 `DOMActivate` 之后的下一个任务里把 `open` 设回去**（主持者定"先探、再定"；修订核对 C 的 M9 改成不留状态）：`DOMActivate` 的目标是 `<summary>` 里的音视频时，取下这个 `<details>` 当时的 `open`，`setTimeout(0)` 里变了就设回；不听 `toggle`、`keydown`、`pointerdown`，不跨事件记东西。三种引擎里探过：Chromium 点控件的空白与时间处会开合（没修时复现），改了之后不开合，只派发一次 `toggle`；读者点 summary 的文字、键盘的 Enter 与 Space、点过控件之后程序里的打开（锚点导航、焦点还原），都照常；Firefox、WebKit 本来就不开合，改与不改相同（`m12/p2-check1-c/probe.log`，`fix=timeout`）。上一版留着记录、等下一次 `toggle` 复原，在 Firefox 与 WebKit 上会撤回点过控件之后程序里的打开；初稿听 `click` 的改法在 Chromium 里从不运行（审查 C 的 I1；4.5 的 15.2）。（P2-42，P2 设计 2.3）
- **420 菜单与选项的高亮在 S3 修**（审查之后主持者定，B 的 Q1 取甲）：菜单项、快速切换的选项与链接补全的选中项加 2 px、`--ring` 的内嵌环（底色照旧），对相邻的颜色至少 3:1；附录 A 的去处随之记（4.3 的 12.4）。（P2-43，P2 设计 2.3）
- **421 Playwright 的项目由一个检查钉住**：`make lint-web` 加 `tools/e2e-projects/check.mjs`，以 `playwright test --list` 断言 `firefox` 项目列出 PG15、`webkit-narrow` 只列出带 `@narrow` 的那几格（审查 C 的 M7；5.1）。（P2-44，P2 设计 2.3）
- **422 附件地址的拼法由 Go 与网页共用的样例钉住**：`testdata/m12/signed-view.json`，Go 一侧断言真的渲染与签名写出的就是它，网页一侧 `patch.test.ts` 读它核对 `addressesOnly` 与 `resign`（审查 A 的 I2；4.4.2）。（P2-45，P2 设计 2.3）
- **423 S4 开工时对着 S1 合并之后的代码重做 4.4.9 的核对**，记进 S4 的记录（审查之后主持者定，A 的 Q1；6.4 第 0 步）。（P2-46，P2 设计 2.3）
- **424 `resign` 不跳过已加载的图**（审查之后主持者定，A 的 Q2）：之后"在新标签页打开图片"拿到的仍是新的地址；每小时重下的带宽由 P4/S2 量（4.4.2）。（P2-47，P2 设计 2.3）
- **425 23.1 连同 `upgrade.approveBody` 的中文**：同一个用词，中文也改"改动"（4.2 的 23.1）。（P2-48，P2 设计 2.3）
- **426 22.13 用中性的一句，不先显示加载中**：树还没读到或读失败时说"它不在回收站里"，读到之后再照树分；改动最小（4.2 的 22.13）。（P2-49，P2 设计 2.3）
- **427 23.12 的中文用 `{count}+`**：与 `search.capped` 一致，不改模板的空格（4.2 的 23.12）。（P2-50，P2 设计 2.3）
- **428 9.5 的英文写"(at the top level of Plans)"**，不用所有格：以 s 结尾的笔记本名的所有格不好看（4.2 的 9.5）。（P2-51，P2 设计 2.3）前一半由决定 584 取代。
- **429 20.2 留被拒的那一句**（`importBusy`，回答这次按下），压下常驻的 `importUnderWay`（4.5 的 20.2）。（P2-52，P2 设计 2.3）

### 2026-11-16：P5 的设计审查之后的取舍 430–452（主持者的决定，负责人可以改判）

430–452 是 [P5 设计](05-P5-deploy-release.md) 2.3 的二十三条（P5-1 到 P5-23，P5-k 是 429 + k）；P5-6、P5-11 的核对在公开之前、P5-21、P5-22、P5-23 是主持者在审查之后定的。每条的理由在 P5 设计 2.3，审查里的出处在 [P5 审查记录](reviews/P5-design-review.md)。

- **430 发布的守卫都是 `platform/buildinfo` 的 Go 短测试**（P5-1）：版本号的默认值；三份工作流的权限、触发与调的脚本；闸门、说明、版本号的 tag 与 Release 的脚本对着假的 `gh`、`docker` 与临时的仓库；要 `bash`、`git`、`jq`。
- **431 MCP 的版本由盖了版本号的 `make test-go-short` 守住**（P5-2）：`mcpOptions` 拆出来，测试的二进制盖 `0.0.0-short`；mcpserver 另有握手测试。
- **432 README 写的默认值也进版本号的守卫**（P5-3）：示例与注释不进。
- **433 镜像的三个 label**（P5-4）：`created` 是提交的时间，不是构建的时间；image-smoke 逐字核对。
- **434 tag 只认 `vX.Y.Z` 与 `vX.Y.Z-rc.N`；`CHANGELOG.md` 的标题 `## X.Y.Z（日期）`，发布之前写"未发布"**（P5-5）：正式版本要写日期，这一节不能是空的。
- **435 actionlint 只在 runner 上跑（`make lint-actions`，CI 的 `costs-1`），`-shellcheck=`**（P5-6）：主持者照决定 308 的先例定（P5 设计审查 A 的 I1、C 的 Q1）。
- **436 演练与发布都不用构建的缓存与 pnpm 的 store 缓存；演练断言镜像是 runner 的架构**（P5-7）。
- **437 SBOM 与许可证在只读的 `sbom` 作业里**（P5-8）：能签名的作业里第三方的只有 cosign-installer，登录用 shell。
- **438 镜像的 SBOM 由 syft 从 amd64 的那一份生成，前端的由 `pnpm sbom --prod`；两份都是 SPDX 2.3**（P5-9）。
- **439 镜像里的第三方许可证 `/licenses` 在 Dockerfile 里收集**（P5-10）：Go 一侧缺许可证文件让构建失败，npm 一侧照实记成"没有文本"。P5/S2 改：npm 一侧带上工作区包的依赖与 tailwindcss，文件名不收代码，两侧的清单对着发出去的东西核对（决定 612–614）。
- **440 Release 先草稿、传附件，两种架构上核对过才公开；推荐打开 immutable releases**（P5-11）：核对在公开之前是主持者定的（P5 设计审查 B 的 Q1）。
- **441 版本号的 tag 已在、指着别的摘要时失败，不改指**（P5-12）：判"不在"只认 registry 答的 not found。Release 有没有、是不是草稿读 REST 的列表，任何失败都失败（P5/S2，决定 615）。
- **442 只签索引的摘要**（P5-13）。
- **443 索引带 `description`、`source`、`licenses` 三个注解，值取自 label**（P5-14）。
- **444 rc 失败时没公开过的草稿删掉，公开了的标"作废"；GHCR 里留下的不删**（P5-15）。
- **445 `deploy/compose.yaml` 用一个不是超级用户的库角色，附件绑定 `./data`，默认网络钉子网、`trusted_proxies` 信任它**（P5-16）：P5 设计审查 C 的 I1、M3、M4；C 在 18.7 上照样例装过、恢复过。
- **446 升级的演练从 M12 最早一条新迁移之前升上来；M12 没有新迁移时从 `5e8af5b5` 升**（P5-17）：P5 设计审查 C 的 M2。
- **447 syft、cosign 与 BuildKit 的版本在工作流里显式写**（P5-18）。
- **448 闸门不等 CI**（P5-19）：CI 绿了之后在同一次运行上重跑。
- **449 版本号的 tag 与 Release 的两步写成脚本，对着假的 `docker`、`gh` 测试**（P5-20）：P5 设计审查 B 的 M2。
- **450 `v0.1.0` 只因流水线自己的错没过核对时不出 `v0.1.1`**（P5-21）：修流水线、手动校验、记进收尾记录；公开草稿先问负责人。主持者定（P5 设计审查 B 的 Q2）。
- **451 P5/S3 只等 P4/S3 与 P5/S2，不等 P2 全部合并**（P5-22）：P2 的部分收尾时补进 `CHANGELOG.md`。主持者定（P5 设计审查 C 的 Q2）。
- **452 总体设计第 14 节的 `NOTIFY` 与 SSE 两行归 P5/S3 改，P4 不改它们**（P5-23）：主持者定（P5 设计审查 C 的 M7、修订核对 C 的 N14；同 P4 设计的 P4-26）。

### 2026-11-16：P4 的设计审查之后的取舍 453–478（主持者的决定，负责人可以改判）

P4 设计的 P4-1 到 P4-27，设计审查之后照审查与修订核对改写；P4-26（`NOTIFY` 与 SSE 两行归 P5/S3）已是决定 452（P5-23），不另编号。正式编号 453–478：P4-1 到 P4-25 是 452 + k，P4-27 是 478。每条在 [P4 设计](04-P4-performance.md) 2.3，审查里的出处在 [P4 审查记录](reviews/P4-design-review.md)。P4-2、P4-3、P4-9、P4-16、P4-19、P4-21 的 B3、P4-23、P4-24 是主持者在第一次修订说明里定的。

- **453 B5 的满负载照 M8 的口径（两种数据的形状），用现成的基准量；每轮的值照 M8 的量法：回收站与撤销的基准三次的均值，后台的一个样本，两轮取较大**（P4-1；设计审查 B 的 M3、M4）。
- **454 推断用本机对 7763 的实测之比，改决定 318 的倍数一半**（P4-2；设计审查 B 的 Q1、M1、M2）：两边同口径（7763 各次的中位除以本机在同一批提交上的中位；每个作业取最近的至多三次 7763，本机在那些提交上只跑那一族），只留回收站与撤销、升级两族，B8 取较大；对硬期限的推断另并列 × 1.25 的一行；旧倍数的结论并列，不同时列给主持者。它多半决定 B4 删除的结论（乘实测之比约 12–15 秒，乘 4.6 约 28–35 秒，推断），负责人可以改判。
- **455 B1 的吞吐、1 小时与保存照原预算直接判；尾巴与体积先比 M8 记的最大，超了在同机同语料上对照 `308eafc6` 两轮整轮，也慢 20% 以上才算回归；积压只记一轮；建索引时与导入的清空期间沿用 M8 的数**（P4-3；设计审查 B 的 I1、I2，C 的 M5）。
- **456 搜索语料的词表取仓库的文字、进 `testdata`；查询的词确切地植入，词表与段落不让非重类的词自然出现**（P4-4；设计审查 A 的 M1、M2）。
- **457 对外量的大笔记本经导入生成的 vault 建起**，不写 SQL 的播种；30 万页只在进程内由规模开关量（P4-5）。
- **458 驱动、采样与 vault 的生成器在 `server/tools/perf/`，Go 只用标准库，读库经 `docker exec … psql`；MCP 手写 JSON-RPC；起服务的是 shell 脚本**（P4-6；设计审查 A 的 M6）。
- **459 内存取 gctrace 的峰值减每轮一次的基线（空闲 130 秒之后强制 GC 的存活堆），末尾取窗口之后第一行的目标作上界；被访问日志插断的行拼回、缺号重量**（P4-7；设计审查 A 的 M4、M5）。
- **460 只为准备的三个登录的桶放宽到每分钟 600，量的限流照默认**；浏览器的测试各自登录（P4-8）。
- **461 B4 的正常形状是导入上限的 vault；被几千页链着的页的改名与 400 页 × 2,000 条链接的 `move` 算病态，用已有的数记成已知限制，并换算到 7763（推断）**（P4-9；设计审查 B 的 Q4、M9）。
- **462 B4 在空载上判，每次删除之前等搜索的队列清空；在编辑者的负载下只记**（P4-10；设计审查 B 的 M8）。
- **463 页面树的量法**：零点是新文档的 `timeOrigin`，终点是条件成立之后的第二帧，每次 T1 开新的标签页、清 HTTP 缓存，T2 只看根列表的第一行；零点的反向对照把主 JS 压后 1 秒（P4-11；设计审查 C 的 I1、N5、U1、U3）。
- **464 树的压缩离线算**（gzip 1、6 级，zstd 3 级）（P4-12）。
- **465 B9 在 Chromium 里量解析本身**：以 Vite 的库构建（`configFile: false`）把编辑器的解析打成一页小的包，e2e 加两个与 web 同版本的 devDependencies，不用别名，增量解析经 `state.update`，`knip.jsonc` 给 e2e 忽略构建用的 web 的 `vite`；按键到下一帧只记（P4-13；设计审查 C 的 M2，修订核对 C 的 M6）。
- **466 10 万页的树的重读用 Go 的驱动模拟标签页**（P4-14）。
- **467 记录写进 P4 设计第 10 节与各 Step 的审查记录，原始数据在本机 `~/.local/state/nervewiki/m12-p4/`，记 sha256**（P4-15）。
- **468 B5 的硬期限照 4.7.1 取 7763 上的实数加 2 秒；窗口只取这几条守卫与它们量的文件没再改过的那一段（`main` 上包含那次改动的运行），不到三次就用已有的、写明次数，一次也没有时取改动之前最近的至多三次、乘本机在两个提交上之比、写"推断"；另并列两行推断（7763 各次的中位 × 1.25 加 2 秒；本机基准里最慢的一阶段（多半是 ANALYZE）对本机守卫之比 × 7763 的守卫加 2 秒），任一行过 15 秒就同规则一层列给主持者与负责人，部署文档的硬件下限照他们定的写；B6、B10 的窗口同样取**（P4-16；设计审查 B 的 I4，修订核对 B 的 M13）。
- **469 B5 的基准先只跑界线那一档；规则没过时，降常量的候选先插值，再以手跑的档位开关在两边各补一档、跑两轮**（P4-17；设计审查 B 的 M5）。
- **470 现有守卫的规模开关只在手跑时用，规模与期限由一个纯函数给：没设时是守卫今天的常量、照字面断言；设了用生产的期限，客户端与写超时等得比期限久，只记不判界；默认值是不带构建标签的文件里返回字面的四个函数，守卫从那里取、计时之前断言自己的期限与客户端等于它，计时与随规模变大的请求经同一个帮手**（P4-18；设计审查 A 的 I2、I3，修订核对 A 的 M11、M12）。
- **471 进程内的经 `pgtest`（`fsync=off`），对外的容器不加；B4 的删除另在 `fsync=off` 的容器上量，推断是"`fsync=off` 的本机数 × 比，加两种容器之差（不乘）"**（P4-19；设计审查 B 的 Q2）。
- **472 knip 经 `e2e/package.json` 的 `perf` 脚本读到 `e2e/perf/` 的配置，配置写字符串的 `testMatch` 与绝对路径的 `globalSetup`；`knip.jsonc` 只给 e2e 加一节忽略 `vite`**（P4-20；设计审查 C 的 M1，修订核对 C 的 M6）。
- **473 B2、B3 的负载照 M9/P2 的形状，B3 的值放进 `_meta`；B3 取两轮六次里的最大**（P4-21；设计审查 A 的 M7、B 的 I3）。
- **474 S3 的批次分四夜，对照与候选有条件时另一夜，每批整段持 heavy 的锁**；机器时间约 16–20 小时，比 4.7.3 原来的估计多（P4-22；P4 设计 3.8）。
- **475 lint 守卫的 worker 期限提到界之上（worker 6 秒、请求 12 秒），断言界低于期限**（P4-23；设计审查 B 的 Q3、A 的 I3）：M10 决定 160 留下的（摘要的界 5.2 秒在 3 秒的 worker 之上，轮不到它判），P4/S1 照决定 359 的做法顺带修；总体设计 13.4 第 8 条那一句随 P4/S1 改（第 7 节"总体设计的修订"第 25 条）。
- **476 P4/S2 不等 P4/S1，P1 之后就能开，与 S1 并行**（P4-24；设计审查 C 的 Q1）。
- **477 语料的载入测试在 `!race` 的文件里，`test-go-costs-3` 单独一行带 `-v`**，不改竞态的分份（P4-25；设计审查 A 的 I1）。
- **478 批次 E 另量一组小机器档**（解析预算 5 MiB、`GOMEMLIMIT=1536MiB`），给部署文档（P4-27）。

### 2026-11-16：P3/S2 的决定 479–482（主持者的决定，负责人可以改判）

479–482 在 S2 的审查之后定。每条在 [P3 设计](03-P3-server-correctness.md) 3.22，理由与审查里的出处在 [S2 的审查记录](reviews/P3-S2-nested-trash-roots-review.md)"主持者的决定"一节。

- **479 后台撤销的性质测试（`TestTheJobsStepsReplayToStar`）的下限，生成器的随机数用法一变就照约定全部重算**（S2 审查 B 的 Q1；P3 设计 3.22 第 10 条）：每个下限是 20 个基 × 2,000 次与 40 个基 × 400 次里最少次数的三分之二、向下取整。修复第 1 轮加第三层之后 20 个全部重算（降 9、升 18）；主持者原先只说降审查 B 点名的 11 个（加第三层之前的生成器上的），实施者照同一约定全部重算、写明偏离，主持者接受。
- **480 多层嵌套（单独删的项套单独删的项）给计数 `nestedDeep` 与下限（`-short` 6、全量 48），不只写进 3.22**（S2 审查 B 的 M1；3.22 第 12 条）：`folder` 造 z 时另一半在 z 下加 w，先单独删 w、再单独删 z，z{zz} 不拆。
- **481 历史文档里被决定 304 改了的旧结果加后注、不改写原文**（S2 审查 C 的建议；3.22 第 1 条）：M8/P4 `04-P4-background.md:530`、`:735`，M9/P3 `03-P3-batch-revert.md:1899`；M11 已是 `done` 的移交不动。
- **482 网页的 `transfer.revertLeftGroup` 提示由 P3/S3 改，S2 不碰网页**（S2 审查 A 的 N1、核对第 1 轮 A 的 C1-N2；3.22 第 1 条）：与决定 304 一致；后半句"不要先撤销这次撤销"不再必要，前半句"逐项恢复就完成这一组"仍对（任务停下之后直接再撤销答 `changed`）。

### 2026-11-16：P3/S1 的决定 483–486（主持者的决定，负责人可以改判）

483–486 在 S1 的审查与修复里定。每条在 [P3 设计](03-P3-server-correctness.md) 3.21（486 在 10.1），理由与审查里的出处在 [S1 的审查记录](reviews/P3-S1-small-correctness-review.md)"主持者的决定"一节。

- **483 MCP 的 `FaultUnknown` 一支也认 `TxCommitFailed` 的标记**（S1 审查 B 的 M1，取甲；P3 设计 3.21 第 11 条）：带标记的答 `the server failed as its changes were being saved, so they may or may not have been; read the notebook to see whether they were before retrying it`，不带的照旧 `the server failed`；码照旧 `internal_error`，日志照旧，REST 不变。理由：默认 `commit_timeout` 2 秒短于 `request_timeout` 15 秒，提交先超时、调用的期限未到是更常见的一种，旧的答复让 agent 重试，不带 `base_version` 的 `edit` 可能重复追加。P3 设计 3.7"这时请求的期限也已过"一句照实改。
- **484 事件流的 hub 顶掉一条流时减它自己账户的数（`oldest.userID`），减到 0 就删掉这一项，与关闭共用 `uncount`**（S1 审查 C 的 M1 的可选改法，核对 C 的 C1-N2；3.21 第 12 条）：凭证跨了账户的回归下数仍对、表不留 0 项。
- **485 网页的事件流只改注释**（S1 审查 C 的 N3，核对 C 的 C1-N1；3.21 第 14 条）：503 `not_ready` 与 429 `rate_limited`（请求的限流，或账户到了流数的上限）都带 `Retry-After`、照它等；行为不改。P3 设计 4.1"网页不改"指的是行为。
- **486 只补核对的 Nit、碰一处很小的生产代码的修复轮，由主持者核对**（S1 修复第 2 轮；P3 设计 10.1）：主持者读 diff、看变异与分支 CI，本地不跑 `make test` 与 `make e2e`，分支 CI 跑全套（02-workflow 3.4 的小轮；`main` 上合并之后照常两次 CI）。

### 2026-11-16：P4/S4 设计的决定 487–505（主持者的决定，负责人可以改判）

487–505 是 P4/S4（页面树的渲染，P4 的条件修复）的设计审查之后的取舍，依次是设计里的临时号 P4S4-1 到 P4S4-19。每条在 [P4/S4 设计](04-P4-S4-page-tree.md) 2.3，审查里的出处在 [P4/S4 设计的审查记录](reviews/P4-S4-design-review.md)。

- **487 只改怎样画，不改列什么**（P4S4-1）：左栏仍列整棵树，首页仍列全部根下的页，全部行最终都在 DOM 里；原型不改列出的内容就过了三条界线，平的每格离界至少 1.7 倍，不需要负责人。
- **488 每一行便宜**（P4S4-2）：行是 `memo` 的组件，属性是列表算好的原始值，上下文随树的一生不变，链接是普通的 `<a>`、点击由树交给路由；一棵树一个菜单，对话框选了才挂。
- **489 先画首屏，其余分批**（P4S4-3）：每个页的列表第一次渲染 100 行，之后每帧画完再加 400 行，直到全部；画全之后一次多出超过一批的照样分批；树要聚焦的行与它路上的列表立刻画到。
- **490 首页、页与标签页的列表同样**（P4S4-4）：`SubpageList` 的行是 `memo` 的普通链接，同样分批；标签页按标签重挂。
- **491 不用虚拟列表**（P4S4-5）：左栏随整页滚、嵌套的分组、页内查找与读屏、拖放与焦点都要另付代价；5 万、10 万页要分层加载时一起看（v0.2）。
- **492 不取"首页只列前若干个加更多"**（P4S4-6）：改的是列什么，是产品的事；单靠它左栏仍有一万行。
- **493 不取悬停、聚焦时才建菜单**（P4S4-7）：换触发时焦点会丢，触屏要另一份打开逻辑。
- **494 拖放照旧每行注册**（P4S4-8）：注册不加 DOM 的监听者，委托到列表要重写放下的判断。
- **495 守卫是 vitest 里 1,000 页上的结构的断言，不计时**（P4S4-9）：第一次提交与每批的行数、别人改一次树只渲染新的一行与至多十个 reaction、行里没有自己的菜单与路由链接、画全之后的增长分批、导航只重渲染两行；原规模照 P4/S2 的方法手跑。
- **496 不加 `content-visibility`**（P4S4-10）：分批之后只快 18 ms，T2 的长任务多约 50 ms。
- **497 P2/S1 合并之后开**（P4S4-11）：P2/S1 改了同一块（`opened`、`arrived`），S4 的点击处理带上同样的状态。
- **498 "菜单开着"经行登记的句柄告诉那一行，列表的 `through` 只在没画全时读**（P4S4-12；设计审查 A 的 M1、B 的 Q1、C 的 M5）：量得列表读一个盒子比原型多 6–37 ms 的长任务，取不让整列重算的写法。
- **499 菜单开着时它那一行离开树，菜单关上，焦点经 `focusActions`**（P4S4-13；设计审查 A 的 M3）：挪走的到新的那一行的按钮，删了的到树的标题。
- **500 "删的是不是正显示的页"每次渲染算，同今天**（P4S4-14；设计审查 A 的 M4）。
- **501 列表的写法记进总体设计 13.2 的新一条，不放进第 25 条**（P4S4-15；设计审查 C 的 M7）：第 25 条是"写者的内容决定数量的地方设上限"，页的列表不设上限；新的一条写明它是第 25 条的例外与理由，号照 505。
- **502 树的菜单保持模态**（P4S4-16）：平的一万页里开一行的菜单约 0.54 秒的长任务，大头是模态（同一构建改成不模态，降到约 0.08 秒）；S4 不比今天慢（542 对 702 ms），打开菜单不在 4.7.2 的界线里，照"超出设计预算才修"不扩范围；不模态是 v0.2 的候选。
- **503 菜单开着时树变了，触发重新定位，不关菜单**（P4S4-17；修订核对第 1 轮 A 的 C1-M1）：打开时在事件处理里量，每次提交之后从那一行当前的按钮再量；别人的改动不该关掉用户正在用的菜单。
- **504 笔记本没了、编辑没存时，左栏照最后找到的那一本**（P4S4-18；修订核对第 1 轮 A 的 C1-M2）：同今天。
- **505 总体设计 13.2 的新条按合并的先后取号**（P4S4-19；修订核对第 1 轮 C 的 C1-I1）：P2/S3（决定 396）与 P4/S4 都在 13.2 加一条；先合并的取第 30 条，后合并的取第 31 条，后合并的那一个在自己的文档提交里改自己的设计、决定与第 7 节"总体设计的修订"里的号。照预期 P2/S3 先合并，P4/S4 取第 31 条。

### 2026-11-16：P2/S1 的决定 506–509（主持者的决定，负责人可以改判）

506–509 在 S1 的审查与修复里定。每条在 [P2 设计](02-P2-web-polish.md) 10.1，理由与审查里的出处在 [S1 的审查记录](reviews/P2-S1-focus-and-shared-review.md)"主持者的决定"一节。

- **506 上传对话框里来源的重试，横幅在前（`banner ?? explain(upload.failure) ?? server.banner`），"Add source"发送中禁用、不卸下**（S1 审查 A 的 M1；P2 设计 9.1）：`focusLost` 不扩到"焦点在对话框容器上"：那比 9.1 的规则宽，会动读者自己放的焦点。设计 9.1"只改 `form.ts` 就全改到"照实改。
- **507 同一路径、换了查询或片段的 `replace`（目录、历史的筛选，"在哪里找"）：看守重新起算；已挂过标题的记录（`offered`）接续**（S1 审查 B 的 M1；P2 设计 26.4、4.1.4、8.2 第 6 条）：之后挂上的"页面不存在"不当作第一次。
- **508 实施者撤自己的、已作废、还没开始的排队运行，可以，照样写进报告**（S1 审查 C 的 Q1）："排着的不要杀"指别人的运行，也指已开始的。
- **509 13.2 第 12 条（P2 设计 8.2 第 1 条）的新文字取实施者的建议**（修复第 1 轮的鼠标探针，S1 审查 A、C 的未核实的怀疑）："焦点不动：按下'确认'时它拿着焦点的（键盘，Playwright 的三种引擎用鼠标也是），发送中与被拒之后仍在它上面（丢了就还给它）；浏览器不给点过的按钮焦点的（Safari），焦点留在原处"。Safari 的情形交 P5/S3 的人工清单。

### 2026-11-16：P4/S2 的决定 510–518（主持者的决定，负责人可以改判）

510–518 在 S2 的实施、审查与修复里定。数在 [P4 设计](04-P4-performance.md) 10.2，与设计不同之处在 4.11，理由与审查里的出处在 [S2 的审查记录](reviews/P4-S2-browser-measures-review.md)"主持者的决定"一节。

- **510 页面树平的形状三格超，在 v0.1 修，开条件的 Step P4/S4**（主持者的消息，页面树的报表之后；4.7.2、4.7.5）：1 万页都在根下时 T1 3.0 秒、T2 1.6 秒、其间长任务 1.4 秒，复核照样超，慢在渲染；深、带附件全过。S4 的设计已提交（`7fdb9f58`，决定 487–505）；P2/S6 排在它之后。
- **511 B9 超，开条件的 Step P4/S5（编辑器的解析）**（主持者的消息，B9 的第一次报告之后；数照修复第 3 轮之后的重量）：一串空格、转义之后一串空格、有 `|` 的一行之后一行空白三种每次解析约 81–82 ms，在段尾打一个字同价；标题没闭、标题在最后一行闭合两种多行的引用式定义，插一个字 86–90 ms；复核照样超。`<?` 连串约 75 ms，不超。S5 的设计在审查。
- **512 B9 的量法**（S2 审查 B 的 I1、I2、M2，核对 B 的 C1-M1、C2-I1）："整篇"量一次解析，走粘贴的路子；加"段尾插一个字"一格，与段中并列，都照 80 ms 判；加四种多行的引用式定义；每种形状的每一格在一个新的页里量（先收一次垃圾），热身 2 次、量 20 次取中位。P4/S5 的再量照这个量法。
- **513 附件列表的重读记成已知限制**（S2 审查 C 的 I1，取它的 Q1 的甲；判据所在的情形是主持者看过第一次报告之后定的）：判据取 A 持全部页的情形（移交那一次的标签页把附件列表展开到底），P4 设计 4.7 的情形（只持第一页）也量；4.7 写"20 秒的上传窗口里约 6–7 次保存"，所以 B 的最后一次停顿落在窗口里。正式两轮照复算记成 0 / 5、3 / 5，复核两轮 1 / 5、0 / 5，"第一次超、复核过"，记成已知限制，不开条件的 Step。记录写出三轮的数、机制（同一登录的桶被附件列表的重读用完，最后一次停顿的那次保存答 429，编辑器不自己重试 429）、读者的退路（字还在编辑器里，再打一个字或按保存就补上；离开时没存的编辑会先问）与 v0.2 的候选（429 之后照 `Retry-After` 重试一次，或"先比较 id 与名称再读"）。
- **514 5 万、10 万页只记**（主持者的消息，第一次报告之后；P4 设计 4.9）：10 万页的树让渲染进程崩溃，写进已知限制，分层加载进 v0.2；已知限制与 v0.2 的清单由 P4/S3 写，总体设计第 14 节页面树一行先由 S2 写上一句。
- **515 T3 挪到 T2 之前量，断言视口里有孩子**（S2 审查 B 的 M1）：深 212 ms、带附件 211 ms，是 P4/S4 的"改前"。
- **516 实施中定的各条都接受**（主持者的消息，第一次报告之后，与修复第 1–3 轮的说明）：实施报告第 6 节原来的 18 条（第 15 条后来改判为偏离，照 513 处理）与修复三轮新加的，列在 S2 的审查记录"实施中定的"一节、P4 设计 4.11，不逐条编号。
- **517 只改 `e2e/perf/` 的提交，推送之前的门禁取轻的**（主持者的消息，第 5 个提交推送之前）：`make lint gen-check knip build` 与 `make test-go-short`；分支 CI 跑全套。
- **518 大图（P4 设计 4.6）挪到 P4/S3**（主持者给 S2 的说明）：S2 时 P2/S4 没合并；`images.perf.ts` 在 S2 里只试跑过一次。

### 2026-11-16：P4/S5 设计的决定 519–529（主持者的决定，负责人可以改判）

519–529 是 P4/S5（编辑器的解析，B9，P4 的条件修复）的设计审查之后的取舍，依次是设计里的临时号 P4S5-1 到 P4S5-11。每条在 [P4/S5 设计](04-P4-S5-editor-parse.md) 2.3，审查里的出处在 [P4/S5 设计的审查记录](reviews/P4-S5-design-review.md)。

- **519 三处扫描在编辑器自己的配置里改成线性，语法树不变**（P4S5-1）：一串空格（在 `HardBreak` 之前）；后面没有 `>` 的 `<` 与没有 `?>` 的 `<?`，`<?` 之后的电子邮件只试一次（在 `HTMLTag` 之前）；`www.`、`http(s)://` 自动链接的括号只数一遍、`&` 只找到网址开头、被拒的记下且不交回（在 `Autolink` 之前）。原型在 Chromium 里把超的、在界线上的都降到 1 ms 上下（每格量 20 次）。
- **520 `inlineLimit` 留 10,000**（P4S5-2）：三个扫描之后上限处最贵的是 `*_` 交错，每次约 62 ms，在界线之内；读者什么都看不到变。
- **521 跨行的引用式定义另设上限 `referenceLimit = 2_000`**（P4S5-3）：标题的两种插一个字 86–90 ms（P4/S2 修复第 3 轮），真的编辑器里在定义的第一行按一个键 75–110 ms、有长任务；2,000 时约 3–5 ms、按键没有长任务。读者看得到的只有长过 2,000 码元的跨行定义（收尾了的也算），仓库的样本里没有跨行的定义。
- **522 不打补丁，不加依赖**（P4S5-4）：13.5 第 5 条，能在自己的代码里兜住的不改依赖。
- **523 守卫是结构的测试**（P4S5-5）：关掉上限只留扫描的二十万码元十六种、读码元的次数（上下界）、有没有扫描树逐节点相同（32 位的随机文档带各分支的下限，加手写的文档）、定义的上限两侧；lezer 自己就平方的几种按"有扫描不比没有扫描慢"的倍数判，界按格定。
- **524 找到的形状进测试，也进 B9 的判据**（P4S5-6）：`)` 结尾的网址、以实体结尾的网址、被拒的网址、`<?` 之后的电子邮件等，与十九种、四种定义一样判；`<ab:` 连串加 ` >` 是 lezer 自己的平方，只记。
- **525 按键的视图一侧只记**（P4S5-7）：P4 设计 4.4 定为只记，不在 S5 查。
- **526 任何形状都不比今天慢**（P4S5-8；设计审查 A 的 M2、B 的 I1）：改后两轮的较大中位数不高于今天的加今天两轮之差（至少 1 ms）。
- **527 被拒的网址也改成线性**（P4S5-9；设计审查 B 的 I1）：记下拒的理由，同一个域名上后面的起点不再重配，被拒的不交回 `Autolink`；只不交回仍是平方。
- **528 扫描不管 CDATA**（P4S5-10；设计审查 C 的 I3）：没收尾的 `<![CDATA[` 今天就不贵，少一处要守、要照抄的。
- **529 推荐 P4/S2 合并之后再开**（P4S5-11；设计审查 C 的 Q1）：第 3 个提交（`e2e/perf/`）照 P4/S2 合并之后的 `editor.perf.ts` 改；与 P4/S4 谁先由主持者排。

### 2026-11-17：P3/S3 的决定 530–532（主持者的决定，负责人可以改判）

530–532 在 S3 的审查之后定。每条在 [P3 设计](03-P3-server-correctness.md) 3.23，理由与审查里的出处在 [S3 的审查记录](reviews/P3-S3-transfer-jobs-review.md)"主持者的决定"一节。修复第 1 轮收下的几条 Nit 不编号，在审查记录里。

- **530 决定 482 的提交 `a48ace20` 留作单独的提交**（3.23 第 2 条）：决定 482 的文字在 S3 的 `en.ts`、`zh-CN.ts` 里（P3 设计 3.11 的文件），断言它的 `transfer-background.test.tsx` 不在 3.11 的文件表里；实施者单独成一个提交，6.3 没有列，留下。
- **531 `README.md:192`、`:193` 的两句与总体设计 13.1 第 4、23 条随 S3 的文档提交，不随代码的提交**（偏离 P3 设计 6.3 第 1–3 条；3.23 第 1 条）：文档的提交是主持者的；代码的提交不碰 `docs/` 与 `README.md`。
- **532 "不是 UTF-8"的文字不改**（S3 审查 B 的 N3；3.23 第 12 条）：契约、domain 的注释与网页两种语言说这两种名字"不是 UTF-8"，对读者的意思是"名字照原样读不出"。从字段读出的 ASCII 名字（`??.md` → `说明.md`）与决定 376 下读成 GBK 的合法 UTF-8 名字（`ü.md` → `眉.md`）其实也是合法 UTF-8，3.23 记一句。

### 2026-11-17：P4/S4 的决定 533–537（主持者的决定，负责人可以改判）

533–537 是 P4/S4 的实施、审查与修复里定的。数在 [P4/S4 设计](04-P4-S4-page-tree.md)第 10 节与 [P4 设计](04-P4-performance.md) 10.2"S4 之后"，与设计不同之处在 S4 设计 3.10，理由与审查里的出处在 [S4 的审查记录](reviews/P4-S4-page-tree-review.md)"主持者的决定"一节。实施中定的各条接受，列在记录的"实施中定的"一节与 S4 设计 3.10，不编号。

- **533 再量发现"移到……"慢了一倍，加一个代码提交修打开它的路**（主持者的消息，再量之后）：深的形状里这个对话框在 `2bc9f7da` 上 2.3–2.5 秒（P4/S2 1,258 ms）。行每次渲染给对话框新建的函数；菜单在挂上对话框的那次提交里关上，行随之再渲染，对话框跟着再渲染一遍，表单把整棵树的 `canHold` 与路径又算一遍。`f2f7ace0` 让行给状态的 setter，对话框只渲染一次，回到约 1.3 秒；对话框本身不改（它对每页调 `canHold` 仍交 P2/S6）。修复第 1 轮又加三个代码提交；文档的提交排在最后（S4 设计 5.1 原来是第 4 个）。
- **534 画全之后别人改一次树，`PageList` 渲染两遍，不改**（S4 审查 B 的 N3）：reaction 5 个，不是原型的 4 个。钩子要拿上次画到的行数与这次比，才分得出超过一批的增长；在渲染时 `setState` 是 React 给"记下前一次渲染的信息"的写法，放进 ref 则在渲染时读、写都不合它的规则（在渲染时写，丢弃的那次渲染会留下错的值）。代价是一万行多算一遍属性（约 40–80 ms），T2 191 ms、它的长任务 94 ms，都在界内。
- **535 与锁外的负载重叠的补量作废、重量**（主持者的消息；同 P4/S2 13:42–13:45 那一次）：`6600a2df` 的第一次补量（04:47–04:55）与 P4/S5 的一位审查者在锁外跑的一次约 45 秒、单核满载的 Node（约 04:50–04:54）重叠。读数都过界，结论不变，但要写进文档的数可能偏高：两轮作废，结果的目录留着、写明原因；05:51–05:59 重量，每 5 秒记负载与前三名进程；与作废的两轮相比各格至多差 7%，往两边都有。文档用重量的数。
- **536 总体设计 13.2 的新条取第 30 条**（S4 审查 C 的 Q1；决定 505）：P2/S3 没开工，S4 先合并。S4 设计第 8 节第 1、5 条与第 10 节、第 7 节"总体设计的修订"第 26 条照实写第 30 条；决定 501、505 的原文不动；P2/S3 合并时取第 31 条，在它自己的文档提交里改。
- **537 `e2e/perf/tree.perf.ts:26` 一行注释 134 字，不另加提交**（核对 C 的 C1-N1）：oxfmt 不重排注释，`check:format` 过；照旧，以后有人改这个文件时顺手重排。

### 2026-11-17：P4/S5 的决定 538–540（主持者的决定，负责人可以改判）

538–540 在 S5 的代码审查与修复里定。数在 [P4/S5 设计](04-P4-S5-editor-parse.md)第 10 节，理由与审查里的出处在 [S5 的审查记录](reviews/P4-S5-editor-parse-review.md)"主持者的决定"一节。实施中定的各条不编号，在审查记录里；找到的第十七种形状"一串空格直到正文末尾"进守卫，属决定 524（找到的形状进测试），也不另编号。

- **538 第 2 条之二过界时在测试里再量一次，界不放宽**（S5 审查 B 的 M2）：lezer 自己就平方的四格（P4/S5 设计 4.1 第 2 条之二）第一次过界时，这一格整格再量一次（轮流各七次、各取最快），两次都过界才红；界照旧 1.5、1.25、1.25、1.25 倍加 2 ms；失败的信息带两次的读数。不用 vitest 的 `retry`：它在第二次过了时丢掉第一次的读数，两次都失败时只报最后一次，做不到"带两次的读数"；它重跑整个测试，这里只需再量；"只在过界时多量"写在测试里，不靠配置。负载下一阵约 2 倍的争抢恰好从第一个"有扫描"的样本起时，比值约 2，一次就红；撤了修复的 x10、x11、x12 两次都过界（高出界 20–60%），不过界时用时不变。
- **539 注释与文档里 `*_` 写实测的范围，约 62–67 ms**（S5 审查 C 的重量）：实施者的再量两轮的较大中位数 62.4–63.6 ms，审查 C 的另一段 66.5–66.8 ms。`markdown.ts` 的 `inlineLimit` 注释、`markdown.test.ts`、总体设计 13.2 第 25 条与移交都这样写。
- **540 只改 `W/editor/` 的代码与测试的修复，推送之前免 `make e2e`**（修复第 1 轮）：`make build-web` 与 `make test-web` 照跑，逐个提交照跑 `make lint-web`、`make knip`、`make test-web`；分支 CI 跑全套。

### 2026-11-17：L1 的旁支的决定 541–547（主持者的决定，负责人可以改判）

541–547 在旁支 `m12-e2e-l1-new-tab`（M6 的故事，在 M12 里发现）的调查、实施、审查与修复里定。理由与审查里的出处在[旁支的审查记录](reviews/M12-e2e-l1-new-tab-review.md)"主持者的决定"一节；实施中定的不编号，在记录里。

- **541 e2e 的 L1 改测法、不改产品，取代 M11 第 277 条里 L1 的"照现状、CI 再红只重跑 e2e"：带修饰键的点击由 window 上最后的监听器核对，地址在故事自己开的标签页里打开；旁支照常审查、合并**（负责人 2026-11-17 第 7 问；旁支审查 A 的 L1A-Q1、C 的 L1C-Q1）。
  - 提交 `12e868bf`、`1b01b1d9`、`d716d9a9`、`5ea4ade0`，合进 `main` 一个 `56537688`，合并见第 12 节；审查与修复见[旁支的审查记录](reviews/M12-e2e-l1-new-tab-review.md)。开旁支、照常审查与合并，同 P1 的旁支（决定 356、359）。
  - **起因**：P4/S4 合并 `main` 的分支 CI（run 43979118856）第一次 attempt，e2e 只红在 L1 (page)：Ctrl 点链接之后 `context.waitForEvent("page")` 等满 30 秒。`ci.yml` 自 2026-10-18 起 400 次运行，L1 在第一次 attempt 红过 3 次（41188823890 在 `main` 上，42913206700，43979118856），重跑都绿；本机约 25 次一次（`89e2389a` 8/200，P2/S1 之前的 `7fdb9f58` 7/200，`41d0cfda` 9/200）。
  - **原因**（核实）：浏览器开了标签页、载入了页面；Chromium 没为它的文档发 `Page.frameNavigated`，且 `Page.getFrameTree` 答 url 为 `""` 的初始文档时，Playwright 1.63 等一个没有时限的承诺，不报这个页（旁支审查 B 的协议日志 200 次：红 13/13 合这两条，绿 0/187；M11/P2/S3 已查到同样的，见 277）。本机能读到的 Playwright 1.62.1 与 1.65 alpha 同样地等。同一修订的完整版 Chromium 200 次 0 红（决定 545）。"headless shell 里标签页的导航与 DevTools 自动接上之间的竞态"是推断。
  - **做法**：L1 在 window 上挂一个只用一次的冒泡监听器，排在应用的监听器（都在文章的容器上）之后，记下点击有没有被取消、带没带修饰键、链接的地址与有没有 `download`，然后取消它，浏览器不开标签页；断言没被取消、带着修饰键、地址是 `baseURL + targetPath`、不是下载；地址在 `anotherTab` 开的标签页里打开，看到标题、重新载入，到测试结束都安静。挂在捕获阶段就分不出取消了点击的应用（旁支审查 C 的对照）。
  - **丢掉的**：e2e 不再看浏览器为这次点击真的开了标签页，算作浏览器的事；对页面的链接，别处没有人守这一步（AS3 守的是 `target="_blank"` 的附件链接）。
  - **不采用的**：用 CDP（`newBrowserCDPSession` 加 `Target.setDiscoverTargets`）找浏览器开的那个标签页。它的应用正在启动、续期，这时关掉它可能丢掉续期（`e2e/fixtures/navigation.ts` 的文件头）；不关，它就在故事旁边一直跑着。
  - 277 里"向 Playwright 上游报问题（对外）"的一半仍是负责人的，不做。第 10 节"e2e 的偶发挡发布"一行的"其余的偶发"从此不含 L1。
- **542 L1 的测试名不改**（主持者的实施说明；旁支审查 C 核过）：名字说的是产品的行为，带修饰键时浏览器在新标签页打开，这没有变；测试证的是"应用把这次点击留给浏览器、地址是真的"，注释写明。
- **543 阅读视图的链接的单元测试在修饰键那一格记下每次点击的 `defaultPrevented`，随旁支改，不交给 P2 的打磨**（旁支审查 A 的 L1A-M1；`1b01b1d9`）：那一格原来把每次点击都取消、只看没有走路由，应用取消了带修饰键的点击也照过；L1 不再等浏览器开的标签页之后，"不取消"在 e2e 里只有 L1 的 Ctrl 一处守，Shift、Alt 哪里都没人守。改后前 13 次是 `false`、别处先取消的那一次是 `true`；A 的 mA、mB 与 C 的 c1 在 vitest 上由活变红。
- **544 05-lessons 第 6 节加一条，随旁支的文档提交**（主持者的实施说明）：Chromium 不总是把浏览器开出的标签页报给 Playwright，不等它，在点击那一刻核对。漏报的条件写成两条合起来，不提 lifecycle 的 `init`（旁支审查 B 的 L1B-M1：13 次红里 3 次有）；写明不能挂在捕获阶段（审查 C）；比例写成本机的（L1C-N3）。
- **545 完整版 Chromium 的对照写进记录，套件与 CI 不改用完整版**（旁支审查 B 的 L1B-Q1）：同一修订（`chromium-1243`）的完整版、new headless，在 `41d0cfda` 上跑原测试 200 次 0 红，同机同时段的 headless shell 9/200；它把"只在 headless shell 里"变成观察到的事实，竞态仍是推断。不改用：整个套件、CI 的安装与用时都要跟着变，改后的 L1 已不靠浏览器报告标签页。
- **546 第 12 节的审查链接与合并号放在 P4 一行**（旁支审查 C 的 L1C-Q2）：旁支不属哪个 Phase，是在 P4/S4 的分支 CI 上见到的；P1 的三条旁支同样放在 P1 一行。
- **547 修复核对之后不开第二轮；已推的提交说明不改写，不准处写在记录的"更正"**（同 `642f9196` 的先例）：第 1 轮三位都没有 Important、Minor；新的两条 Nit，注释的 L1B-N2 由只改注释的 `5ea4ade0` 改，`d716d9a9` 说明的"as theirs do"（L1C-K1-N1）与第一轮指出的 `12e868bf` 说明的不准（L1B-M1、N1，L1C-M1、N4）在"更正"。

### 2026-11-18：等锁帮手的旁支的决定 548–554（主持者的决定，负责人可以改判）

548–554 在旁支 `m12-lockwait-by-statement`（P3/S1 的测试，另有 M10 的一格）的查因、实施、审查与修复里定。理由与审查里的出处在[旁支的审查记录](reviews/P3-lockwait-by-statement-review.md)"主持者的决定"一节；实施中定的不编号，在记录里。

- **548 P3/S1 的 `TestTheLogReadsOneMomentAcrossARevert` 的偶发是测试的问题：开旁支修，照常审查与合并；测试按 sqlc 的语句名认出读停下的那一条（`pgtest.WaitForTableLockWaitsBy`），不加长等待**（主持者的查因说明与实施说明；旁支审查 A、B、C）。
  - 提交 `7b3ac149`、`3d61fc8e`，修复一轮 `aee229af`、`974c7fa2`，只改注释的 `5fb535a8`、`9f322c1d`，合并见第 12 节；审查与修复见[旁支的审查记录](reviews/P3-lockwait-by-statement-review.md)。开旁支、照常审查与合并，同 P1 与 L1 的旁支（决定 356、359、541）。
  - **起因**：旁支 `m12-submit-deadline-log` 的 `ad68ca6a` 的分支 CI（run 44078438315）第一次 attempt，bootstrap-7 红在 `MCP's_log` 一格：`R listed true; want the moment of its first read: neither`。那个提交只改 `wiki/app` 的测试，编译不到它；只重跑失败的作业，为绿。P3/S1 的审查 B 当时已疑到这里，没有复现，记为接受（[S1 的审查记录](reviews/P3-S1-small-correctness-review.md) `:60`）。
  - **原因**（核实）：原帮手 `pgtest.WaitForTableLockWaits` 数这个库里任何在等 `page_revisions` 的后端。服务起来之后几秒之内，River 跑 `RunOnStart` 的清理（`platform.purge_soft_deleted`，拿到 leader 之后 `StaggerStart` 先睡 0–1 秒），它 18 条语句里有 5 条也等这张表；测试数到它，就在读走到快照之前写下 R。本机改前没有负载 750 次红 3 次（`main` 450 次红 2 次，带探针的 300 次红 1 次），带负载 300 次 0 次；唯一带诊断的那一次，写 R 时等着的只有清理的 `PurgeTrimmedChangesets`，读还在快照之前的 `FindNotebook`。一个先等上锁的普通 `SELECT` 作外来者，改前查因者 100 格红 100 格、审查 B 红 98 格。CI 那一次与本机另两次也是清理，是推断。
  - **产品没有问题**：答复在一个快照里；把快照的隔离级别改成 READ COMMITTED，两个测试都红（审查 C）。
  - **做法**：`WaitForTableLockWaitsBy(t, pool, table, statement, n, limit)` 只数 `pg_stat_activity.query` 以 `-- name: <statement>` 开头、在等这张表的关系锁的后端，与原帮手共用一条查询；空的语句名立即失败。五格各写读停下的那一条：四格 `ChangesetFacts`，`log {path}` 一格 `ListChangesetsOfPage`；名字写成它之前或之后的一条，都在 10 秒的时限上红，信息带语句名。改后本机 450 次 0 红，外来者在场 100/100 格绿。
  - **不采用的**：加长等待。05-lessons 第 1 节：偶发的测试，等得更久治不好；这里要等的是对的那一个后端。
- **549 M10 的 `TestVirtualRead/rename_snapshot` 随同一条旁支改，等 `ChangesetFacts`；同类的别处不改**（主持者的实施说明"查同类"；旁支审查 B）：它的 team 也没等完清理，原帮手同样会数到外来者。它没红过：去掉过滤、外来者在场也 20/20 绿，靠的是改名（一次 MCP 调用）比读慢，不是靠等待。`asset_blobs` 的十处都经 `transferTeam`，持锁之前已等完启动的任务；`ingest_virtual_error_test.go` 已等完清理；迁移代价的测试与 linking 适配器的两处没有 River。别的等锁帮手也没有同样的问题（审查 B 核实）。这一个提交写 `(M10 story, found in M12)`。
- **550 另一种做法也写下：持锁之前先 `awaitJob` 等完服务启动时的任务**（旁支审查 B 的 LWB-Q1，采纳）：`transferTeam` 自 M7/P5A（`55ba1074`）起就这样做，P3/S1 的测试没照它做。`WaitForTableLockWaits` 的注释点名它（`aee229af`），05-lessons 那一条也写，免得读的人以为 `asset_blobs` 的十处也该改。
- **551 后缀：实施的第一个提交、改 pgtest 与日志的测试的修复与注释提交、文档的提交与合并都写 `(M12/P3/S1)`；改 virtual 那一格的写 `(M10 story, found in M12)`**（旁支审查 C 的 LWC-Q1）：照第 7 节"旁支"一条"写它所属的 Step 或 Phase"，被修的测试是 P3/S1 的，与已推的提交一致，不重写分支。C 倾向 `(M12/P3)`（帮手的旁支修已合并的 P1/S1 的帮手，写 Phase，`4fe6e523`），也合第 7 节。主持者的实施说明里建议的 `(M12/P3/S1 story, found in M12)` 不在规则里："story, found in" 只用于更早的 M。
- **552 第 12 节的审查链接与合并号放在 P3 一行**（旁支审查 C"文档的去处"）：被修的是 P3/S1 的测试；P1 的三条旁支在 P1 一行，L1 的旁支不属哪个 Phase、放在 P4 一行（决定 546）。
- **553 文档随旁支的文档提交改**（旁支审查 C"文档的去处"）：05-lessons 第 3 节"交错与锁"加一条"等锁的帮手要认出等的是谁"，放在"交错用确定的办法逼出来"之后（不放第 1 节：这不是测试能不能失败，是测试在好代码上会红）；总体设计 13.4 第 4 条加一句、第 15 节一行（先例 `8635667a`）；P3 设计 3.8.3 第 3 步与 P3/S1 的记录 `:60` 照决定 481 的做法加后注，不改写原文。M10 的文档、移交与 `06-environment.md` 不改。
- **554 修复核对之后不开第二轮；新的 Nit 由只改注释的两个提交改，与文档提交一起推送；已推的提交说明不改写，不准处写在记录的"更正"**（同决定 547 与 `642f9196` 的先例）：第 1 轮三位都没有 Important、Minor；新的 Nit 四条（去重三条），LWA-N3 与 LWC-N6 的测试注释由 `5fb535a8` 改，LWB-N2 = LWC-N5 由 `9f322c1d` 改。`7b3ac149` 说明的"most of its statements"（LWB-N1）与次数（LWC-N4）、`3d61fc8e` 与 `974c7fa2` 说明里"读还没开始"的一句（LWC-N5）、`aee229af` 的标题（LWC-N6）在"更正"。

### 2026-11-18：P5/S1 的决定 555–563（主持者的决定，负责人可以改判）

555–563 在 P5/S1 的实施、代码审查与修复里定。数在 [P5 设计](05-P5-deploy-release.md) 10.1，与设计不同之处在 P5 设计 3.24，理由与审查里的出处在 [S1 的审查记录](reviews/P5-S1-version-pipeline-review.md)"主持者的决定"一节。实施中定的各条不编号，在审查记录里；修复两轮里守卫与闸门测试的扩开并进 558、560。

- **555 BuildKit 钉 `moby/buildkit:v0.34.2@sha256:c5f8862f…`，不取那时才发 6 天的 v0.35.0**（S1 实施报告 7.2；审查 B、C 同意）：`buildx-stable-1` 那时仍指 v0.34.2，钉的就是 setup-buildx-action 不钉时会拉的那一个，不改变行为（决定 447 说取"S1 实施时的稳定版"）。摘要是 index 的，两种架构都有；两种 runner 都按它拉成功，审查 B 以 `docker buildx imagetools inspect` 核过。v0.35.0 以后照 4.3 的路升。
- **556 `actions/setup-go` 三份工作流照旧 v7.0.0，v7.1.0 一起升、走 4.3 的路**（S1 实施报告 7.3；审查 C 同意）：只升 `release.yml` 的 `gate` 会让闸门的 Go 环境与 `ci.yml` 的不同；`make outdated` 会一直列出它。
- **557 `make lint-actions` 的 `-format` 末尾写裸的 `\n`**（S1 实施报告 7.1；审查 C 对 actionlint v1.7.12 的源码核实）：actionlint 在解析模板之前先把 `\n` 等反斜杠转义换成字符，P5 设计 3.5 写的 `{{"\n"}}` 成了没结束的字符串。第 5 个提交因此在 CI 的 Lint workflows 一步上红在我们自己的 `make lint-actions`，不是设计预料的"现有的工作流过不了 actionlint"（审查 C 的 N5）；照设计的办法另起一个提交 `2db96b0a`，之后三份工作流 0 错，没有"the workflows pass actionlint"那个提交。
- **558 闸门与说明的测试补设计的表没盖住的格子，每格照 runner 检出；说明认的标题与闸门数的同一种**（S1 实施报告第 8 节与修复两轮；审查 B、C 同意）：闸门加四格（同一提交另一个分支、事件、提交的更新的绿运行，缺右括号的标题），"没有 IMAGE"要闸门自己的字样，说明加结尾的空格行与行中的标题（`38916978`；没有一格能让一条检查单独变红就等于没测，05-lessons 第 1 节）；每格照 `actions/checkout` 检出 `GITHUB_SHA` 的提交，没设时检出 tag 的（审查 B 的 M2、N7，C 的 M2）；说明的一节只从以"）"结尾的标题起，"下一个 `## `"先判，同一个版本写坏了的标题在前不是起点、在后结束这一节（审查 B 的 N1、N5）。
- **559 README 的"需要安装"写"bash、git、jq 与 awk"**（S1 实施报告 7.5；审查 C 建议给号）：P5 设计写"加 jq"；发布脚本的测试同样要 bash、git 与 awk，四样都列出来，06-environment 同此。
- **560 守卫加两条、不加一条，加的两条照核对第 1 轮扩开**（S1 审查 B 的 N3；核对第 1 轮 B 的 M3、N6，C 的 N7）：
  - 加：`release.yml` 里跑 image-smoke 的步骤不写 `if`；`rehearse` 与 `build`（S2 加 `verify`）各恰有一步跑它，`run` 里有一行 `deploy/image-smoke.sh … "$VERSION"`、后面没有别的。
  - 加：演练不引用 secret、不登录：`rehearse` 作业与工作流顶层的 `env`、`defaults` 里没有字符串匹配 `(?i)\bsecrets\b`，演练每一步的 `uses` 与 `run` 合起来不匹配 `(?i)login`。演练推不出东西靠令牌只有 `contents: read`，一个别人的令牌加一次登录就越过它。
  - 不加："tag 的运行不取消"。取消在 `build` 之前时什么也没推；在它之后，GHCR 里留下没有 tag 的摘要，同失败时（决定 444）；版本号的 tag 在证明与签名之后才打、Release 核对过才公开（P5/S2），被取消的运行留不下按版本号拉得到的东西。
  - 同一守卫照审查的 Minor、Nit 改的几处（`run:` 里在每一处 `${{` 上核对，矩阵的值写死，`ci.yml` 的 actionlint 一步恰是三个键、`costs-1` 是作业的一份）是发现的处置，不另编号。
- **561 `build` 断言它检出的就是闸门放过的提交，交 P5/S2**（S1 审查 B 未核实的第 1 条；措辞照核对第 2 轮 B 的 N11）：`build` 用 `HEAD` 构建、不看闸门的 `sha`，两个作业是同一个提交是推断，只有推 tag 才证明得了。S2 在 `build` 的 Labels 之前加 `test "$(git rev-parse HEAD)" = "$SHA"`；`publish` 不用 `HEAD` 构建，它的 Index 一步可以另核对推上去的镜像的 `revision`。S1 不改：要改 `release.yml`，与 S2 的 `publish`、`verify` 一起写，tag 的作业在 S2 合并之前走不到（P5 设计第 9 节"P5/S2"）。
- **562 smoke 一步关掉 `bash -e` 的写法交 P5/S2**（核对第 2 轮 A 的 N6、B 的 N10、C 的 N9，三位都提）：S1 的守卫按行核对 smoke 那一行，前面一行 `set +e`、这一步或作业写不带 `-e` 的 `shell` 时，smoke 失败了这一步照绿。S2 写 `publish`、`verify` 与它们的 `shell` 时，守卫一并不许跑 smoke 的步骤与作业写 `shell`、`defaults.run.shell`、`set +e`。理由：S1 的守卫防的是无心的改名与挪动（同审查 C 的 N6 的定级）；有意的写法列不尽，要与 S2 加的 `shell` 一起看。主持者先定"不改、只记"，看了 A 的 N6（`build` 那一步自带第二行）改判为交 S2（P5 设计第 9 节"P5/S2"、3.10）。
- **563 S1 审查 B 的 N2、C 的 N1 不改代码**（S1 实施报告"修复第 1 轮"；核对第 1 轮判理由站得住）：闸门的日期只核对 `YYYY-MM-DD` 的形状（`2026-99-99` 放行）：它挡的是忘了写日期（决定 434），日期由主持者在 P5 设计 3.23 第 7 步照当天写，P5 设计 3.6 写明；MCP 的版本只测到 `mcpOptions`，`mcpServer` 只剩一行 `mcpserver.New(mcpOptions(...))`，这一跳由代码的形状守着、不由测试守着（决定 431），写进审查记录。

### 2026-11-18：提交期限的旁支的决定 564–571（主持者的决定，负责人可以改判）

564–571 在旁支 `m12-submit-deadline-log`（M11 的故事，在 M12 里发现）的查因、实施、审查与修复里定。理由与审查里的出处在[旁支的审查记录](reviews/M12-submit-deadline-log-review.md)"主持者的决定"一节；实施中定的不编号，在记录里。

- **564 M11 的 `TestTheSubmissionsDeadline` 的偶发是测试的问题：开旁支修，改测试、不改产品，替身照生产的先后读钟；旁支照常审查、合并**（主持者的查因与实施说明；旁支审查 A 核实生产没有错，B 核实原因）。
  - 提交 `ad68ca6a`、`203fc064`，修复第 1 轮 `1f94312b`、`75c38196`，第 2 轮 `9845aa7e`；合并见第 12 节。开旁支、照常审查与合并，同决定 356、359、541、548。
  - **起因**：P4/S5 合并之后第一次 `main`（`5e58e844`，run 44022740779）第一次 attempt，`server (short)` 红在 `TestTheSubmissionsDeadline/the_request_expired`：告警行的 `timeout_ms` 是 53，测试要至多 50。只重跑失败的作业，第二次 attempt 全绿。
  - **原因**（核实）：`timeout_ms` 是期限减提交的时刻（`submit_upgrade.go:104-107`、`:131`）。请求先到期的格，期限是 `WithTimeout` 读的真实时钟加 50 ms；时刻来自假时钟，`newSubmitFake` 造替身时就读了，早于 `WithTimeout`。所以记下的是 50 ms 加两次取时的间隔，向下取整。失败日志里的 UUIDv7 与记下的 53 定出那一次两次取时相隔 3.00–3.32 ms，停在 `newCheckFake` 造完 id 之后、`newSubmit` 造 actor 之前（旁支审查 B）。改前在 `WithTimeout` 之前睡 2 ms，请求先到期的四格 20/20 红；不加探针 200 次 0 红。停顿的原因（GC、调度或虚拟机被抢）是推断。
  - **生产没有错**（审查 A 沿代码核实，没有跑整个程序）：API 的 `deadline` 中间件与 MCP 的 `outer` 在用例之前定好请求的期限，第一个事务之后才读 `clock.System`，`timeout_ms` 不超过请求的超时。
  - 不把间隔算进上界：那是在生产不会有的先后下认可 53 这样的值（实施中定）。不加长任何超时。
- **565 告警行的断言随旁支补成精确的：`timeout_ms` 是测试自己的时刻到期限的精确值，行的时刻须等于它，`elapsed_ms` 也精确；请求有期限的格另断言钟自己的读数离期限不超过请求的超时**（实施者报的 M5；旁支审查 A 的 SDA-N1、N2，C 的 SDC-M1、N1；修复核对第 1 轮 A 的 SDA-M1）。
  - 理由：原来的界只要至多 50，记 0（M5）、少记 1 ms、向上取整都照过；`elapsed_ms` 不断言，记 0 也照过。两处都是 M11/P2/S2 起就有的缺口，与偶发同一行、同一种，随旁支补，不另开。
  - `203fc064` 改为精确值。`75c38196`：期望值改用测试自己的 `f.at()`（由产品写的 `SubmittedAt` 算时，挪动时刻的变异在本测试里活），`elapsed_ms` 须等于 `tc.elapsed`，并加回"不超过请求的超时"这个生产的性质，让读钟的先后被改回时看得见。按整毫秒比，改回之后还要停 1 ms 以上才红，又是原来的偶发（SDA-M1）；`9845aa7e` 改为比较钟自己截到微秒之前、带单调读数的读数：改回、中间不停，四格 20/20 红；墙上时钟往回跳也不误红。
- **566 两个循环都在 `Execute` 的前一行读假时钟**（旁支审查 B 的 SDB-M1，取 B 的第 1 种改法；`1f94312b`）：COMMIT 两格 50 ms 的 `SubmitTimeout` 也是真实的 ctx 期限，起点同样是早读的钟，改前在 `Execute` 之前睡 60 ms，两格 5/5 红。`ad68ca6a` 那四格读钟的位置也挪到这里，写法统一，结果不变。B 的另两种（只补这两格；不改、只写明余量）不取。
- **567 读钟之后、`Execute` 走到相应的一步之前停 50 ms 以上照样红，不改**（旁支审查 C 的 SDC-N3，修复核对第 1 轮 B 的 SDB-N4 = C 的 SDC-N5）：请求先到期的四格答 `context deadline exceeded`，COMMIT 两格答 `server_busy`。这是 50 ms 期限本身的余量，改前就有，要停的是 CI 那次停顿的 15 倍；以后在这两处见到这两种答复，不是这个偶发回来了。写在记录的"接受与推后的"。
- **568 分支 CI 的两次红都与本支无关，只重跑失败的项：bootstrap-7 那一次由等锁帮手的旁支修（决定 548）；costs-3 那一次记成观察的偶发，M12 收尾之前再出现一次就开旁支查**（旁支审查 C 的 SDC-M2 与"costs-3 那一次红记在哪里"）。
  - `ad68ca6a` 的 run 44078438315 第一次 attempt，`server (bootstrap-7)` 红在 P3/S1 的 `TestTheLogReadsOneMomentAcrossARevert/MCP's_log`；本支编译不到它；只重跑失败的作业，为绿；原因由旁支 `m12-lockwait-by-statement` 查清、修（决定 548）。
  - `203fc064` 的 run 44163179005 第一次 attempt，`server (costs-3)` 在 AMD EPYC 9V74 上红在 M8 的 `TestTrashHistoryAtItsBoundCountsInTimeWithForcedPlans/no_pg_stats/force_generic_plan`（`trash_history_forced_cost_test.go:200`）：40 万行的计数 2.72 秒，界 1 秒；同一测试的另三格 0.25–0.30 秒。本支只改 `wiki/app` 的测试，`page/adapter/postgres` 编译不到；`ad68ca6a` 的运行里 costs-3 绿。
  - 照决定 360 的先例只重跑失败的项（与被跳过的 `e2e`、`image`），第二次 attempt 在 Intel Xeon 6973P-C 上 19 项全绿。主持者查了最近 100 次失败的 `ci.yml`，没有这个测试；文档里也没有它偶发的记录。原因没查，推断是计划或停顿。
  - 理由：只出现一次，在代价守卫里，不在本支改的路径上；照 360，偶发只重跑失败的项，开旁支的代价大于一次重跑。收尾记录核对 `main` 的红时（决定 334），一并查这个测试在 `main` 与各分支 CI 上有没有再红。
- **569 第 12 节的审查链接与合并号放在 P4 一行**（旁支审查 C 的 SDC-M2）：旁支不属哪个 Phase，起因在 P4/S5 合并之后的 `main` 上见到，同决定 546 的写法。
- **570 05-lessons 加两条，随旁支的文档提交；M11 的文档与总体设计不改**（旁支审查 C"文档的去处"；B 的 SDB-M2、N3）：
  - 第 2 节"替身"一条：一个值里混着替身的时钟与真实的期限时，断言精确值、照生产的先后取时、再断言先后本身，附 grep 的 `WithDeadline` 写法。不放第 1 节：那里已有"每个新的断言都要证明它会失败"，M5 是它的又一例，新的是替身取时的先后。
  - 第 7 节"CI"一条：失败日志里的 UUIDv7 读得出现场的时间，要配合记下的值；挨着"读重跑过的运行，要指明 attempt"，都是读失败现场的办法。
  - M11 的 P2 设计写的"假的时钟以 `time.Now()` 起算"仍成立，P2/S2 的记录是当时的记录；总体设计第 13 节的约定没变，没有新的帮手（先例 357、541 也没改）。理由写在记录里。
- **571 修复核对第 2 轮只请 A，之后不开第三轮；已推的提交说明不改写，不准与细化写在记录的"更正"**（同决定 547、554 与 `642f9196` 的先例）：核对第 1 轮只有 A 的 SDA-M1 一条 Minor 没关，B、C 没有未关的 Minor；第 2 轮 A 没有新的发现。`ad68ca6a`、`203fc064`、`1f94312b` 说明里不准的几句（SDA-N2，SDB-N1，SDC-N2、N4）在"更正"；"Logging SubmitTimeout fails"一句记成细化，不算不准（SDC-Q1）。

### 2026-11-18：P4/S1 的决定 572–579（主持者的决定，负责人可以改判）

572–579 在 P4/S1 的实施、代码审查与修复里定。数在 [P4 设计](04-P4-performance.md) 10.1，与设计不同之处在 P4 设计 3.10，理由与审查里的出处在 [S1 的审查记录](reviews/P4-S1-perf-tools-review.md)"主持者的决定"一节。实施中定的各条接受，列在记录的"实施中定的"一节与 P4 设计 3.10，不逐条编号（578）。

- **572 重类的四个词照设计，不提高目标，交 S3 判读**（S1 审查 A 的 Q1；份数照核对第 1 轮 A 的 N3）：提高目标就改了 B1 与 M8 的同口径，还要挪动所有页的补放，收益不确定。`中国`、`ai`、`kernel` 自然只在约 0.5%、1.9%、1.3% 的页，补放到 5% 的下限；`linux` 自然约 5.3%，一页也没补放；四个都只在约 5% 的页，不是真语料里"最常见"的键。S3 量 B1 时列出它们的份数与 M8 记下的最大（719、827.2 ms），判第 1 步时考虑这几格偏轻（P4 设计 3.6 的 B1）。
- **573 采样逐行另记 cgroup 的 `anon`、`shmem`、`file`；判界的口径不在 S1 改**（S1 审查 B 的 Q1；核对第 1 轮 B 的 N7）：PostgreSQL 的 `memory.peak` 多是页缓存（一轮 2.19 GB 里 `file` 2.04 GB）。`file` 含 `shmem`：收得回的页缓存约是 `file − shmem`，收不回的是 `anon + shmem`，三样不能相加。S3 照 P4 设计判 B2、B3，三样都写进记录。
- **574 删掉语料生成器里两处到不了的核对（own-j、own-k）**（S1 审查 C 的 Q1）：重抽在词一层，段落一层与一串词末尾的核对在任何种子上都到不了，留着让设计与变异表以为重抽在段落一层；删了之后随机流不变，sha256 逐字相同。P4 设计 3.2、决定 456 的复述与 7.2 第 5 行照实改（3.10 第 1 条）。
- **575 驱动到期只停发，在途的答完再汇总，排空至多 60 秒、不加 `-drain`；截断与失败分开数**（S1 审查 B 的 M1、M2，C 对实施报告小决定第 4 条的建议；核对第 1 轮 C 的 M5、B 的 N6）：到期取消在途的请求，被截的是更长的那些，另数也压低分位（审查 B 的病态一轮截了 12%）。每个循环发每个请求之前看期限（核对第 1 轮 B 的 M7），编辑者只关会话；排空 `drive.DefaultDrain` 60 秒，是 serve 请求期限 15 秒的四倍，命令行不可改，没有要调的场合；汇总分开数 `cut` 与 `errors`，有 `errors` 时命令失败。
- **576 手跑的规模开关各有下限：`NW_SCALE_PAGES` 至少 20,000，`NW_SCALE_CHANGESETS` 至少 100,000、5 的倍数；更小就报错**（核对第 1 轮 C 的 M6，取它的 (b)；S1 审查 A 的 M1，实施者取它的第一种改法）：两处都是开关收下了守卫跑不了的规模，红在正确性的断言上、不在界上。lint 的孤页过 10,003 页才封顶在 10,000、发起的 1,000 个 wanted 不随规模，2 万页是设计的试跑量过、四个守卫都过的规模；日志测试"rare client"一格要 CLI 的变更集 ⌊n/1000⌋ 不少于 100。开关只为往大里量，给下限比让断言随规模推简单（P4 设计 3.3、3.10 第 3 条）。
- **577 4.7.4 的两句照 S1 之后的实情改；第 7 节 P4/S1 一行"7763 上推断约 15 秒"不改**（S1 审查 C 的 M3、M1，核对第 1 轮 C 的 N7）：语料的三个测试真的跑（两个不连库的也在 `short`），`server/tools/perf` 的单元测试在 `costs-3`，驱动不进 CI。分支 CI 的 `costs-3` 三次落在 EPYC 9V45、Xeon Platinum 8370C、8573C 上，都不是 7763，12.2 秒不是推断的那个数；合并之后两次 `main` 的数照机型记在 S1 的审查记录。
- **578 实施中定的各条都接受**（S1 实施报告第 7、8 节与修复两轮；三位审查者各在自己的范围里判过）：列在 S1 的审查记录"实施中定的"一节与 P4 设计 3.10，不逐条编号，同决定 516。S3 要知道的：vault 的附件定长、每个最深的文件夹至少一页；`perf prepare` 的 PAT 遇 429 每秒重试、至多 60 次；`stack.sh` 一轮一个目录，`--env` 写进 `serve.env`。实施报告的"导出经签名地址下载"是实现了设计（审查 B 的 N2），不算决定。
- **579 两处 Nit 不改；核对第 2 轮只请 B、C，之后不开第三轮；已推的提交说明不改写，不准处写在记录的"更正"**（同决定 547、554）：`perf prepare` 遇 429 不读 `Retry-After`（审查 C 的 N4：桶每 0.1 秒补一个，固定 1 秒不短于它）；`EarlyBusy` 用 `ToHeader` 照绿（审查 B 的 N4 第 3 点：503 的体很小，差不到 1 ms，判的是 2.0 秒）。核对第 1 轮之后 Minor 都在 B、C，A 的两条 Nit 只关草稿，由 C 代看。第 2 轮只有 Nit：B 的 N10 = C 的 N10 由实施者加一个只加测试的提交 `2153782c`，与文档提交一起推送；C 的 N9 在文档提交里；C 的 N11（`9ef5fa2b` 说明的一句）与 `f098cffd` 说明的一句在"更正"。

### 2026-11-18：导入守卫的旁支的决定 580–583（主持者的决定，负责人可以改判）

580–583 在旁支 `m12-ingest-request-deadline`（M10 的故事，在 M12 里发现）的查因、实施、审查与修复里定。理由与审查里的出处在[旁支的审查记录](reviews/M12-ingest-request-deadline-review.md)"主持者的决定"一节；实施中定的不编号，在记录里。

- **580 `5154e0de` 的 `main` 上，guards-sources 红在 M10 的 `TestIngestOfMoreThanBatchSourcesTakesItsTime`：只重跑失败的项；开旁支修，照决定 359 的做法：401 个来源的单个 REST 改名改在生产的 15 秒请求期限下跑，它的界从 4 秒（请求期限）换成 11 秒，由 11 秒判整个请求；产品不改，照常审查、合并**（run 44232764121；同决定 356、359、541、548、564）。
  - 提交 `8fd158fd`，修复第 1 轮 `a1b7b7e5`；合并见第 12 节。
  - **起因**（核实）：第一次 attempt 在 AMD EPYC 9V45、4 CPU 上，`single_rename_401_sources/0` 4.006 秒答 500，给 401 页重建索引做到第 42 页时撞上测试配置的 4 秒期限。make 停在这一行。只重跑失败的作业，第二次 attempt（Xeon 6975P-C）为绿，1.55–1.57 秒。
  - **定因**（核实）：
    - 2026-11-05 起 guards-sources 的 247–250 个作业，这一格从没贴近 4 秒：7763 上 64 次最慢的样本 2.66–2.99 秒，是 4 秒的 67–75%；9V45 上另 31 次 1.35–1.62 秒。对照决定 359 的撤销，一直只比 4 秒少 3–18%。
    - 样本多的四种机型（7763、9V74、9V45、8573C）在 S2 前后差不到 0.02 秒，看不出与代码有关的台阶。
    - 失败那一作业整份都慢：跑了的四个导入格是 9V45 中位的 1.7–3.2 倍，Go 进程的 CPU 时间也多 1.6–2.7 倍；其余作业里这四格至多 1.37 倍。
    - 再前一次 `main`（run 44224495963，`f08dca69`）的 guards-core 也在 9V45 上、也慢约 2 倍：11-18 02:36–04:01 UTC 之间，9V45 上两个作业慢 2–3 倍；同一份产品代码前一天在 9V45 上正常，`5154e0de` 与上一次绿的 `b26a8dbd` 在 `server/` 上相同。
  - **推断**：那段时间的 9V45 慢了，不是产品；照它的速度，整次改名约 5–6.5 秒。
  - **结构上的毛病**（核实）：M10/P4 设计这一格时，由测试的 4 秒请求期限判（[04-P4](../M10-llm-wiki/04-P4-virtual-ingest.md) 第 215 行："以现有request和parser界验证/记录实际可达规模，不提高任何限制"）；`6d1549fc` 把批一支的字面 11 秒做成参数之后，传给这一格的 11 秒是死参数（REST 改名不记单元，`unit_ms` 只在 `apply_batch.go:168` 写）。越过期限只答 500，记不下用时。
  - **改法**：
    - 请求 15 秒、写 60 秒；读仍是测试配置的 5 秒（`ingestCostServerBudget` 只设请求与写），不照抄决定 359 的"读 30 秒"；提交 2 秒不变。
    - 11 秒判整个计时的请求，比单元的时间严。
    - 期限由界推出：4、15、30 秒里比界大的最小一个（30 秒是 M10 决定 150 的两个稠密形状）；在起应用之前与形状给的期限对证，起来之后再与应用实际的期限对证；五个额外形状都经它（582）。
    - 计时的改名用 30 秒的客户端：`client()` 5 秒就放弃，会先于界来判。
    - 夹具、样本与别的判定不变（负责人 2026-11-06 的话）。
  - **预算**：归 4.7.1 的 B4（每个请求 15 秒），不归 B5（审查 B）。这是 401 个来源、各 300 条链接的改名，改写 401 页，远轻于决定 461 算作病态的"被几千页链着的页的改名"（M6 量过 2,500 页 10.3 秒）。7763 上实数最慢 2.99 秒；照这次的倍数推到 7763 上约 8.1–9.6 秒；加 `parse_max_wait` 的 2 秒都在 15 秒之下。改后 11 + 2 = 13 秒，仍守着 15 秒。本机单跑 1.6 秒。
  - **守卫少证了的**：不再证明这一格在测试配置的 4 秒之内完成。本机睡 3 秒，改前 4.0 秒答 500、改后 4.58–4.69 秒照过；睡 10.5 秒，改后 12.24 秒红在界上。
  - **同类**（审查 B 逐格核过）：只有这一格。界等于期限的几格（十万页那一格、lint 的两格、MCP 预览）真等于，不改；`virtual-export-whole` 的 5.74 秒量的是后台任务加轮询与下载，没有一个请求承担它，不在此类。决定 359 的旁支当时没有查同类，这一格那时（`ad0bf40c`）已在同一份 guards-sources 里，4 秒对 11 秒（审查 C）。
- **581 这一格的界取 11 秒**（审查 B 的 IDB-Q1）：与同规模的批形状（`unreferenced_old_f`，400 页，15 秒对 11 秒）一致；慢的 7763 照这次的倍数（推算 8.1–9.6 秒）也过得了。11 秒不是照 13.4 第 8 条从这一格量的三倍（本机三倍约 5 秒），是借来的，照实写进 13.4 第 8 条。
  - **可以改判**：约 6 秒（7763 最慢的 2 倍）。好处是 3 秒级的变慢仍抓得到（本机 1.6 + 3 秒）；代价是照这次的推断（5–6.5 秒），那台慢的 9V45 仍会红，而且是界上的红，要再开一次旁支。
- **582 删除组那一格也由界推出期限、与起应用的期限对证；两个正式的 batch 形状不动**（审查 A 的 IDA-M1、IDA-Q1）：
  - `a1b7b7e5`：一个界变量同时给 `unit_ms` 的判定，不再写字面的 11000；期限给 4 或 30 秒都在起应用之前红。
  - `old_f_120000`（15 秒对 11 秒）与 `maximum_old_f_hundred_k_100_ops`（30 秒对 29 秒）也是"期限与界都由调用者给"，不在 4 秒下，不属同类，本支不碰；以后改这两个形状时一并接上。
- **583 文档的去处与收口**（审查 C 的 IDC-Q1、Q2 与第 4 节）：
  - 13.4 第 8 条的例外加这一格，并加半句：batch 的形状一向在 batch 的 bench 的 15 秒下，不算例外，这一格是其中唯一的 REST 改名；第 15 节加一行；移交 `M4-performance.md` 的例外加 580。
  - 第 12 节的链接与合并号放在 P4 一行（同决定 546、569）；记录叫 `reviews/M12-ingest-request-deadline-review.md`。
  - 05-lessons 第 1 节决定 359 那一条下加两个子条（死参数的反向对照；修了一格之后查同类），不另立泛泛的一条。M10 的文档不改（同决定 359、570）。
  - 修复核对第 1 轮三位都没有 Important、Minor（B、C 各 2 条 Nit，在记录里写对），不开第二轮。已推的 `8fd158fd`、`a1b7b7e5` 的说明不改写，不准处写在 `a1b7b7e5` 说明的"Corrections"与记录的"更正"（同决定 547、554、571）。

### 2026-11-18：P2/S2 的决定 584–588（主持者的决定，负责人可以改判）

584–588 在 S2 的审查与修复里定。每条在 [P2 设计](02-P2-web-polish.md) 10.2，理由与审查里的出处在 [S2 的审查记录](reviews/P2-S2-text-review.md)"主持者的决定"一节；实施中定的不编号，在记录里。

- **584 `page.nameAtTop` 不带笔记本名："{name} (at the notebook's top level)"、"{name}（笔记本的顶层）"**（S2 审查 A 的 Q1；P2 设计 9.5、8.2 的 S2）：与回收站、转移、索引、历史、移动对话框是同一个说法（决定 386）；同一本笔记本里笔记本名不区分什么，带上它反而可能读成同名那一页的顶层。取代决定 428 的前一半（名字里带笔记本名）；后一半（不用笔记本名的所有格）随之没有对象。`distinctName` 去掉 `notebook` 参数，十六处调用随之改（`1580e7ca`）；总体设计 13.2 第 17 条的示例写"Install（笔记本的顶层）"。
- **585 替身的任务项照服务端的解析**（S2 审查 B 的 M1、C 的 N6；P2 设计 6.1）：只有冒号之后是单个目的地、其后只有空白的行是定义形，没勾的勾上答 422，勾着的不是任务项；别的 `]:` 行照普通任务项（`a1138dd7`）。实施者"`- [x]: /u` 不是任务项"的推断由审查 B 在服务端上核实，只在这一形状上成立。`- [ ]: a)b` 替身答 422、服务端照写，接受，代码不改（核对第 1 轮 B 的 C1-N1）。
- **586 实施门禁里 `TestLintBusyReturnsBeforeCleanup` 那一次红不是 S2 的，也不记成已知偶发；另开旁支 `m12-lintbusy-await-expiry`（M10 的故事）修，合并见第 12 节**（S2 审查 C 的 Q1）：M10 的测试等到 `server_busy` 与 `pg_sleep` 被取消就断言新建一次运行，而这一次运行的过期由 `context.AfterFunc` 在另一个 goroutine 里做，两件事之间没有先后；探针在 `expire` 开头睡 300 ms 时确定地红，测试再等 `RunExpired` 就过。
- **587 推后进 v0.2 的**（S2 审查 A 的 Q2、N5；P2 设计 4.2）：撤销被拒为 trimmed 时两句近乎重复；`AGENTS` 模板写的 prompt 与 MCP 实际登记的对照（测试改名为 `TestTheAGENTSTemplatesNameTheUpgradePrompt`，`62e40b25`）；设计已定的照旧（15.3 的 T8，22.16、23.4 照决定 390，23.1 服务端的英文照决定 387、其中"…reverted whole"P3 或 v0.2 改，H4 的功能）。
- **588 文档的去处与收口**（核对第 1 轮 A 的 C1-N1 = C 的 C1-N2，S2 审查 C 可选的一条）：P2 设计正文里仍写旧形式的各处照实改、注"实际见 10.2"，决定 428、388 只加注；记录叫 `reviews/P2-S2-text-review.md`，读屏清单在记录里，P5 设计 3.19 加链接；05-lessons 加两条（替身也不能拒真东西收下的；改了文字，找全旧断言、每种改了的语言都有格）；总体设计 3.6 的"5 MB"改成 5 MiB；核对第 1 轮之后不开第二轮，已推的提交说明不改写，不准处写在记录的"更正"（同决定 547、554、571、579、583）；P2/S2 先于 P2/S3 合并，S3 照决定 378 后合并。

### 2026-11-18：P2/S3 的决定 589–594（主持者的决定，负责人可以改判）

589–594 在 S3 的实施、审查与修复里定。每条在 [P2 设计](02-P2-web-polish.md) 10.3，理由与审查里的出处在 [S3 的审查记录](reviews/P2-S3-narrow-bdi-contrast-review.md)"主持者的决定"一节；实施中定的不编号，在记录里。

- **589 33.1 复现了，按 B 在 S3 修：`NativeSelect` 的基类加 `contain-paint`，不用 4.3.1.1 的办法**（P2 设计 4.3 的 33.1；S3 审查 B）：没改的 `f08dca69` 在 Playwright 1.63 的 WebKit 26.6 里，SE1 (page) 的 375 一段页宽 704，三遍都是，Chromium 375；WebKit 把选择显示的那一项的文字整个算进页面的宽度。对选择写 `overflow` 在 WebKit 里不生效，外层的 `min-w-0` 从头布局也不管用；外层 `overflow: hidden` 管用但裁掉选择的焦点环（审查 B 核实）。原因在组件，所以放在基类上；它不裁选择自己的环与边框（`72b1e679`）。SE1 拆出 `@narrow` 的一格（决定 413）；附录 A 的 33.1 仍在 M 类，注明复现与修好，真的 Safari 上发布之后看（决定 590）。
- **590 WebKit 原生下拉没有 3 px 的环，按 C 进 v0.2；真的 Safari 上的两项进 P5/S3 的人工清单**（S3 审查 B 的 N1、Q1；4.5）：Playwright 的 WebKit 不画原生下拉的 `box-shadow`，聚焦时只有 1 px 的边框变成 `#737373`，对页 4.74，满足 AA；改前就是，不是 S3 引入的。v0.2 的改法已核（`select:focus-visible` 的 2 px 外框，同时去掉 `ring-[3px]`）。人工清单（真的 Safari，发布之后）：窄屏的搜索页（33.1）；原生下拉的焦点。
- **591 暗色 destructive 按钮悬停的对比度在 S3 修**（S3 审查 B 的 M1）：悬停的底回到 destructive 的 90%，白字 3.45；改前就有，属于 12.4 的对比度，小、可修。`dark:hover:bg-destructive/50`，3.45 → 8.07，亮色 4.77、4.54 不变；PG4 (page) 钉住两种主题的静止与悬停（`3c4d5710`）。
- **592 钉住 Playwright 项目的检查**（S3 审查 C 的 M1、N1、N2；P2 设计 5.1，决定 421）：`tools/e2e-projects/check.mjs` 的 `REQUIRED` 在 MC3、PG2 之外列 SE1 那一格：丢了标签的格只有它看得见，SE1 那一格是 `contain-paint` 唯一的守卫（`1d8a2fd9`）；经 `pnpm --filter @nervewiki/e2e exec playwright` 列格，不用 knip 的 `ignoreBinaries`；检查看的是各项目列出的文件与格，看不见引擎，注释与报错照此写（`63004369`）。
- **593 13.2 的新条是第 31 条**（S3 审查 C 的 M2）：照决定 505、536，P4/S4 先合并、取了第 30 条，P2/S3 接在它之后；决定 396 与第 7 节"总体设计的修订"第 21 条照此注明。
- **594 文档的去处与收口**（核对第 1 轮 C 的 C1-N1，S3 审查 A 的 N2、Q1）：P2 设计正文里与实施不同的各处照实改、注"实际…（10.3）"，决定 396、413 只加注；记录叫 `reviews/P2-S3-narrow-bdi-contrast-review.md`，真的 Safari 上的两项写在记录里，P5 设计 3.19 的人工入口加链接；05-lessons 加两条（量计算之后的样式之前先结束元素与它祖先的过渡；WebKit 把原生选择显示的文字算进页宽）；设计 32.4 举的两个乱序的例子三种引擎里都不乱，照实写；已推的提交说明不改写，不准处写在记录的"更正"（同决定 547、554、571、579、583、588）；S3 照决定 378 后合并 `main`，`history-filters.tsx` 的冲突两边都留；核对第 1 轮之后不开第二轮。

### 2026-11-18：lint 忙的测试的旁支的决定 595–597（主持者的决定，负责人可以改判）

595–597 在旁支 `m12-lintbusy-await-expiry`（M10 的故事，在 M12 里发现）的实施、审查与修复里定；开旁支、不记成已知偶发由决定 586 定。理由与审查里的出处在[旁支的审查记录](reviews/M12-lintbusy-await-expiry-review.md)"主持者的决定"一节；实施中定的不编号，在记录里。

- **595 M10 的 `TestLintBusyReturnsBeforeCleanup` 改测试、不改产品：等被认定的那件事自己的事件，不等旁证；不加长等待；照常审查、合并**（P2/S2 审查 C 的 P2S2C-Q1；旁支审查 A 的 LBA-M1、B 的 LBB-M2；同决定 356、359、541、548、564、580）。
  - 提交 `7abfd6ef`，修复第 1 轮 `d8e664b3`、`9079f9af`；合并见第 12 节。
  - **原因**（核实）：过期由 `context.AfterFunc` 在另一个 goroutine 里忘掉 flight（`wiki/app/lint_runs.go:98`、`:186-197`），测试等到 `pg_sleep` 被取消就让新的同键调用者发起，两者没有先后；`expire` 开头睡 300 ms，测试确定地红，信息与那一次一字不差。那一次就是 `expire` 排晚，是推断。
  - **改法**：新的同键调用者发起之前等旧 flight 的 `RunExpired`；新 flight 的跟随者发起之前等旧 worker 的 `RunFinished`（S61 第三种变异原来靠"新 flight 到了 Prepare"这个旁证，`finish` 慢时活下来）；末尾的连接数限时轮询（被取消的连接由 puddle 在另一个 goroutine 里销毁）。
  - **对产品**：deadline 与 `expire` 之间来的调用者多一次 503，与 `Runs` 的注释相符。窗口平常在 0.1 ms 之内；测试从取消到新的调用者被收下要 1.3–5 ms（审查 A、C 量的）。照推断，那一次在负载下到了毫秒级。
- **596 transfer 心跳的取消是同一类，查明、不改，记成潜在的先后问题，不是已知偶发**（旁支审查 B 的 LBB-M1，取它的 (b)）：`cancelSeen`（7 处调用）与取消运行中的导入，等到心跳写下读到取消的一行，就当作作业已 `stop`；心跳先写行、后 `stop`，两批之间看的是 `running`。心跳读到取消之后 `stop` 晚 300 ms，恢复、撤销、导入三个测试全红，作业成了 `succeeded`。`stop` 之后心跳不再写什么，只改测试修不了。M12 收尾之前它在 CI 或本机门禁上红一次，就开旁支给 transfer 的作业加一个只供测试的观察点（仿 lint 的 `RunObserver`），`cancelSeen` 改等它（同决定 568 的写法）。
- **597 文档的去处与收口**（旁支审查 C 的 LBC-N3、Q1、Q2）：
  - 第 12 节的审查链接与合并号放在 P2 一行（同决定 546、569：放在见到它的那一行，这一次是 P2/S2 推送之前的本机门禁），排在 P2 的各个 Step 之后。
  - 05-lessons 第 3 节"交错与锁"首条之下加三个子条，不放第 1 节；记录叫 `reviews/M12-lintbusy-await-expiry-review.md`；M10 的文档与总体设计不改（同决定 570）。
  - 文档提交之前先把 `main` 合进本支。
  - 修复核对第 1 轮三位都没有 Important、Minor（A 1 条、C 5 条 Nit，去重 5 条，在记录里写对），不开第二轮。
  - 已推的提交说明不改写：`7abfd6ef` 的推断与"well under a millisecond"、`d8e664b3` 的两句写在记录的"更正"；P2/S2 记录决定 586 的"亚毫秒"加后注（同决定 481、547、554、571、579、583、588）。

### 2026-11-18：e2e 装浏览器的旁支的决定 598–603（主持者的决定，负责人可以改判）

598–603 在旁支 `m12-e2e-browsers-install` 与 `m12-e2e-browsers-cache`（M0 的故事，在 M12 里发现）的查因、实施、审查与修复里定。理由与审查里的出处在[旁支的审查记录](reviews/M12-e2e-browsers-cache-review.md)"主持者的决定"一节；实施中定的不编号，在记录里。

- **598 e2e 装浏览器撞上作业的 20 分钟：开旁支修，时限分开，并缓存 apt 的系统包**（主持者的查因与实施说明；实施报告；旁支审查 B 核实查因的数）。
  - 提交 `aca36959`、`7c7865d5`，修复第 1 轮 `8641a14e`、`c926e8be`；合并见第 12 节。
  - **起因**：e2e 在"Install the Playwright browsers"一步撞上作业的 20 分钟被取消，E2E 没开始或跑到一半：2026-11-18 到约 12:30 UTC 至少 20 个 attempt，11-11 `main` 的 4 个，10-07 的 1 个。都只重跑 e2e。
  - **原因**（核实）：
    - 11-18 是 GitHub 放在 AWS 上的 runner（11-14 起才出现）的 apt 镜像 `us-east-2.ec2.archive.ubuntu.com` 取包慢，87–147 kB/s；那天 AWS 上要从镜像取包的都慢，Azure 上都快。
    - 11-11 是 Azure 上解析不了 `azure.archive.ubuntu.com`；10-07 卡在 `apt-get update`。
    - 约 125 MB 的系统包自 M10/P4/S3 的 `ec6569cb`（11-07）起才有。
    - 镜像为什么慢是推断。
  - **不采用的**：
    - 换镜像：哪个镜像从 AWS 上快核实不了；apt 的 mirror 方法只在失败时换下一个。
    - apt 的 `Acquire` 超时：管不了慢而不断。
    - Playwright 的官方容器：`make e2e` 要 Docker 起 PostgreSQL，容器里要另装 Go 与 Node，每次还要拉约 2 GB。
- **599 E2E 一步自己 18 分钟，是第 3 节第 11 条的界；装浏览器一步 30 分钟；作业 55 分钟；守卫 `TestTheE2EStepHasALimitOfItsOwn`**（实施报告；旁支审查 A 的 EIA-M2 = B 的 EIB-M2，B 的 EIB-Q1，C 的 EIC-M3；核对第 1 轮 A 的 EIA-C1-N5、B 的 EIB-N3）。
  - **18**：原来作业 20 分钟里 E2E 实际能用的时间，绿的 235 次中位 18:32（B）；约是最长的绿 E2E（8:52）的两倍。超过时这一步失败、作业红，不是被取消：探针 run 44283057396 的 E2E 一步 "timed out after 1 minutes"，作业 failure，报告照传。
  - **30**：11-18 截在半途的最慢一次推算约 24.7–24.9 分钟（B）；新分支的 run 44298257881 不命中缓存，实测 25:06（86.5 kB/s）。不命中时取包平均低于约 72 kB/s 就会超过，是推断；那时红在这一步，不是被取消。实施时的 25 由修复第 1 轮改。
  - **55**：两步各自的时限先于作业的到；守卫要求不少于 30 + 18 + 5。
  - **守卫**：E2E 一步的时限恰是 18；两步都有自己的时限、都不带 `if`；作业的 `if` 是 server 作业的；不许 `continue-on-error`。在 `main` 的 `ci.yml` 上红；变异见记录。
- **600 装浏览器的系统包按周进缓存**（实施报告；旁支审查 A 核过键、restore 与 save、`autoclean`）。
  - 键 `playwright-debs-<ImageOS>-<Playwright 的版本>-<ISO 周>`，restore-keys 是去掉周的前缀。
  - 放回 `/var/cache/apt/archives`，装完 `apt-get autoclean` 再存。
  - restore 与 save 分开，装完就存：合在一起的 `actions/cache` 只在作业成功时存，E2E 红了，重跑就取不到。两个子 action 钉同一个提交，不是新的第三方 action。
  - 命中时 apt 说 "Need to get 0 B"（run 44283057396、44286707271，后者在 AWS 上）。
- **601 放回的 `.deb` 不核哈希，信任写进 `ci.yml` 的注释与记录**（旁支审查 A 的 EIA-M1；核对第 1 轮 A 的 EIA-C1-N3）。
  - apt 只比文件名（含版本）与大小（A 在 apt 3.2.1 上实测：同大小改一个字节照用），之后以 root 装上。
  - 不加核对的理由：
    - 大家共用的那一份只有 `main` 上的运行写得进；能推 `main` 的人本来就能改 `ci.yml`。
    - 在 `main` 上跑的代码也写得进：钉住的依赖、`go run` 的工具、action，以及合并进 `main` 的分支代码。但它们本来就能经 setup-go 与 pnpm 的缓存或 `pnpm install` 影响这个作业。推断：缓存的写入凭作业的运行时令牌，不归 `permissions` 管。
    - 别的分支与 fork 的 PR 存下的只有它自己读得到。
    - 这个作业的令牌只有 `contents: read`，工作流不用任何 secret：实施者在 `7c7865d5` 上核过，A 在 `c926e8be` 上再核。
  - 怀疑缓存坏了时，删掉键以 `playwright-debs-` 开头的条目；那是写操作，先问负责人。
- **602 反向对照的探针与它的撤销不进 `main`**：故意红的 `cd306960` 违反"每个提交单独为绿"。
  - 修复与文档在从 `7c7865d5` 开出的 `m12-e2e-browsers-cache` 上做，合并的是它。
  - `m12-e2e-browsers-install`（头 `44ae77c7`）留在远端作证据，不删、不强推。
  - 探针的 run 44283057396 照样记进记录。
- **603 后缀、文档的去处与收口**（旁支审查 C 的 EIC-Q1、Q2；核对第 1 轮 C 的 EIC-C1-Q1；主持者）。
  - **后缀**：写 `(M0 story, found in M12)`，因为作业的 20 分钟与 `--with-deps` 出自 M0/P6/S1 的 `211d2a9a`；约 125 MB 来自 M10 的 `ec6569cb`，写在记录的"起因"。范围写 `build`、`server`。
  - **修复核对之后不开第二轮**（同决定 547、554、583）：核对第 1 轮三位都没有 Important；新的 Minor 只有 B 的 EIB-M4，说的是已推的 `c926e8be` 的说明，写进记录；新的 Nit 都在本提交写对或记成接受。
  - **已推的说明不改写**（同决定 547、554、571）：`aca36959`、`7c7865d5` 的不准处在 `c926e8be` 的说明；`c926e8be` 自己的两处（EIB-M4、EIC-C1-N1）在记录的"更正"。
  - **记录与第 12 节**：记录用合并的分支名 `reviews/M12-e2e-browsers-cache-review.md`，标题写两个分支。第 12 节放 P5 一行：11-18 最早出事的 run 44213948330 是 P5/S1 记号的 `main`，`ci.yml` 也属 P5 的工作流（决定 546、569 的规则）。
  - **随本提交改的**：第 3 节第 11 条改说 E2E 一步的 18 分钟；第 10 节"e2e 的偶发挡发布"一行；06-environment 第 4 节加一条；05-lessons 第 7 节加一条。
  - **接受、不改代码**：
    - 缓存的善后命令失败也让作业红、E2E 不跑（EIA-N2）；
    - `ci.yml` 注释里"至 18 日"的计数截在 07:34 UTC（EIA-C1-N4），记录写明口径。

### 2026-11-19：P2/S4 的决定 604–611（主持者的决定，负责人可以改判）

604–611 在 S4 的实施、审查与三轮修复里定（604–608 在修复第 1 轮，605–607 第 2 轮、606 第 3 轮补句）。每条在 [P2 设计](02-P2-web-polish.md) 4.4 与 10.4，理由与审查里的出处在 [S4 的审查记录](reviews/P2-S4-reading-view-review.md)"主持者的决定"一节；实施中定的不编号，在记录里。

- **604 写入之前就在、之后仍在的音视频，`adopt` 不换它**（S4 审查 A 的 M1、Q2，C 的 M1；核对 C 的 C1-M1、A 的 C1-N2）：撤掉时记下当时文章里全部的音视频（`WeakSet`），重跑时仍在文档里的跳过，键照旧按全部音视频；开始了与否都算。别人删掉它前面同一附件的块、或在它前面插进同一附件又改了后面的块，它都留在原处、照样播放、全屏不退（`9892df53`、`09c7518d`）。接受的限制（P2 设计 4.4.8）：它所在的块换新、它之前同一附件的嵌入多了或少了时，可能接不回、接错位置，或它之前被删掉的那一个被接回到它的位置；改前整篇写时同样；按块的映射配对要改 `adopt` 的接口，进 v0.2。
- **605 滚动区只去掉自己给的 `tabindex`**（S4 审查 A 的 M3；核对 A 的 C1-N1）：第 4 步与锚点给的 `-1` 留着，焦点不再一帧之后丢到 `body`；变宽时盖掉了别人给的，记下，变窄时还回（`59a5012d`、`c9bdfdb7`）。
- **606 第 3 步只在映射到的块与同一处插进的近旁几块里找**（S4 审查 A 的 M4、Q1；核对 A 的 C1-M1、C2-M1）：先在映射到的块里，再在同一处（两个留着的块之间）插进的别的块里，按远近、同远时先后面的，只看离映射到的那一块不超过 max(1, \|插进 − 删掉\|) 块的；都没有走第 4 步；不在全篇找，焦点与视口不被拉到远处同键的元素（`59a5012d`、`680c5ef5`、`1294383e`、`1e43b3cc`）。上限按整段的差算，读者那一块前后的删插方向相反时找不到它的新版本，走第 4 步（4.4.8，核对 A 的 C3-N1）。按远近取可能取到别人新写的同键元素，写进 4.4.8（核对 A 的 C2-N1）。
- **607 6.2 之后焦点没有着落时给页的标题**（S4 审查 B 的 M1；核对 B 的 C1-M1、C2-N1、C2-N2）：照 S1 的 18.6；只在 `forbidden` 的拒绝之后等那一次重读，答复之后等待就结束：仍可写不动，被拒之前就已不可写的当场给，这次重读才变成不可写的在那次渲染之后给；可写与否读 store（今天与读渲染等价，读 store 是防御）（`296b0a71`、`6b5afe82`）。可写与否因事件变了、不经拒绝时的失焦 S4 之前就有，进 4.4.8 与 v0.2。
- **608 PG15 第 3 格的视频三种都播**（S4 审查 B 的 M2）：`e2e/fixtures/assets.ts` 的 `vp8Webm`，在 Playwright 的 Chromium 里录一次的 VP8；WebKit 放得出；M4-12b 三种都红（`b8edc731`）。
- **609 Go 的样例测试在 asset 模块的根包**（S4 实施报告第 7 节第 1 条；S4 审查 C 的 M3）：`modules/asset/signed_view_test.go`，用模块自己的 `Embeds.Of` 与 `ContentURL`：根上的 `Embeds` 字段不导出、`NewEmbeds` 只收连接池，组合根里要么起库、要么照抄要钉的那一段。照抄的组合根两段（`registrants.go` 的 `markdownExtensions`、`assets.go` 的 `assetEmbeds.assets`）各有注释"改了要同步"（`5b3a6669`、`131f7d27`）。
- **610 8.1 复现了，在 S4 修，再看多一个条件**（S4 第 0 步，审查 C 的 M2、N5；P2 设计 4.4.4 的 8.1，决定 402）：Playwright 的三种引擎里 far 的形状 30/30 复现；POP 导航去锚点之后的下一帧，窗口在视图滚过去之后又滚过、焦点仍在锚点的元素上、读者没有动作、元素不在视口里时，再滚一次；四个条件各有一格正反例，PG15 加一格（`37386f54`、`34bb6315`）。读者离开之前滚到看不见那个标题的地方的，后退回来视图仍回到标题（4.4.8）。附录 A 的 8.1 照此加括注。
- **611 S5 合并 `main` 时删掉它自己那一份 `wav(seconds)`，用 S4 的**（S4 审查 B 的 Q1）：S4 先合并、在 `e2e/fixtures/assets.ts` 加了它（P2 设计 6.4 第 6 条写的是"S5 已先合并的，用 S5 的"）。

### 2026-11-19：P5/S2 的决定 612–617（主持者的决定，负责人可以改判）

612–617 在 S2 的审查与修复第 1 轮里定。每条在 [P5 设计](05-P5-deploy-release.md) 3.25 与 10.2，理由与审查里的出处在 [S2 的审查记录](reviews/P5-S2-publish-release-review.md)"主持者的决定"一节；实施中定的不编号，在记录里。

- **612 npm 一侧的许可证取 `@nervewiki/web` 与它用的工作区包的生产依赖，另固定收 tailwindcss**（S2 审查 A 的 I1、Q1）：`npm.mjs` 列 `pnpm --filter "@nervewiki/web..." licenses list --prod --json`：只写 `@nervewiki/web` 时 pnpm 不往工作区包 `@nervewiki/api-client` 里走，打包进前端的 `openapi-fetch`（MIT）没有许可证（它的依赖 `openapi-typescript-helpers` 也一起漏了）。tailwindcss 是 devDependency，它的 preflight 与默认主题编进 CSS，固定收（没装就失败）；vite 的 modulepreload polyfill 是构建工具的运行时片段，不收。今天 233 个包（`dcaa7fe3`）。改了决定 439 的"照 `pnpm licenses list --prod --json`"（P5 设计 2.3）。
- **613 许可证文件的名字不收代码，另收 `unlicense`**（S2 审查 A 的 M1）：两侧同一条 `^((un)?licen[cs]e|copying|notice|patents)([.-].*)?$`（不分大小写），再排除以 js、cjs、mjs、jsx、ts、cts、mts、tsx、go、json、map、sh、py 结尾的名字。cytoscape 的 `license-update.mjs` 不再进镜像；只有 `license.go` 的模块让构建失败（`dcaa7fe3`）。
- **614 两侧的清单对着发出去的东西核对**（S2 审查 A 的 M2）：`go.sh <程序> <输出目录>` 收完之后，`go list` 的模块要恰是 `go version -m <程序>` 的 `dep` 行、程序的 Go 版本要是收的那个，不等就失败、构建失败；演练与 `sbom` 作业的 Check the SBOMs 要前端 SBOM 里 `@nervewiki/*` 之外的每个包都在 `/licenses/npm/INDEX.tsv`，不在的 `@types/` 只打出名字（只有类型声明，构建丢掉；今天 `@types/trusted-types`）；`image-smoke.sh` 另要 `openapi-fetch` 一行与两份清单至少 40、200 行，作本机一侧的底线（`dcaa7fe3`）。
- **615 Release 有没有、是不是草稿读 REST 的列表**（S2 审查 C 的 M4）：`gh api --paginate "repos/$GH_REPO/releases?per_page=100" --jq '.[] | select(.tag_name == "<tag>") | .draft'`，任何失败都失败，同一个 tag 多于一个也失败。gh v2.103.2 的 `release view` 并行查 REST 与 GraphQL，两次都失败、其中一次是 not found 时报 `release not found`：重跑 `draft` 时一次临时的错就会被当作没有、再建一个草稿。列表对 `contents: write` 的令牌含草稿是推断（文档："Only users with push access will receive listings for draft releases"），rc 上证明（`0fe3d663`）。改了决定 441 的"`gh release view` 同样只认 release not found"（P5 设计 2.3）。
- **616 守卫另核对的几条**（S2 实施报告第 8 节；审查 B 的 M1、M2、N1–N3、Q1）：`publish` 的最后一步是版本号的 tag、它的步骤都不写 `if`、第三方的 action 只有 cosign-installer、恰有一步逐字的 `cosign sign --yes "$IMAGE@$DIGEST"` 与三步 `actions/attest`（摘要都是索引的、`push-to-registry: true`、来源证明与两份 SBOM 各一）；`verify` 的每个跑命令的步骤同跑 smoke 的只有 name、env、run，矩阵恰是两种架构、`strategy` 与 `build` 的相同，`gh attestation verify` 与 `cosign verify` 的身份与签发者逐字、没有 `-regex`；只有 `release` 公开 Release；决定 561 的两步只有 name、run（Index 加 id）；三份工作流的每个 `uses:` 都钉 40 位的 SHA（`e5ecae06`、`e6f535aa`）。别的 Step 改 `release.yml`、`ci.yml`、`vuln.yml` 时照它们写，新的 action 也钉 SHA。
- **617 SBOM 的文档 `name` 在不推送的地方先核对**（S2 实施报告第 8 节第 6 条；审查 C 的 Q1）：演练要镜像的那份是 `nervewiki`、`sbom` 作业要 `$IMAGE`，前端的都要 `@nervewiki/web`。syft 换了命名时，tag 的运行停在只读的 `sbom`，不到 `verify` 才红（那时版本号的 tag 已经出现，只能删草稿、出 rc.N+1）。镜像的那份是 `$IMAGE`、不是设计推断的 `$IMAGE@sha256:…`，是 P5 设计 3.12 预先准许的偏离：`785d48be` 的演练读出 `nervewiki`，tag 上是 `ghcr.io/open-nerve/nervewiki` 由 syft v1.54.0 的源码推出，rc 上证明（`e5ecae06`）。

### 2026-11-19：P2/S5 的决定 618–624（主持者的决定，负责人可以改判）

618–624 在 S5 的实施、审查与四轮修复里定（618、619 在开工的核对之后，620、622、623 在修复第 1 轮，621 照实施的结果，624 在 17.7 的实施与修复第 3、4 轮）。每条在 [P2 设计](02-P2-web-polish.md) 4.5 与 10.5，理由与审查里的出处在 [S5 的审查记录](reviews/P2-S5-editor-assets-review.md)"主持者的决定"一节；实施中定的不编号，在记录里。

- **618 15.2 在 `requestAnimationFrame` 里设回 `open`**（S5 开工核对 4.5.0.5 第 2 条；改了决定 419 的写法）：`setTimeout(0)` 每次点击画出一帧展开的 callout（Chromium，20/20），rAF 在同一次渲染更新的绘制之前设回，0 帧；三种引擎里其余各格相同。AS6 加"没有一帧画成展开"，"改回 `setTimeout(0)`"只由它抓（`8d2b04de`）。
- **619 12.8 照 goldmark 去掉容器的记号**（S5 开工核对 4.5.0.5 第 4 条；审查 B 的 M1、N1，核对 B 的 CHK1-B-N1、C2-N1）：段落之后的行在引用里去掉引用记号（callout 也是），在列表项里去掉至多到首行那一列的行首空白，顶层照原样；照字面 5/44 坏，去掉之后 0/52。审查与核对又补了 setext 的下划线、ASCII 空白、列表项的列、越过列的制表符与引用记号之后的制表符，都先对着 goldmark v1.8.6 核过（`2eeedea6`、`e1e95d61`、`fa849645`、`d6594761`）。`$$` 公式块与"首行比内容列多缩进"记成限制，进 v0.2。
- **620 16.3 的"停住"从第一次上传进度起算**（S5 审查 A 的 I1、Q2）：`send` 之后不起计时，第一次 `upload` 的 `progress`（或 `upload.load`）才起；HTTP/1.1 下浏览器里排队等连接的上传不再被判停住（三种引擎的探针：8 个 5 MiB 改前后 3 个 60 秒中止，改后都答 201）。不在 store 里限并发（`4390039f`）。
- **621 15.1 的 CSS 另加 `@layer base` 的 `height: revert-layer`**（S5 实施）：Tailwind 的 preflight 在 `base` 层写 `img, video { height: auto }`，比 `height` 属性强，设计的三条在三种引擎里都留不住写下的高度；标了的元素在 `base` 层退回到属性的高度（`8637c655`）。P2 设计 2.1 第 4 条随之改。
- **622 16.11 按批记 `before`**（S5 审查 A 的 M1、C 的 M1）：在 `upload` 里与 `used` 同一时刻按批记兄弟的 id 集合，被拒重试时与 `free` 同一时刻再记；设计的"`send` 开头记"每个文件一次，2,000 × 2,000 的守卫贴着限；只比名字不等价（别人原地改名时叫读者删掉别人的附件）（`87771394`）。新建、移来、恢复的同名附件仍分不开，进 v0.2。
- **623 AS6 不复制附件行，也不在 Firefox、WebKit 另跑**（S5 审查 C 的 Q1 取 (a)、N2）：Chromium 里附件行是可拖的 `li`，复制不出；行的 `select-none` 由 vitest 的结构断言守。决定 407（P2-30）的依据照此加注（`5dea6e7a`）。
- **624 17.7 被拒的记录**（S5 的 17.7；核对 A 第 2 轮的 C2-M1–M3、C 的 C2-Q1，第 3 轮的 C3-M1）：`signAnew` 自己看记录；计时与 `online` 到点时，没有错、在失败处或之后的或错已清、读者拖过的（`seeking`）丢掉记录、不签（错还在的也拖得动：Firefox 留着网络错之前缓冲的数据）；撤销只清这个容器的计时，下一次运行给留着的重起计时（间隔照记下的、不翻倍）；有记录的媒体也收进 `kept`，块换新时新元素放到记下的位置（`c5820210`、`d593c67d`、`d8027fb4`）。与决定 604 不冲突：留在原处的不换。

### 2026-11-19：P2/S6 的决定 625–632（主持者的决定，负责人可以改判）

625–632 在 S6 的实施、审查与修复第 1 轮里定（625 在实施之后、审查开始时，放在修复第 1 轮做；626–628 在修复第 1 轮；629–632 是实施与修复里改了设计写法的几处，照 S4 的先例编号）。每条在 [P2 设计](02-P2-web-polish.md) 4.6 与 10.6，理由与审查里的出处在 [S6 的审查记录](reviews/P2-S6-versions-trash-history-search-review.md)"主持者的决定"一节；实施中定的其余几条不编号，在记录里。

- **625 "移到……"对话框的子树与高度在循环外算一次**（S6 实施报告的问，4.6.6 末；审查 C 的 Q1）：`page-tree.ts` 加 `holdersOf(tree, id)`，被移的页的子树与高度只算一次，答一个判断函数，`MoveForm` 用它；`canHold` 不改（拖放仍用它，测试拿它作参照，深与平两种形状里对根与每一页比两者）。深的形状同一会话各量一轮：中位数 1,008 → 190 ms（`07ca08bc`）。
- **626 27.2 设计原文的两处在 S6 修**（审查 A 的 M1、M2、Q1）：笔记本的外壳卸下时没读过的树也标未确认（第一次读树在途时离开，那次读不算数）；`PageLayout`、`VersionPage` 挂上时未确认就记下缓存里的错误，只说别的错误（离开前读树失败的不在强制的读答复之前先说），已确认时挂上不记（`75888280`、`47e8871d`）。
- **627 macOS 上方向键等不算一步**（审查 B 的 Q1，取甲）：`onMac()` 时方向键、Home、End、PageUp、PageDown 不算一步，只有打字找选项的单个字符算。方向键在 Mac 上若真是逐项换，照此是立即用，与改前相同；若打开列表（推断），算一步则列表里的选择要等读者离开，比改前更坏。测试 mock 平台；真的 Safari 与 Chrome 进发布之后的人工清单（`c2aba4ae`）。
- **628 22.2 的 `merged` 留在外壳**（审查 C 的 M2）：项离开回收站又回来、`Preview` 重新挂上时照旧说；设计判"放回 `Preview`"等价不成立，那一条变异改成"不等价，由一格抓"，补那一格（`03ebf362`）。
- **629 24.8 的表项读完并显示才删**（S6 实施；审查 B 的 N2）：刷新器在键最近没读过时当场开始读，照设计的"读开始时取下"写，紧接着的 `links` 事件看不到它；表项带 `started`，读开始时标上，读完并显示之后落定、是同一项才删，读的途中又来树变了的 `pages` 事件就放一个新的；等过之后先看这一代停了没有（`6ba7f186`、`4dd02868`）。
- **630 26.5 `useChoice` 的按键**（S6 实施；审查 B 的 M1、M2、N4）：每次按键都重设"下一个 `change` 是不是一步"（不只在一步的键上设真）；`stepping` 另排除 Ctrl、Meta；有等着的选择时回车 `preventDefault`（Chromium 在关着的选择上按回车打开列表），没有时照旧；用的值已在用时不再读（`8af5c970`、`c2aba4ae`、`47e8871d`）。
- **631 22.19 的面包屑与跟随的 `not_trash_root`**（审查 A 的 M3、N2、N3）：外壳的面包屑包在带 `useFocusLeaving` 的小组件 `Trail` 里（不取"说明页也画面包屑"：说明页的标题是"回收站"，单项可能还没读到）；面包屑 `mb-2`、alert `mb-4`，面包屑到标题 0.5rem，同变更集页与页的外壳；跟随的恢复被拒为带 `root_id` 的 `not_trash_root` 时不等单项（`fd4eddeb`）。
- **632 21.10 另一个版本显示时清掉说的话**（S6 实施）：设计写"只在 `said.of === data?.id` 时显示"，只比 `of` 时前进回到原来的版本又说，设计自己的格红；显示仍比 `of`，管另一版本还在读的那一段（`d5ffb17f`）。

### 2026-11-21：P4/S3 的决定 633–651（主持者的决定，负责人可以改判）

P4/S3 的实施与修复第 1、2 轮里由主持者定的（出处是主持者给实施者的说明：`brief.md` 与说明 1–5，审查之后的处置 `review/host-verdicts.md`，都在设计工作目录的 `m12/p4s3/`）。每条在 [P4 设计](04-P4-performance.md) 3.11、3.12 与 10.3，理由与数在 [S3 的审查记录](reviews/P4-S3-server-measures-review.md)"主持者的决定"一节；实施中定的其余几条不编号，在记录里。

- **633 3.8 的省法取前三条**（`brief.md`）：5,000 本只跑一轮；F 的第二轮只做 B4、不导出；`fsync=off` 的库只删一次。不取第四条（3.5.4 每个作业仍取最近的至多三次 7763：B4、B5 的推断靠它）。实际省了约 47 分钟、不到 1 分钟、约 5 分钟（推断，照 r2 每次操作连同恢复与等安静约 2 分钟）。
- **634 批次接着跑、不按夜，机器归 S3 独占**（`brief.md`）：每批整段持一次 heavy 的锁、批与批之间释放；次序 4.6 的大图、B、C、D、E、F，G 推算。
- **635 B4 的复核**（说明 2 第 6b 条）：主行推断两轮都过 18 秒（15 秒的 1.2 倍）的那种操作不做 3.9 第 2 条的复核：超出远大于两轮之间约 10% 的摆动，复核改不了结论，省约 1.5–2 小时；15–18 秒之间的照 3.9 复核那一格两轮。实际：删除（记录的式子 16.80–16.88，同口径 17.5–18.1，10.3）复核两轮也超；移动、改名不复核。
- **636 B5 第一行推断的中位两种取法都写，按较差的下结论**（说明 1 第 3 条）：窗口里 8 次时下中位与平均不同（恢复 11.463 与 11.575 秒，撤销一次删除 11.861 与 11.900 秒）；两种都过 15 秒。
- **637 离 5 秒近的两格不复核**（说明 2 第 2 条）：`BenchmarkRevert` 删除 / 10 万 / 无统计 4,813 ms、1,800 个版本无统计 4,732 ms，照 3.6 是"过"；记录写离界线近、两轮差约 10%。
- **638 B4 的插值**（说明 2 第 6a 条）：删 `vault` 与删第一层文件夹两点按节点数线性，给出删除的主行与 × 1.25 一行到 15 秒的节点数（约 52,700、41,500）与导入上限 45,000、40,000 时的主行（13.0、11.7 秒），都记为推断，给负责人定 `MaxJobItems` 用。修复第 1 轮（审查 A-I1）：这几个数是记录的式子的；同口径约 48,900–50,300、38,100–39,400 个节点，45,000、40,000 时主行 13.6–13.9、12.2–12.5 秒，× 1.25 一行 40,000 时 15.2–15.7 秒（10.3）。
- **639 B2 的 k = 8 病态"第一次超、复核过"**（说明 3；修复第 2 轮改写，审查 B-I1）：同一方法的六次 2,282–2,525 MiB（第一次的第一轮 2,380、上界 2,433；E 2,525、2,282；复核 2,328、2,348；审查 B 的重跑 2,345），照 3.9 第 2 条是"第一次超、复核过"。不开 Step：负责人第 13 问第 4 条已把 B2 改判成约 2.6 GiB，k = 8、k = 16（五次 2,385–2,592）都在其内。
- **640 B5 的两件等批次 F 之后与 B4 一起问负责人**（说明 2 第 4 条）：导入上限就是 `MaxJobItems`，B4 删除的正是导入上限的 vault，两件连在一起，负责人一次看全（第 13 问）。
- **641 设施的问题都不在 v0.1 修，进 v0.2 的移交**（说明 2 第 3 条、说明 3）：
  - 升级批准守卫在规模开关下的计划断言（`plans_alone` 的 `AttachmentInfos` 走 Seq Scan：该随规模放宽，还是附件也随规模放大）；
  - 按页的日志 10⁶ 的撤销数组测试的播种（`NOT IN` 的子查询不哈希，29 分钟没做完；`NOT EXISTS` 或先建临时表就是哈希的反连接）；
  - `BenchmarkRevert` 给 10 万个别的节点的播种（`others`）每次 30–37 分钟（推断是外键检查用了表还小时缓存的计划）；
  - `NERVE_BENCH_VERSIONS=1000` 单独设时只有 259,900 行；P4 设计 3.6 的那一句照实改成 1,800 个版本、419,900 行；
  - `perf ops` 碰到 500 就整个退出，500 之后该量的几种操作量不到（3.6 的判据把本机的 500 当作结果）；
  - `perf bodies` 的组合之间不触发 GC，下一个组合的第一次 GC 携带上一个组合的垃圾：窗口要去掉那一次（修复第 2 轮改，审查 B-I1）；
  - `perf editors` 的版本读数含树写者的新建（修复第 1 轮加，审查 C-I1）；
  - 量时间的段里没有每 5 秒记负载（修复第 2 轮加，审查 B-M2）。
- **642 负责人第 13 问之后的两个条件 Step，都照 4.7.5**：
  - **P4/S6 `MaxJobItems` 降到 45,000**：常量与随它的守卫；导入上限的文字（`config.yaml` 的注释、README 与文档里写"6 万"的）；`server/tools/perf` 导入上限的 vault 形状，P4 设计 3.4.3 随之改。改动小，不另写设计文档：设计是 P4 设计 3.12，放在 S3 的文档提交里，与实施一起审查。
  - **P4/S7 B4 的树写**：照负责人的选项，先写设计，查清移动、改名的时间花在哪；目标是 6 万个节点的删除、移动、改名在 7763 上不超过 15 秒；设计若要大改（例如改成后台任务），先问负责人。S6 先合并；S7 的设计写明导入上限降到 4.5 万之后 6 万个节点的最上层怎样搭（例如两次导入再移到一页之下）。
  - P4 设计第 0 节与第 12 节 P4 一行都加这两个条件 Step。
- **643 负责人第 13 问的后两条落在 P5/S3**：B5 硬期限的余量写进部署文档的硬件一节（4.8 原已这样计划）；B2 的约 2.6 GiB（2,662 MiB）：`README.md:141`、`:630` 的"约 2.4 GB"由 P5/S3 改，写明单位，部署文档按约 3 GiB 留内存。本提交在 [P5 设计](05-P5-deploy-release.md) 3.18 与 4.7.1 的 B2、B5 两行加了这一问的注。
- **644 B5 规则一层在守卫的状态下过 5 秒，记为已知限制**（说明 4；审查 A-I2、C-Q1）：规则一层照 3.6 的量法（计时之前 `VACUUM`）判"过"；同一形状在守卫的状态下（无统计、不 `VACUUM`、历史补满），本机单元约 5.4–5.9 秒，推断过 5 秒。与负责人第 13 问第 3 题（界线处的恢复与撤销一次删除）是同一处：负责人选了不降常量、写明余量；进部署文档的余量一句与 v0.2 的清单。主持者在汇报里告诉负责人，负责人可以改判。
- **645 一页 100 万行历史的删除与 30 万页的升级只量，不开 Step**（说明 4；审查 C-I2）：照 4.7.1、4.7.5 是只量的规模；推断 7763 上分别约 19–20 秒、19–26 秒，过期限（约 75–80 万行；约 14–16 万页过 11.25 秒、18–20 万页过 15 秒）；去处是已知限制、部署文档与 v0.2。`batch` 在链接的界上 8.2–8.7 秒也进已知限制。负责人可以改判。
- **646 P4/S7 量改前、改后时，`fsync=off` 的库删三次、取中位，记每一次的位置**（说明 4；审查 A-Q1）：S3 的删除三次依次变慢，`fsync=off` 只删一次（633）让主行的式子两边位置不同；S7 不取这一条省法。
- **647 B4 在编辑者负载下的那一样交 P4/S7，在修完之后只记**（说明 4；审查 A-M4、C-M3）：S3 的批次 F 漏量了（3.6 B4 的"只记"，取舍 P4-10）。
- **648 3.7 第一行补量**（说明 4；审查 C-M4）：F 量时死行的条件没造出、`Ancestors` 选错了叶；修复第 1 轮另起一个栈，关 `nodes` 的 autovacuum、删除并恢复一次 `vault`、不 `VACUUM`、活着时 `ANALYZE`，`Ancestors` 取最深的一页，经 `heavy.sh` 一次锁。
- **649 B3 在上限处重量，设计 3.6 B3 的形状照实改**（说明 5；审查 B-I2、B-Q1 取 (a)）：B3 是有预算的一行，设计 3.6 一面写"照 M9/P2"，一面给了小的体（上限的 14% 与 0.2%），是设计的错，不是实施者的。S3 在 `m12-p4-s3` 上加设施的提交（`119611ca`，修复第 3 轮的 `125a765c`、`9def3138` 改它的测试；`tools/perf` 的 `mcp -bound`：每个请求体恰在 37,814,272 字节，正文是六字节转义的 5 MiB，`_meta` 的长字符串补到上限；另一种是只有长字符串），这是 S3 唯一改代码的提交，不改服务端；两轮重量，每 5 秒记负载：七种形状两轮的堆增量 705–810 MiB（过了值的上限的 166–257），两轮的最大 810 MiB，第一次的第一轮 881 MiB，都 ≤ 1,024 MiB，B3 过；第三个凭证等满 2 秒答 503（L 除外）。原来的小体留作另记。
- **650 B2 的窗口：预热留着，只去掉每个组合携带上一组合垃圾的那一次 GC**（说明 5；审查 B-I1）：实施时整段去掉了预热，病态组合在预热里的真峰值也去掉了；全部格照新的窗口重算，k = 8 病态 E 第一轮 2,525（原 2,408），结论不变。05-lessons 的那一条照审查 B 的写法。
- **651 部署文档写 B2、B3 与树的重读同时到的推算**（说明 5；审查 B-I2、C-M1）：解析预算、大请求体的两个名额与树的重读是各自独立的界，可以同时满；最坏相加：B2 与 B3 上限处同时到，堆约 3.3–3.5 GiB、进程约 3.5–3.8 GiB；再加 8 个标签页重读 10 万页的树，约 5 GiB（推算，没有量）；约 3 GiB 只覆盖 B2。主持者在汇报里告诉负责人。

### 2026-11-21：P4/S7 设计的决定 652–667（主持者的决定，负责人可以改判）

652–667 是 P4/S7（B4 的树写，P4 的条件修复，决定 642）的设计审查之后的取舍，依次是设计里的临时号 P4S7D-1 到 P4S7D-16。每条在 [P4/S7 设计](04-P4-S7-tree-writes.md) 2.3，审查里的出处在 [P4/S7 设计的审查记录](reviews/P4-S7-design-review.md)。"A"是大的到达换一种读法，"B"是一个单元只读、只解析一次。

- **652 A 与 B 都做**：不改接口、网页、事务与锁，不是负责人说的"大改"，不再问负责人。只有 A 时移动、改名推到 7763 上约 13 秒、× 1.25 一行约 16 秒，贴着期限；A 加 B 推断约 7 秒、× 1.25 约 9 秒；删除约 11–12 秒、× 1.25 约 14–15 秒（推断，S7 照实量）。
- **653 门槛：一个单元的到达的键与目标合起来至少 4,000 个时读宽读**：只数键与目标，不数来源；照设计 1.3 的切点，约 2,000 个节点以上的改名、移动、删除走宽读，内容的写留在今天的读上。
- **654 宽读两条、按 `plan_cache_mode` 选**：平常读 `LinksReachedWide`（常量数组的 `= ANY`，PostgreSQL 哈希它们，不看大小与 `work_mem`）；设了 `force_generic_plan` 时读 `LinksReachedWideGeneric`（成员与排除都是对 `unnest` 的子查询，通用计划里都是哈希的子计划）。`Store.Links` 过门槛时经 sqlc 生成的 `PlanCacheMode`（缓存的语句，在单元的事务里）读设置；三条读各在一个同形的转发函数里，archtest 的规则不放宽。
- **655 `LinksReachedExcept` 不改**：B4 的四种操作都不走它；走它的是 LLM Wiki 的笔记本里一个 batch 同时写准备好的来源、又删除、移动或改名大的子树。删除至多约 1 万个节点（推断约 0.2–0.3 秒）；移动、改名推不出比笔记本小的界。已知限制。
- **656 B 的条件写死，经 `Write` 带一份单元的备忘**：树单元、不是 batch（`s.Batch != nil` 给的显式布尔）、是重定位，参与者存它读到的链接与解析；单元的变化与参与者跟着的逐个相同，观察者用它，不再读、不再解析。参与者写页之前丢掉，观察者在 `spread` 之前取走；不存到达；方法对 nil 接收者安全。跨模块交给链接同 `Event.BeforeIngest` 的先例。
- **657 判据照 P4 设计 3.6 的 B4**：删除的 `fsync=off` 删三次取中位、记每一次的序号（决定 646）；× 1.25 一行只列给主持者；改被链得最多的页与删第一层文件夹"不退步"是改后两轮的较大中位 ≤ 改前 × 1.2，违反了复核两轮、仍过列给主持者。
- **658 6 万个节点的最上层由两次导入搭成**：P4/S6 给 `perf vault` 加 `-part k/n`（S6 本来就改它的导入上限的形状；S6 的实施说明由主持者写），S3 的参数逐个写出、按第一层文件夹分成两份，两次导入都导到同一页 `vault` 之下，不再移动。与一次导入的三处不同（变更集、行的次序、跨份的链接）写明，核对比节点数与链接数多。
- **659 改前重量、交错着量**：改前、改后用同一对 zip（记 sha256）与同一个 `perf`，改前 r1、改后 r1、改前 r2、改后 r2，`fsync=off` 两边紧接着；改前是 S7 分支的合并基点（`git merge-base H main`），改前、改后只差 S7 的提交；代码审查的修复改到读、备忘或观察者时重量改后。
- **660 组合根的守卫**：约 1 万个节点的最上层（没人链着）的移动、改名、删除，以按 sqlc 的语句名数调用的结构断言为主（移动、改名 `PlanCacheMode` 1、`LinksReachedWide` 恰 1、`LinksReached` 0；删除宽读 1、`LinksReached` 0），撤回 A、撤回 B 都确定地红；计时只兜期限，界 = max（本机两轮均值 × 3.6，本机两轮均值 + 2 秒）。它守结构与期限，不守 B4 的余量。份按 EPYC 7763 上的作业时间定：守卫进 `test-go-costs-3`；随机对照与宽读的计划测试（`!race`）进 `test-go-costs-2`，`costs-2` 的时限照约两倍的规则从 45 分钟提到约 60 分钟（实施时按本机两轮的墙钟算，写进 `ci.yml` 的注释）。
- **661 编辑者负载下的四种操作，改前、改后各一次、只记**（决定 647）：`perf editors` N = 20 在另一本笔记本里，`perf ops -reps 1 -quiet notebook` 在它稳态之后起、结束之前完；`-quiet notebook`（S7 加）只等 `vault` 那本的搜索队列与搜索之外的 River 任务。
- **662 病态形状照改后的数重新换算到 7763**，标推断，仍是已知限制；400 页 × 2,000 条链接的 `move` 与被几千页链着的页的改名 S7 不改（A 不碰、B 不用）；20 万个节点的到达可能先因共享内存报错。
- **663 调查的 E 是已知限制，F 的两小项不做**：删除随子树的条目与版本数变慢的那一部分不随 S7 变；改后哪一行的主行仍过 15 秒，停下列给主持者，F 的两项作候选。
- **664 今天的 `LinksReached` 在通用计划下对中等以上的到达是平方的**，门槛之下留着：花费随指向到达的键的链接数乘来源数，没有界；只在运维设了 `force_generic_plan` 时出现。已知限制。
- **665 次序写死 P4/S6 → P4/S7**：S6 合并之后再量；S7 的代码可先写，只有 `tools/perf/main.go` 与 S6 同文件、不同函数。
- **666 移动、改名的期限只在不改写任何页时成立**：改写了页的大子树 B 不用，只有 A 的余量，推断主行约 13 秒、× 1.25 约 16 秒；主行仍在 15 秒之内，满足负责人定的目标，× 1.25 那一行只列给主持者。已知限制。
- **667 观察者不另算到达再与参与者的比**：B 的条件之下两边的到达相同；以后改 `spread`、`Affected` 的用法，由代码旁的注释与"两边读 `Store.Links` 的到达相同"一格测试守着。

### 2026-11-21：P4/S6 的决定 668–674（主持者的决定，负责人可以改判）

P4/S6（`MaxJobItems` 降到 45,000，条件的修复，决定 642）的实施、审查与两轮修复里由主持者定的（出处是主持者给实施者的说明：`brief.md`、审查开始时的说明（实施报告第 11 节）、`fix-1.md`、`fix-2.md`，都在设计工作目录的 `m12/p4s6/`）。每条在 [P4 设计](04-P4-performance.md) 3.13，审查里的出处在 [S6 的审查记录](reviews/P4-S6-max-job-items-review.md)。

- **668 `e2e/perf` 的三种大形状在 S6 里修，另一个提交；`folder50k` 两包进同一个文件夹**（审查开始时的说明；实施报告第 9 节第 1 条）：`flat50k`、`flat100k` 照原来的段每段均分成至多 45,000 页的包（两包、四包，各 25,000 页），导入之后的树不变；`folder50k` 保持"一个文件夹下 5 万页"：导入支持目标父页，第一包是文件夹与前 5,000 页，第二包 45,000 页以 `parent_id` 导到那个文件夹之下，不缩到 44,999 页。提交 `45aa3026`；`b447dd28` 让第一包至少一页（核对 B2-N2）。
- **669 A-M1 两样都做**（`fix-1.md` 第 1 条）：`server/tools/perf/vault` 的测试读 `jobs.go` 的 `const MaxJobItems = ` 一行与 `config.yaml` 的 `import_max_entries`、`import_max_unpacked_bytes`、`import_max_bytes`，与 `ImportLimits()` 逐项比；跨模块不引包，读不到或格式变了就失败、不跳过；反向对照：常量与 `jobs_test` 一起改回 60,000，这一格红。`jobs_test.go` 的注释列出随常量手改的各处。
- **670 守卫的界不重定**（`fix-1.md` 第 2 条，审查 A-Q1）：两个请求的界照决定 249 按 7763 定；最大的一步那一格，界取的是 60,000 时本机的三倍，留着是为 7763 上的余量，免得偶发地红。对着 45,000 的导入守卫实际松了（最大的一步的界是本机的约 3.5 倍、6975P-C 的约 2.7 倍、7763 的约 2 倍），P4 设计 3.13 写明。
- **671 导入上限的 vault 形状照现在的写法**（`fix-1.md` 第 3 条，审查 B-Q1）：S3 的形状按比例缩到四分之三（44,910 个节点）；节点在上限处，解压约 73%，不是字节上的最大导入。M12 余下没有人用它量字节上的最大，B8 已用 S3 的形状量完；要量取 `-attachment-bytes 1072KiB`（推断，未实写）。
- **672 仍取 45,000，不补量**（`fix-1.md` 第 6 条，审查 C-Q1）：S3 的 50,000 两轮（有统计 5,070、5,136 ms，过 5 秒 1.4–2.7%）与 45,000 的候选都量在批次 D 的尾巴上（开始四到五个半小时之后）；新的一段上，45,000 低约 17%（S6 两轮 3,419–3,683 对候选 4,114–4,454 ms，被量的代码相同）；审查 C 在新的一段上单轮量的 50,000 是 3,968 / 4,154 ms，只有一轮，B5 要两轮，不足以判。收官时列给负责人，由负责人决定要不要补量两轮 50,000。`jobs.go` 的注释照 S3 量到的事实，不改。
- **673 `14808583` 的注释不写 7763 的倍数**（`fix-2.md`，审查 C2-M1）：界的注释写本机与 6975P-C 的倍数与"为 7763 的余量留着"，指向 P4 设计 3.13；7763 的数（6.93–6.97 秒，60,000 时 10.24 秒）在 3.13 与 10.4。
- **674 在原规模上再量取分支的头**（实施报告第 7 节第 7 条，主持者接受）：3.12 没写取哪个提交；取 `89d9c701`（前四个提交都在），被量的后台代码与 S3 的 `f1317bc3` 相同（`server/internal` 的生产代码只差常量与注释、`server.gen.go` 一行说明）；第一次在 `d544acbe` 上，与锁外的负载重叠、又撞上 `arrays` 基准，作废重量。

### 对主持者说明的核实

- **第 302 条的修法**：读 `page/domain/revert.go` 与 `page/app/rollback.go:70-125`，形状与 I3、移交的描述一致。初稿没有造过、没有跑。设计审查 A 在 domain 与整个程序上复现了形状，指出先失败的是 `rootHolds`、一次恢复只放回一个回收站项，并在原型上跑通了更小的改法；修订照它改（4.6.1）。
- **heavy.sh 的锁**：`5e8af5b5` 的 `docs/v0.1/claude_to_codex/tools/heavy.sh:8` 是 `L=${NW_HEAVY_LOCK:-${TMPDIR:-/tmp}/nervewiki-heavy.lock}`，与说明一致；已由主持者改好、提交（`8c247237`）。
- **`ci-wait.sh`**：`5e8af5b5` 的 `:13` 按提交取第一个运行，与审查 B 说的一致；已由主持者改为按工作流取、提交（`b54cd679`）。
- **第四份 413.5 秒**：这是收尾之前那两次 `main`（`07abd0f7`、`9e787bab`，都在 7763 上）的平均。收尾之后的两次（`05dac827`、`a057b6ad`）第四份是 339、285 秒（平均 312，都不在 7763 上）；第五份 427、423 秒（平均 425，都在 7763 上）（[M11 收尾审查](../M11-upgrade/reviews/M11-closeout-review.md)"守卫与分份的数"一节的表）。第 7 节"分份"与第 10 节照此写。
- **`TestRevertingARollback…` 在第八份**：`Makefile:296`、`:328` 都点了它的名（`5e8af5b5` 的行号；`194ca350` 上是 `:321`、`:365`）。
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
| 2 | 组织允许公开的容器包；"允许的 actions"包括 `docker/*`、`sigstore/cosign-installer`、`anchore/sbom-action` 这几个第三方的（`ci.yml` 今天只用 GitHub 自己的）；组织的 Actions 设置允许工作流申请 `packages: write`、`id-token: write`、`attestations: write`、`artifact-metadata: write`（I1-D19；审查 B 指出这几项可能本来不限，照实核对） | 照做 | 推 rc 之前（P5/S2 合并之后就可以）；演练在每次推送上用 `docker/*`（P5/S1 起）与 `anchore/sbom-action`（P5/S2 起），"允许的 actions"受限时那个 Step 的分支 CI 就红（P5 设计 2.2）。**2026-11-17 第 11 问**：先跑 P5/S1 的分支 CI 看，红在权限上再请负责人设；推 rc 之前仍照实核对。**P5/S1**：分支 CI 没有被"允许的 actions"挡住：`release.yml` 的演练用的 `docker/setup-buildx-action`、`docker/build-push-action` 每次都跑了（run 44073973297 起，[P5 设计](05-P5-deploy-release.md) 10.1）；`docker/login-action` 只在 tag 的 `build` 里、还没跑过，S2 的 `sigstore/cosign-installer`、`anchore/sbom-action` 与写权限也还没有，推 rc 之前仍要照实核对。**P5/S2**：分支 CI 上 `anchore/sbom-action` 没被挡住（`release.yml` 的 run 44451246873 起，[P5 设计](05-P5-deploy-release.md) 10.2）；`docker/login-action`、`sigstore/cosign-installer` 与写权限只在 tag 的作业里，推 rc 之前照实核对 |
| 3 | 给 `v*` 的 tag 加规则集，只有负责人与主持者能推（I1-D19） | 照做 | 推 rc 之前 |
| 4 | 开私密的漏洞报告（`SECURITY.md` 指向它）；Dependabot alerts 可以开，只管 Go 与 actions：GitHub 的依赖图读不出 pnpm 12 的锁文件，npm 一侧靠 `make vuln` 里的 `pnpm audit --prod`（I1-D19；设计审查 B 改） | 照做 | 私密的漏洞报告在 P5/S3 写 `SECURITY.md` 之前；alerts 随时。**2026-11-17 第 10 问**：两个都开，负责人做 |
| 5 | 把 GHCR 的包设为公开，**不能撤回**（I1-D19） | rc 核对之后 | 4.11 第 5 步：rc 核对之后、推 `v0.1.0` 之前 |
| 6 | Dependabot 的版本更新 PR 要不要开（移交 M0-P6 第 4 条写明由负责人定） | 不开；发布之前在 runner 上跑 `make outdated`（4.3） | M12 收尾之前（写 v0.2 的移交时） |
| 7 | 有没有 v0.1.0 之前就在跑、要升级到 v0.1.0 的实例（I1-D13） | — | **已确认（2026-11-17）**：没有（第 8 问）；不写开发版的附录、不量批次 H |
| 8 | 输入法清单（M4–M8 的 21 步）是否同样在发布之后验收（4.12） | 是：同 2026-10-13 的原话 | **已确认（2026-11-17）**：发布之后验收（第 9 问） |
| 9 | 罕见的 A 类（H1、24.1、T4、T2）怎样处理 | **已确认（2026-11-14）**：T2、T4 在 v0.1 修，H1、24.1 进 v0.2（第 5 问） | — |
| 10 | 只从 ARIA 推断的九项怎样处理 | **已确认（2026-11-14）**：并进发布之后的读屏验收（第 6 问）；其中 49 与 24.3 同一种问题，留在 v0.1 | — |
| 11 | 打开仓库的 immutable releases（P5 设计 2.1 第 14 条） | 打开：公开之后 tag 与附件都不能改，与"不重推同一个 tag"一致；流水线先草稿后公开，打开不打开都能用 | 推 rc 之前 |
| 12 | GHCR 的 `open-nerve` 下没有、也不要有手动推的 `nervewiki` 包：别的凭证推过的同名包，`GITHUB_TOKEN` 推不了（GitHub 文档；P5 设计 2.1 第 14 条） | 照做：第一次由 `release.yml` 推，包自动关联仓库 | 推 rc 之前 |

改了负责人原话或负责人可能想自己定的，在 M12 的汇报里列出：315（流数的上限与重置）、324（rc 的重推不再问）、331（MCP 地址进 v0.2）、312 里 49 留在 v0.1（比第 6 问所选多做一项）、322 里不打 `0.1`（I1 推荐打）。

## 12. Phase 进度表

| P | Phase 文档 | 状态 | 审查 | 合并 |
|---|---|---|---|---|
| P1 地基（S1、S2） | [01-P1](01-P1-foundation.md) | 完成（2026-11-16） | [设计](reviews/P1-design-review.md)、[S1](reviews/P1-S1-e2e-navigation-review.md)、[S2](reviews/P1-S2-toolchain-review.md)、[分配守卫的旁支](reviews/P1-prepared-alloc-review.md)、[帮手的旁支](reviews/P1-navigation-settle-review.md)、[来源守卫的旁支](reviews/P1-source-revert-deadline-review.md) | S1 `bbf3eb26`、S2 `4190bac0`、分配守卫的旁支 `ad0bf40c`、帮手的旁支 `4fe6e523`、来源守卫的旁支 `194ca350` |
| P2 网页的打磨（S1–S6） | [02-P2](02-P2-web-polish.md) | 完成（2026-11-20） | [设计](reviews/P2-design-review.md)、[S1](reviews/P2-S1-focus-and-shared-review.md)、[S2](reviews/P2-S2-text-review.md)、[S3](reviews/P2-S3-narrow-bdi-contrast-review.md)、[lint 忙的旁支](reviews/M12-lintbusy-await-expiry-review.md)、[S4](reviews/P2-S4-reading-view-review.md)、[S5](reviews/P2-S5-editor-assets-review.md)、[S6](reviews/P2-S6-versions-trash-history-search-review.md) | S1 `d1ade2ed`、S2 `6505c3bf`、S3 `e639b9f7`、lint 忙的旁支 `32431f25`、S4 `ad5d78b5`、S5 `22e9f2bd`、S6 `e4d16a0a` |
| P3 服务端的正确性与历史（S1–S3） | [03-P3](03-P3-server-correctness.md) | 完成（2026-11-17） | [设计](reviews/P3-design-review.md)、[S1](reviews/P3-S1-small-correctness-review.md)、[S2](reviews/P3-S2-nested-trash-roots-review.md)、[S3](reviews/P3-S3-transfer-jobs-review.md)、[等锁帮手的旁支](reviews/P3-lockwait-by-statement-review.md) | S1 `f3e5a6de`、S2 `46b6940f`、S3 `7696d8df`、等锁帮手的旁支 `5d782ff8` |
| P4 性能与内存实测（S1–S3，条件的 S4 起） | [04-P4](04-P4-performance.md)、[S4](04-P4-S4-page-tree.md)、[S5](04-P4-S5-editor-parse.md)、[S7](04-P4-S7-tree-writes.md) | 进行中（设计已审查；S1 已合并，2026-11-18；S2 已合并；条件的 S4 已合并，2026-11-17；条件的 S5 已合并，2026-11-17；S3 已合并；条件的 S6（`MaxJobItems` 降到 45,000）已合并，2026-11-21；条件的 S7（B4 的树写）设计已审查，2026-11-21，负责人 2026-11-21 第 13 问） | [设计](reviews/P4-design-review.md)、[S1](reviews/P4-S1-perf-tools-review.md)、[S2](reviews/P4-S2-browser-measures-review.md)、[S4 设计](reviews/P4-S4-design-review.md)、[S4](reviews/P4-S4-page-tree-review.md)、[S5 设计](reviews/P4-S5-design-review.md)、[S5](reviews/P4-S5-editor-parse-review.md)、[L1 的旁支](reviews/M12-e2e-l1-new-tab-review.md)、[提交期限的旁支](reviews/M12-submit-deadline-log-review.md)、[导入守卫的旁支](reviews/M12-ingest-request-deadline-review.md)、[S3](reviews/P4-S3-server-measures-review.md)、[S7 设计](reviews/P4-S7-design-review.md)、[S6](reviews/P4-S6-max-job-items-review.md) | S1 `f08dca69`、S2 `54a89127`、S4 `35501dec`、S5 `5e58e844`、L1 的旁支 `7ce00077`、提交期限的旁支 `4e309997`、导入守卫的旁支 `e635b957`、S3 `a8b51831`、S6 `（待填：合并号）` |
| P5 部署文档与发布流水线（S1–S3） | [05-P5](05-P5-deploy-release.md) | 进行中（设计已审查；S1 已合并，2026-11-18；S2 已合并） | [设计](reviews/P5-design-review.md)、[S1](reviews/P5-S1-version-pipeline-review.md)、[S2](reviews/P5-S2-publish-release-review.md)、[装浏览器的旁支](reviews/M12-e2e-browsers-cache-review.md) | S1 `3ef1501f`、S2 `24e7b316`、装浏览器的旁支 `105ff7c8` |
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
| 2026-11-15 | P1 的 S1 实施、审查与修复（实施 8 个提交，修复一轮 5 个，主持者另加 1 个只改注释的，一共 14 个；合并见第 12 节）：帮手 `e2e/fixtures/navigation.ts` 的 `goto`、`reload` 先等在途的接口请求、续期的锁（或租约）、这个文档有了给它令牌的答复，至多 10 秒；`watchPage` 登记在途的请求与登录页的落点；110 个文件机械地换上；门禁 `tools/e2e-navigation/check.mjs`（`make lint-web`）核对读到了帮手的模块、读 Playwright 默认的八种结尾、认三种引号；A15 九格，三种浏览器；第 50 条的一行（分支的 CI 上触发了一次）。与设计不同之处 10 条（帮手另认登录、注册的答复且先看锁再看记录，A6 的一个标签页先受监视，页面里的读至少 1 秒，第 5 格先断言路由的结局，落点按上一次主框架的地址去重，A15 九格，门禁的核对与读的文件，3.7 的"复原之后看不到"在本机做不出，那一行只在落到过 `/sign-in` 时写）；实施中定的几条没有给号，记在 P1 设计 3.15；决定 339 照实施改，连粗体，决定 342 的数法改为按 ` after a refresh answered 401` 数。审查（Important 0、Minor 4、Nit 9）：在这个文档里登录、注册时帮手不等锁，租约一支没有测试，文档稿的两处；修复一轮 5 个提交（头 `3c2425c1`）照改，变异改前活、改后死，分支 CI 43642007840 一次全绿；修复核对一轮三位都没有 Important、Minor，新 Nit 三条（数法、锚、倍数），一条由 `937df215` 改注释，其余在本提交。4.4、第 7 节 S1 一行、"总体设计的修订"第 12 条与决定 339、342 随之改；总体设计 13.4 第 3 条与第 15 节，移交 M5-polish 一行；README 的 e2e 一节加帮手一条，05-lessons 加设计的一句与审查的三条。P1 设计随之改头、第 0 节、2.3、3.2–3.7、3.14、第 4、5 节、6.1、7.1、7.2、7.4、第 8 节第 3、5、7 条，加 3.15 与 10.1。合并之后两次 `main` 的 e2e 记在 S2 的记录 | [P1 设计](01-P1-foundation.md) 3.15、10.1 与 [S1 的审查记录](reviews/P1-S1-e2e-navigation-review.md)（A 2M 1N、B 1M 2N、C 1M 7N；修复核对 A 1N、B 1N、C 2N） |
| 2026-11-15 | S1 合并 `bbf3eb26`，合并之后第一次 `main` 的 CI（run 43650412259）19 项第一次 attempt 全绿：八份的 Test 一步 253（EPYC 9V45）、285（EPYC 9V74）、422（EPYC 7763）、302（Xeon 6975P-C）、265（6975P-C）、372（7763）、328（7763）、236（6975P-C）秒，都在 450 之下，第三份在 7763 上最贴近；第五份这次在 6975P-C 上，决定 352 的 7763 那一次还没来；e2e 9:38（E2E 一步 8:08，565 个、1 个跳过），第 50 条那一行打出（waited 432 ms）。第二次是记合并号这个提交的 run，记在 S2 的记录 | [P1 设计](01-P1-foundation.md) 第 8 节第 1 条、7.5；决定 352 |
| 2026-11-15 | S1 合并之后第二次 `main`（记合并号的 `0da2fb5d`，run 43653071437）19 项第一次 attempt 全绿：八份的 Test 一步 403（7763）、261（6975P）、410（7763）、297（6975P）、254（9V45）、383（7763）、240（6975P）、236（6975P）秒；两次的平均 328、273、416、299.5、259.5、377.5、284、236，都在 450 之下，第三份最贴近；第五份两次都没落在 7763 上，决定 352 的那一次还没来；e2e 8:29（E2E 一步 7:11），第 50 条那一行打出（waited 213 ms）。这两次是 S2 的"之前"（P1 设计 7.5） | [P1 设计](01-P1-foundation.md) 第 8 节第 1 条、7.5、10.1；决定 352 |
| 2026-11-15 | P1 的 S2 实施、审查与修复（实施 12 个提交，修复一轮 6 个，一共 18 个；合并见第 12 节）：Go 1.27.4（两份 `go.mod`，Dockerfile 的 tag、摘要与构建时的断言，README；守卫 `TestTheImageBuildsWithTheModulesToolchain` 读整行的断言）；PostgreSQL 18.7 五处，守卫加 README；node（24.22.3）与 distroless 的摘要，setup-node 7.1.0、upload-artifact 7.0.2；`make vuln`、`make outdated`、`tools/outdated.sh` 与 `vuln.yml`，本机拒绝（判断与干活同一行，`outdated.sh` 自己也判）；`vuln.yml` 第一次报出的两个 Go 模块（只在测试里）与 dompurify 升了，katex 的公告进 `audit.ignore`（决定 353），之后为绿；第三、第五份各挪一个给第八份（决定 354）；`ci.yml` 只让竞态的九份取存 Go 的缓存，`vuln.yml` 不取不存（决定 355）。与设计不同之处 13 条，记在 P1 设计 3.16。审查（Important 1、Minor 4、Nit 11、Q 1）：合并之后 `main` 的缓存键会由不带竞态的 `short` 存下；守卫不核对报错里的版本；"取回 3 MB"说多了；`make -i` 绕过本机的拒绝；`outdated.sh` 没读到也写"没有过时的"。修复一轮 6 个提交（头 `80d76191`）照改，变异改前活、改后死，分支 CI 43681898393 一次全绿（冷的）；修复核对一轮三位都没有 Important、Minor，新 Nit 一条（反向对照没进 3.14 与 7.2），在本提交。4.3 的 `paths`、第 7 节 S2 一行与"分份"随之改，第 11 节加决定 353–355；总体设计 7.1、13.5 第 2 条、13.1 第 32 条与第 15 节；06-environment、05-lessons、03-project-background；移交 M0-P6-image-release 一行。P1 设计随之改头、第 0 节、2.3 的决定 346、3.8、3.9、3.10、3.12、3.13、3.14、6.2、7.1、7.2、7.4、第 8 节第 6、7 条，加 3.16 与 10.2，10.1 加 S1 合并之后的两次 `main`。S2 合并之后两次 `main` 的组合根照 P1 设计第 8 节第 1 条看 | [P1 设计](01-P1-foundation.md) 3.16、10.2 与 [S2 的审查记录](reviews/P1-S2-toolchain-review.md)（A 1M 3N、B 1I 1M 2N 1Q、C 2M 6N；修复核对 C 1N） |
| 2026-11-15 | S2 合并 `4190bac0`（19 个提交），合并之后第一次 `main` 的 CI run 43689756257：第一次 attempt 18 项绿，e2e 红在 S1 的 A15 第 3 格（Firefox：带逃生口离开之后页面仍登录着，旧文档的第一次续期没在下一个文档之前到 nervewiki），交给旁支 `m12-p1-navigation-settle`（决定 356）；第二次 attempt 只重跑 e2e，为绿（E2E 一步 8:36）。`vuln.yml` 在 `main` 上第一次跑，run 43689756196 为绿（`vuln` 0:39，`outdated` 0:46）。这一次 Go 的缓存是冷的（新的 Go 与 `go.sum`）：九个竞态份都 "Cache is not found"，第一个跑完的竞态份 `bootstrap-7` 存下键（268.81 MiB），`short` 没有存（决定 355 照预期）。八份的 Test 一步 431（7763）、408（7763）、317（6975P）、367（9V74）、438（7763）、346（9V74）、265（6975P）、300（6975P）秒，Test 减包 54–82 秒（冷的编译）。决定 352：第五份 S2 之后第一次落在 7763 上，438 秒，比 425 多 13 秒，全是冷的编译（包 356.9 秒，编译 81 秒）；暖的时候约 359 秒。两次的平均在记合并号这一次的 `main`（run 43693474739）那一行 | [P1 设计](01-P1-foundation.md) 10.2、第 8 节第 1 条；决定 352、355、356 |
| 2026-11-15 | 分配守卫的旁支 `m12-p1-prepared-alloc`（M10 的故事，在 M12 里发现；`2b65d4b9`、`c82cc3b6`、`19160e7a`；合并见第 12 节）：M10 的分配守卫 `TestPreparedSourceDoesNotCopyTextForComparison` 在 CI 的竞态一项上被 pgx 的后台读取器（8 KiB 缓冲）撑过 2 MiB（`m12-p1-navigation-settle` 的 run 43694312336）；界不变，按 pgx 的分配形状配对减去它的缓冲，取至多三次里最小，留在竞态一项（决定 357）。审查 A 0I 2M 2N、B 1I 1M、C 0I 3M 4N 2Q，修复一轮（B 的 I1：整类减去藏得住 8 KiB 块的复制，改为配对）；修复核对一轮三位都没有 Important、Minor，新 Nit 八条（去重六条），代码的两条由注释提交改，其余在本提交。第 11 节加决定 357，05-lessons 加两条，06-environment 加一条，P1 设计 10.2 末尾加一句 | 决定 357；[旁支的审查记录](reviews/P1-prepared-alloc-review.md) |
| 2026-11-15 | 分配守卫的旁支合并 `ad0bf40c` 之后第一次 `main`，run 43721644553：第一次 attempt 16 项绿；`server (guards-sources)` 红在 EPYC 7763 上，`TestSourceRevertAtTenThousandTakesItsTime` 三个状态的 sample2 都撞上 4 秒的请求期限，答 500（"API request deadline exceeded … transaction abandoned after index page"），sample0、1 是 3.21–3.65 秒；`e2e`、`image` 被跳过。同一棵树在分支的 run 43719110918 上绿（EPYC 9V45，sample2 2.31–2.50 秒）。第二次 attempt 只重跑失败的一项与被跳过的 `e2e`、`image`，全绿：guards-sources 又落在 EPYC 7763 上，sample2 3.48–3.54 秒；e2e 7:11（E2E 一步 5:44）。守卫开旁支 `m12-p1-source-revert-deadline` 修，第 11 节加决定 359 | 决定 359 |
| 2026-11-15 | S2 合并之后第二次 `main`（记合并号的 `5bbd2a56`，run 43693474739）19 项第一次 attempt 全绿：八份的 Test 一步 273（6975P）、223（9V45）、353（7763）、366（7763）、340（Intel）、257（6975P）、294（7763）、314（7763）秒；两次的平均 352、315.5、335、366.5、389、301.5、279.5、307，都在 450 之下，第五份最贴近；第五份这次不在 7763 上，决定 352 的第二次 7763 还没来（第一次是冷的 438 秒，暖的约 359）；e2e 7:17（E2E 一步 5:45）。S2 合并之后两次 `main` 的余量照 P1 设计第 8 节第 1 条成立 | [P1 设计](01-P1-foundation.md) 10.2、第 8 节第 1 条；决定 352 |
| 2026-11-15 | 帮手的旁支 `m12-p1-navigation-settle`（决定 356；七个提交：实施三个、修复一轮两个、改注释一个 `57d883d4`、合进 `main` 一个 `ea86d371`；合并见第 12 节）：帮手的 `settle` 读完页面为空时隔一个 `pollMs` 再看在途的请求，同一轮两样都空才放行；`followRequests` 在主框架提交它最后请求的那个文档时清掉留下的请求（Chromium 读完答复体才报结束，文档走了的两样都不报；Firefox 对下一个文档请求之后发的也不报），同文档的移动不清；A15 第 3 格在 Firefox 上确定（闸门每问各自发，离开之前等"Loading…"）；A15 加第 10–13 格，门禁的表 A15 ×3。旧文档在提交之前以同一地址 `replaceState` 的窄口不改，写进 P1 设计 3.17（决定 358）。审查（Important 0、Minor 3、Nit 3）：有路由时读完马上再看会漏；按地址配对与复位没有测试；照实改的清单不全。修复一轮 2 个提交（头 `5db08ef1`），分支 CI 43705519890 一次全绿，合进 `main` 之后 43721745639 一次全绿；修复核对一轮三位都没有 Important、Minor，新 Nit 六条（A-N1 = C-N4），一条由注释提交改，其余在本提交。4.4、第 7 节 S1 一行与决定 356 随之改，第 11 节加决定 358；第 12 节同时记分配守卫的旁支的合并号 `ad0bf40c`，它的审查记录补上文档提交的分支 CI；05-lessons 加五条。P1 设计随之改头、第 0 节、3.2、3.4、3.5、3.6、3.14、第 5 节、7.1、7.2、7.4、10.1，加 3.17，10.2 加 S2 合并之后的两次 `main` | [P1 设计](01-P1-foundation.md) 3.17、10.1 与[旁支的审查记录](reviews/P1-navigation-settle-review.md)（A 1M、B 1M、C 1M 3N；修复核对 A 2N、C 4N） |
| 2026-11-15 | 帮手的旁支合并 `4fe6e523` 之后第一次 `main`，run 43729199362：19 项第一次 attempt 全绿（e2e 7:23，E2E 一步 6:12；guards-sources 落在 EPYC 9V74 上，13:52）。帮手的两种偶发失败（决定 356）合并之后在 `main` 上还没有再见；源文件撤销的守卫的旁支（决定 359）仍在审查 | [帮手的旁支的审查记录](reviews/P1-navigation-settle-review.md)；决定 356、359 |
| 2026-11-16 | 来源守卫的旁支 `m12-p1-source-revert-deadline`（M10 的故事，在 M12 里发现；`83437b79`、`b0c80b74`、`80de075a`，合进 `main` 一个 `2308438e`；合并见第 12 节）：分配守卫的旁支合并之后第一次 `main`（run 43721644553 attempt 1）guards-sources 在 EPYC 7763 上红在 M10 的 `TestSourceRevertAtTenThousandTakesItsTime`，三个状态的 sample2 在 4.0 秒的请求期限答 500，`e2e`、`image` 被跳过。不是变慢：同一棵树的重跑也在 7763 上，3.54 秒；7763 上 M11 起 38 次，撤销的 sample2 一直只比 4 秒少 3–18%，失败那台每一项都最慢；9V74 与本机看不出与代码有关的台阶，7763 上 S2 之后高约 2–4%、分不出来源。4 秒的期限在 7.6 秒的界之下，起作用的是期限。恢复、撤销改在生产的 15 秒期限下跑，由不变的界来判；期限由界推出、与起应用的表对证；lint 的计时保存测试仍断言 4 秒；计时的请求经 `sendRequestBy` 用 30 秒的客户端（决定 359）。13.4 第 8 条的例外加这一个。审查 A 0I 3M 3N 1Q、B 0I 2M 2N、C 0I 5M 7N 2Q，修复一轮两个提交（B 的 M1、M2、N1、N2），分支 CI 43735796911 一次全绿；修复核对一轮三位都没有 Important、Minor，第一轮的都关上，新 Nit 七条（A 3、C 4）都是文档稿的措辞，在本提交。第 11 节的 359 补上旁支的做法、定因与理由，标题改为"来源的恢复与撤销的守卫"；总体设计 13.4 第 8 条与第 15 节、P1 设计的头与 10.2、05-lessons 两条、移交 M4-performance 的例外随之改 | 决定 359；[旁支的审查记录](reviews/P1-source-revert-deadline-review.md) |
| 2026-11-16 | 来源守卫的旁支合并 `194ca350` 之后第一次 `main`，run 43745380462：第一次 attempt 16 项绿，`server (race)` 红在 M1 的 pgtest 测试 `TestWaitForLockWaitsOnCountsTheRowsOfItsTable`（读超时，0.87 秒），`e2e`、`image` 被跳过；第二次 attempt 只重跑失败的项，全绿（race 14:44，e2e 9:39，E2E 一步 8:10）。guards-sources 在 EPYC 7763 上，新期限下恢复的 sample2 2.82–2.93 秒、撤销 3.31–3.38 秒，18 个样本都是 200 | 决定 359、360 |
| 2026-11-16 | P1 完成：两个 Step 都已合并（S1 `bbf3eb26`、S2 `4190bac0`），另有三条旁支（分配守卫 `m12-p1-prepared-alloc` 合并 `ad0bf40c`，决定 357；帮手 `m12-p1-navigation-settle` 合并 `4fe6e523`，决定 356、358；来源守卫 `m12-p1-source-revert-deadline` 合并 `194ca350`，决定 359）。P1 设计第 8 节的完成标准：第 1 条，两个 Step 合并之后各两次 `main` 的组合根每一份平均都在 450 秒之下（S1 最贴近的第三份 416，S2 最贴近的第五份 389）；决定 352 的第五份在 S2 合并之后第一次落在 EPYC 7763 上是 438 秒（冷的编译，暖的约 359），之后几次 `main` 的第五份 213–317 秒，没有两次落在 7763 上的平均过 450。第 2 条，第 3、5 节的测试与 7.2 的变异记在 10.1、10.2 与各旁支的记录。第 3 条，`vuln.yml` 在 S2 分支上为绿（10.2）；门禁报出逃生口只有 A15 的三格（第 1、3、11 格）；A15 十三格在三种浏览器里为绿（WebKit 跳过第 3 格）；"S1 合并之后 `main` 的 e2e 没有一次因为被跳回登录页而红"由收尾记录核对（决定 334），到这里 `main` 上 e2e 只红过一次（run 43689756257 第一次 attempt，A15 第 3 格在 Firefox 上该被登出而没有，是那一格的闸门写法，旁支的 `cb13cc9a` 修了，决定 356），不是被跳回登录页；另有两次因服务端的一项红而被跳过（43721644553、43745380462，决定 359、360）。第 4–7 条在各 Step 与旁支的文档提交里。主持者的决定编到 360（360：一次 pgtest 的读超时，只重跑、观察）。下一步：P2、P3 的设计已在写，P4/S1、P5/S1 的设计随后 | [P1 设计](01-P1-foundation.md) 第 8、10 节；两个 Step 与三条旁支的审查记录 |
| 2026-11-16 | P3 设计已审查（三份审查：A 0I3M6N、B 0I5M13N、C 3I5M9N2Q；三次修订，修订核对三轮：第 1 轮 C 1 条 Minor、三位共 11 条 Nit，第 2 轮 B 1 条 Minor，都改了，第 3 轮没有新的）。P3 设计 2.1 的修订落进本文：`Makefile` 的行号括注 `194ca350` 的；302 的 `needed` 是修正，性质测试分时刻装载、回滚的只数嵌套的根；快照放在四个用例里，从第一条属于答复的读起；MCP 超时的码不改，提交时跨过期限的另用一句；30.2 照 Unicode 的推导、管三种名字，标签块不去掉；429 沿用 `rate_limited`、网页不改；T3 的报告在任务结束时加、解码在适配器里；GBK 的包里恰好合法 UTF-8 的名字也按 GBK 读（决定 376，相对今天的回归写进已知限制）；T2 答 409、码的次序照今天的检查；T4 取第三种做法，以 SQL 种排队行的测试带上 River 的行；`reindex` 与切罗基文的撞键；9.6 的变异；与 P2 的重叠；5.1 的 `page.title_taken`。P3 的取舍 361–377 写进第 11 节 | [P3 设计](03-P3-server-correctness.md)、[P3 审查记录](reviews/P3-design-review.md) |
| 2026-11-16 | P2 设计已审查（三份审查：A 2I9M7N2Q、B 0I5M6N1Q、C 1I8M7N2Q；三次修订，修订核对三轮：第 1 轮 A 7M5N、B 2M1N、C 1M7N，第 2 轮 A 2N、B 1N、C 1M4N，都改了；第 3 轮 C 只有一条 Nit，主持者改了）。P2 设计 2.1 的修订落进本文：次序改为一次合并一个、只守硬依赖，S6 在页面树的结论之后开，另三处"按序合并"随之改；S1 一行与修订第 11 条照 S1 的焦点规则；"总体设计的修订"加六条；S4、S5 的部件加 server；9.1 的阅读视图与超宽两句；附录 A 加菜单与选项的高亮（S3）与 P2 推后的几处。P2 的取舍 378–429 写进第 11 节 | [P2 设计](02-P2-web-polish.md)、[P2 审查记录](reviews/P2-design-review.md) |
| 2026-11-16 | P5 设计已审查（三份审查：A 1I 8M 10N、B 6M 12N 2Q、C 1I 11M 11N 2Q；三次修订、两轮修订核对：第 1 轮 B 1 条、C 2 条 Minor，三位共 9 条 Nit；第 2 轮只有 3 条 Nit；都改了）。P5 设计 2.1 的修订落进本文：演练的用时改为分钟级；SBOM 与许可证在只读的作业里；Release 先草稿、两种架构上核对过才公开；版本号的 tag 不改指；核对与说明的 `gh attestation verify` 钉住身份、核对两份 SBOM；MCP 版本的短测试盖版本号；守卫是 Go 的短测试；actionlint 只在 runner 上（`make lint-actions`）；P5/S3 不等 P2；`0.1.1-dev` 的清单；syft 与 BuildKit 的版本；读 tag 的运行；负责人的设置加两项；发布物的名字与注解；流水线自己的错不出 `v0.1.1`；"总体设计的修订"第 6、7、9 条（第 14 节的 `NOTIFY` 与 SSE 两行归 P5/S3）；部署文档的数与 P4 对齐（小机器一档由 P4 另量）；compose 样例的库角色、附件目录与可信的代理；移交与已知限制的去处；闸门的写法与 tag 指的提交。第 11 节加 P5 的取舍 430–452，"需要负责人确认"加两项 | [P5 设计](05-P5-deploy-release.md)、[P5 审查记录](reviews/P5-design-review.md) |
| 2026-11-16 | P4 设计已审查（三份审查：A 3I10M6N、B 4I10M9N4Q、C 2I5M7N1Q；三次修订，修订核对三轮：第 1 轮 A 2M2N、B 4M6N、C 2M4N，第 2 轮 A 2N、B 1M3N、C 1M1N1Q，都改了；第 3 轮 B、C 只有 C 的一条 Nit，主持者改了）。P4 设计 2.1 的修订落进本文：B5 的满负载是 M8 的两种数据的形状，规则一层的统计量照 M8 的量法，硬期限只取被量的代码没再改过的那一段里的 7763、另并列两行推断，第 3 节第 6 条随之加硬期限一层；B1 的吞吐、1 小时与保存照原预算判，尾巴与体积对照 `308eafc6`，建索引时不重量、积压只记；推断改用本机对 7763 的实测之比（4.7.1 的五格与"判在哪台机器上"、4.7.3、决定 318 的一半，第 3 节第 6 条与 9.4 的"倍数"）；B2、B3 的堆峰值的读法；页面树的 T2 与深的形状；vault 经导入、压缩离线算、knip 经脚本；P4/S1 的 Makefile 只给 `test-go-costs-3` 加一行，lint 守卫的 worker 提到界之上（"总体设计的修订"加第 25 条：13.4 第 8 条随 P4/S1 改）；P4/S2 不等 P4/S1；机器时间重估。P4 的取舍 453–478 写进第 11 节 | [P4 设计](04-P4-performance.md)、[P4 审查记录](reviews/P4-design-review.md) |
| 2026-11-16 | P3 的 S2 实施、审查与修复（实施 5 个提交，修复第 1 轮 4 个，一共 9 个；合并见第 12 节）：撤销计划把被这个变更集删了、现在本身是回收站根的节点当作它自己的顶（`topOf`，五处构造计划传 `Nodes`），后台撤销的 `needed` 也看段内后面的放回，domain 的表十二格；两个性质测试扩开：后台撤销的造单独删的项与三层、数五种嵌套，20 个下限照约定全部重算（决定 479、480），回滚的撤销回滚的变更集、请求与后台轮流；组合根的 nested 两格撤销得了、改名 `TestRevertingARollbackPutsBackWhatTheAgentLeft`，另两格照决定 304 由 409 翻过来（种下的上下相套的两项、交错 20 的"往回"，后者是 4.6.1 的来源 3，不是任务自己的变更集）。回滚的性质测试的 agent 边走边删会留下真库没有的孤儿（别的种子基上 56,000 次里 15 次），改为先读整棵。审查没有 Important：`needed` 的"只放回到那个成员"没钉住；多层嵌套没有计数；实测的差与原因不对；交错 20 不是历史移交第 6 条的形状；漏了移交与 00-M11 的两处文档。修复第 1 轮 4 个只改测试的提交（头 `63cfcd9b`），变异整个重跑，分支 CI 43800625515 一次全绿；核对第 1 轮三位都没有 Important、Minor，新 Nit 4 条只在文档，在本提交。第 7 节"总体设计的修订"第 8、15 条落了，第 11 节加决定 479–482；总体设计 5.8 第 6 步、13.1 第 2 条与第 15 节；00-M11 第 302 条的括注；M8/P4、M9/P3 的后注（决定 481）；移交 M8-history 一行；05-lessons 四条。分份 554 个、145/50/119/120/60/15/30/15 不变；竞态一份的两个性质测试共多约 2 秒 | [P3 设计](03-P3-server-correctness.md) 3.22、10.2 与 [S2 的审查记录](reviews/P3-S2-nested-trash-roots-review.md)（A 1M 2N、B 2M 2N 1Q、C 2M 6N；核对第 1 轮 A 2N、B 1N、C 1N） |
| 2026-11-16 | P3 的 S1 实施、审查与修复（实施 8 个提交，修复第 1 轮 6 个、第 2 轮 4 个，一共 18 个，另两次合并 `main`；合并见第 12 节）：标题键对切罗基文幂等（`TitleKey`、`TitleKeyContext` 末尾把小写换成大写，`IndexKeyer` 原地换；`searchFold` 删掉，搜索与 grep 的键改调 `TitleKey`，逐码位与旧的相同，分词的版本不升）；标题 id 带上自动链接的标签与原始 HTML，与锚点相遇；标签列表不列 `/`；什么也不显示的显示名、工作区名、PAT 的名字答 `required`，这样的客户端名称算没有，照 Unicode 的推导；MCP `log` 的 `"/"` 是整本、`""` 答 `too_short`；MCP 调用超时的文字，读写事务的提交错误带标记、答"可能已保存"；日志与变更集的详情各在一个快照里（交错 `TestTheLogReadsOneMomentAcrossARevert`）；事件流每个凭证 4 条、第 5 条顶掉最老的（`superseded`），每个账户 64 条、再开答 429（`TestASignInsFifthStreamSupersedesItsOldest`）。审查没有 Important：标题 id 分不出自动链接的 `Label` 与 `URL`；S1-27 的替身收 `"/"`；提交在调用的期限之前超时的答 "the server failed"（设计 3.7 的推断，决定 483）；PAT 的凭证没有测试守（决定 484）；偏离 7 的理由不对。修复第 1 轮 6 个提交（头 `1f1d3d11`），分支 CI 43793058599 一次全绿；核对第 1 轮三位都没有 Important、Minor，新 Nit 4 条由修复第 2 轮 4 个提交改（头 `a7723228`，CI 43801996056 一次全绿），主持者核对（决定 486）；合并 `main` 的 `d1e90929`（`Makefile` 的冲突），CI 43814516991 一次全绿。第 7 节"总体设计的修订"第 10 条落了，第 11 节加决定 483–486，4.6.4 与决定 365、369 加括注；总体设计 13.1 第 28、30 条与第 15 节；README 的事件流与 MCP 两节；移交 M5-polish、M5-performance、M4-performance、M8-history 各一行；05-lessons 四条。分份 556 个、145/50/119/120/60/15/32/15，第七份多两个（竞态共约 4.6 秒）。S2 合并之后两次 `main`（`46b6940f` 的 run 43812781960、记合并号的 `41c5ac25` 的 run 43816198500）19 项第一次 attempt 全绿：八份的 Test 一步，第一次 284（6975P）、251（6975P）、269（6975P）、293（6975P）、280（9V74）、270（6975P）、237（6975P）、308（8370C）秒，第二次 362（8573C）、282（8573C）、249（9V45）、280（9V45）、275（9V74）、284（6975P）、233（6975P）、249（6975P）秒；两次的平均 323、266.5、259、286.5、277.5、277、235、278.5，都在 450 之下，第一份最贴近，第七、第八份都没落在 7763 上；竞态一份 532、544 秒；E2E 一步 6:18、6:06 | [P3 设计](03-P3-server-correctness.md) 3.21、10.1 与 [S1 的审查记录](reviews/P3-S1-small-correctness-review.md)（A 2M 2N、B 1M 1N、C 2M 3N；核对第 1 轮 A 1N、B 1N、C 2N） |
| 2026-11-16 | P4/S4（页面树的渲染）设计已审查（三份审查：A 0I5M4N、B 0I4M9N1Q、C 1I9M4N；两次修订，修订核对两轮：第 1 轮 A 4M2N、B 2M7N、C 1I1M3N，都改了；第 2 轮只有 A 的两条 Nit，改了）。只改怎样画、不改列什么：行便宜（`memo` 与原始值的属性、普通链接、一棵树一个菜单、对话框选了才挂），每个页的列表先画 100 行、其余每帧画完再加 400 行，不加虚拟列表；菜单保持模态（大头在模态，S4 不比今天慢），不模态是 v0.2 的候选；守卫是 1,000 页上的结构的断言。原型在平的一万页上 T1 约 0.19 秒、T2 约 0.21 秒（P4/S2 的改前 3.0、1.6 秒），实施时照 P4/S2 的方法再量。第 7 节"总体设计的修订"加第 26 条；P4/S4 的决定 487–505 写进第 11 节 | [P4/S4 设计](04-P4-S4-page-tree.md)、[P4/S4 设计的审查记录](reviews/P4-S4-design-review.md) |
| 2026-11-16 | P2 的 S1 实施、审查与修复（实施 10 个提交，修复第 1 轮 4 个、第 2 轮 1 个，一共 15 个，另一次合并 `main`；合并见第 12 节）：`ConfirmDialog` 发送中 `aria-disabled`、被拒之后焦点在"确认"，`aria-disabled` 的按钮变暗；`useForm` 只有横幅说的拒绝、焦点已丢时给横幅（`focusLost` 认空、`body` 与禁用的元素）；到达的焦点：迟挂上的标题让读者（`useArrivalWatch`），树打开的页带 `opened`、后退前进时标题从无人处取，随页离开的链接带 `arrived`，代替页的"页面不存在"从无人处取；退出登录之前先问这个标签页与同一登录的别的标签页（100 毫秒）有没有未保存的修改，三处共用 `useSignOut`；撤销对话框的描述说全；其余焦点与可访问名称的十来项。审查没有 Important：上传对话框里来源的重试，横幅不取焦点（决定 506）；邀请页与会话不可用页的"先问"没有测；换了查询的 `replace` 没接续 `offered`（决定 507）；新键两种语言的格；"三种浏览器在拒绝到达时焦点都在 `body`"不成立（Firefox、WebKit 让焦点留在禁用的按钮上）。修复第 1 轮 4 个提交（头 `137356d6`），分支 CI 43869362051 一次全绿；核对第 1 轮只有一条注释的 Nit，修复第 2 轮补上（`2d707feb`，主持者核对）；合并 `main` 的 `01283f3f` 没有冲突，门禁与分支 CI 全绿。e2e 十二格多 5.3 test-seconds。第 7 节"总体设计的修订"第 11、17、19 条落了（第 11 条的新文字照决定 509 改），第 11 节加决定 506–509 | [P2 设计](02-P2-web-polish.md) 10.1、[S1 记录](reviews/P2-S1-focus-and-shared-review.md)；决定 506–509 |
| 2026-11-16 | P4 的 S2（浏览器一侧的实测）实施、审查与修复（实施 5 个提交，修复第 1 轮 4 个、第 2 轮 2 个、第 3 轮 1 个，一共 12 个，另一次合并 `main`；只改 `e2e/perf/` 与它的配置；合并见第 12 节）：三份审查 A 0I1M4N、B 2I2M3N1Q、C 1I5M10N1Q；核对三轮（第 1 轮 A 1N、B 1M1N、C 3N，第 2 轮 B 1I，第 3 轮 B 1M，只关说法）。页面树 1 万页平的形状 T1 3.0 秒、T2 1.6 秒、其间长任务 1.4 秒，三格超（深、带附件全过；慢在渲染），改法在 P4/S4（决定 510），P2/S6 排在它之后；T3 挪到 T2 之前重量，深 212、带附件 211 ms（决定 515）。B9 超（每格新页、热身 2 次、量 20 次的中位，决定 512）：一串空格类三种每次解析约 81 ms，标题没闭与在最后一行闭合两种多行的引用式定义插一个字 86–90 ms，界 80；`<?` 约 75 不超；改法在条件的 Step P4/S5（决定 511）。附件列表的重读在 A 持全部页的情形上判：B 跟上传窗口一起停手时正式两轮 0 / 5、3 / 5，复核 1 / 5、0 / 5，第一次超、复核过，记成已知限制（决定 513）；只持第一页两轮 0 / 5、没有 429。任务列表的轮询、链接目标、5 万与 10 万页只记，10 万页的树让渲染进程崩溃（决定 514）；大图挪到 P4/S3（决定 518）。第 7 节"总体设计的修订"第 9 条的页面树一行落了，第 11 节加决定 510–518 | [P4 设计](04-P4-performance.md) 4.11、10.2、[S2 记录](reviews/P4-S2-browser-measures-review.md)；决定 510–518 |
| 2026-11-16 | P4/S5（编辑器的解析，B9）设计已审查（三份审查：A 0I3M2N、B 1I3M5N、C 3I5M9N1Q；四次修订，修订核对四轮：第 1 轮 A 1M1N、B 1M2N、C 2M2N，第 2 轮 A 1N、B 4N、C 1M3N，第 3 轮 C 1M，第 4 轮 C 1N，都改了）。编辑器自己的 Markdown 配置加三个行内解析器，把 lezer 的三处平方（一串空格；没收尾的 `<?` 与后面没有 `>` 的 `<`；网址自动链接的修剪与被拒）改成一串只扫一次，语法树逐节点不变；跨行的引用式定义另设上限 2,000 码元；`inlineLimit` 留 10,000。原型在 Chromium 里（每格新页、量 20 次）超的一串空格类三种约 81 → 0.2 ms、标题的两种定义插一个字 87–89 → 2.8–4.5 ms，找到的 `)` 结尾的网址约 188 → 1 ms；改后上限处最贵的是 `*_` 交错约 62 ms；没有一格比今天慢。第 7 节"总体设计的修订"加第 27 条；P4/S5 的决定 519–529 写进第 11 节 | [P4/S5 设计](04-P4-S5-editor-parse.md)、[P4/S5 设计的审查记录](reviews/P4-S5-design-review.md) |
| 2026-11-17 | P3 的 S3 实施、审查与修复（实施 4 个提交，修复第 1 轮 7 个，一共 11 个，另一次合并 `main`；合并见第 12 节）：启动时收拾上一个进程取到而没开始的任务（T4）：transfer 的五种任务设 `DiscardRunning`，`AtStart` 照种类跑 `lost`，`lost` 失败让启动失败，3.14.3 的六处种子带 River 持有的行（应用起来之后种的三处与排队行写成一条语句）；按人的队列上限（T2）：每人 5 个（各笔记本合计，含正在上传的导入），409 `transfer.too_many_jobs`、不带 `Retry-After`，先按人、再全局，回滚算进去、从不因此被拒；名称的编码（T3）：没有标志的名字先读第一个 0x7075，包里不是 UTF-8 的名字都按 GBK 读得出时整包按 GBK 读，报告列出 `name_unicode_path`、`name_gbk`，适配器的表 13 格、差分 13 个计数各带下限、模糊；决定 482 的提示去掉后半句（单独的提交，决定 530）。审查没有 Important：`unicodePath` 的三处边界没有种子钉住，后台开始在锁下的按人计数没有一格钉住；修复第 1 轮 7 个只改测试的提交（头 `cbda8435`），分支 CI 43948816167 一次全绿；核对第 1 轮新 Nit 2 条只在文档，在本提交。第 7 节"总体设计的修订"加第 28 条（落了），第 11 节加决定 530–532；总体设计 13.1 第 4、23 条与第 15 节；`README.md` 的两句（决定 531）；M8/P4 的后注（决定 481 的先例）；移交 M7-transfer 一行；05-lessons 两条。分份 558 个、145/50/119/120/60/15/34/15；第七份竞态下多约 4.4 秒 | [P3 设计](03-P3-server-correctness.md) 3.23、10.3 与 [S3 的审查记录](reviews/P3-S3-transfer-jobs-review.md)（A 4N、B 1M 3N、C 1M 3N；核对第 1 轮 A 1N、B 1N） |
| 2026-11-17 | P3 完成：三个 Step 都已合并（S1 `f3e5a6de`、S2 `46b6940f`、S3 `7696d8df`），决定 361–377、479–482、530–532。P3 设计第 8 节的完成标准逐条：第 1 条，每个 Step 的提交都在、各自单独为绿，7.4 的闸门与分支的 CI 都绿（各 Step 的记录）；第 2 条，7.3 各 Step 的变异先绿后红、等价的写明（10.1–10.3 与三份记录）；第 3 条，三个 Step 都经三份代码审查与修复核对，对设计的修正在 3.21–3.23；第 4 条，第 8 节的新文字随各 Step 的文档提交落了（第 1–10、12、13 条各标已落，第 11 条是本文第 12 节的进度）。S2 另：23 个下限在测试里，整个程序的 nested 两格在 S2 合并之后的 `main` 上一直绿（此后各次 `main` 的 CI 第一次 attempt 都绿）。S3 另：3.14.3 的六处种子都改了，第 4 条的反向对照 5 次 4 红，点名的四个测试 `-count=20` 80 次全过（10.3）。S3 合并之后两次 `main`（run 43978501165、43982677358）组合根八份 Test 一步的平均：第一份 282.5 秒、第二份 254.5、第三份 326、第四份 341、第五份 290、第六份 275.5、第七份 238、第八份 292.5，都在 450 秒之下；竞态一份 9:18、9:13；e2e 的 E2E 一步 5:58、6:13。S3 文档提交的分支 CI 里竞态一份 Test 14:38，比此前约 9 分慢，原因没有查，合并之后的两次回到约 9 分。 |
| 2026-11-17 | P4 的 S4（页面树的渲染）实施、审查与修复（实施 4 个提交，修复第 1 轮 3 个，一共 7 个，另两次合并 `main`；合并见第 12 节）：三份审查 A 0I2M2N、B 0I1M3N、C 0I2M5N1Q；核对一轮（A 1N、B 1N、C 3N，都只关文档的说法与一行注释）。行便宜（`memo` 与原始值的属性、普通链接、一棵树一个菜单、对话框选了才挂），每个页的列表先画 100 行、其余每帧画完再加 400 行；平的形状（1 万页）T1 3,015 → 178 ms、T2 1,609 → 191 ms、T2 的最长长任务 1,429 → 94 ms，约 2.4 秒全部行进 DOM（其间最长的长任务 80 ms），堆 604 → 80 MB；深与带附件不慢，T3 212 → 30 ms。再量发现"移到……"慢了一倍，加提交修了（决定 533）；画全之后改一次树 `PageList` 渲染两遍，不改（534）；与锁外的负载重叠的补量作废重量（535）；13.2 的新条取第 30 条（536）。变异 55 条抓 53（没抓的是设计写明不守的两条），6.2 的六条反向对照照期望 | [P4/S4 设计](04-P4-S4-page-tree.md)第 10 节、[P4 设计](04-P4-performance.md) 10.2、[S4 的审查记录](reviews/P4-S4-page-tree-review.md)；决定 533–537 |
| 2026-11-17 | P4 的 S5（编辑器的解析，B9）实施、审查与修复（实施 3 个提交，修复第 1 轮 4 个，核对第 1 轮 1 个，一共 8 个，另一次合并 `main`；合并见第 12 节）：编辑器自己的 Markdown 配置加三个行内解析器（`linearScans`：一串空格；没收尾的 `<?` 与后面没有 `>` 的 `<`；`www.`、`http(s)://` 自动链接的括号只数一遍、被拒的记下），语法树逐节点不变；跨行的引用式定义另设上限 `referenceLimit = 2_000`。守卫：关掉上限的十七种（实施时加"一串空格直到正文末尾"）、lezer 自己的平方不加重的四格（过界时再量一次，决定 538）、读码元的次数、编辑器带着三个扫描、2 万篇随机文档加 61 篇手写的差分。在原规模上再量（今天与 S5 轮流各两轮）：每一格都不比今天慢、都 ≤ 80 ms，今天超的都下来了（一串空格类 85.6–87.2 → 0.1–0.3 ms，两种标题的定义插一个字 90.3–93.2 → 3.0–4.6，`)` 网址一次解析 199–212 → 1.3）；上限处最贵的是 `*_` 约 62–67 ms（决定 539）；B9 过了。三份审查 A 0I1M1N、B 0I2M1N、C 0I2M5N；核对第 1 轮 A 1N，B、C 没有新的。分支 CI 每次第一次全绿；`markdown.test.ts` 在 CI 上 2.5 → 7.3–7.9 s，Test 一步没有变长。第 7 节"总体设计的修订"第 27 条落了，第 8 节加网页一项，第 11 节加决定 538–540；总体设计 13.2 第 25 条、13.5 第 5 条与第 15 节；移交 M4-performance 一行；05-lessons 两条 | [P4/S5 设计](04-P4-S5-editor-parse.md)第 10 节、[P4 设计](04-P4-performance.md) 10.2 与 [S5 的审查记录](reviews/P4-S5-editor-parse-review.md)（A 1M 1N、B 2M 1N、C 2M 5N；核对第 1 轮 A 1N）；决定 538–540 |
| 2026-11-17 | 负责人答了五问（第 11 节"2026-11-17：L1 的改测法与 P4/S3、P5 之前的四问"，第 7–11 问，都选推荐）：L1 采用改测法、取代 M11 第 277 条；没有 v0.1.0 之前的实例，P4/S3 不量批次 H、不写开发版的附录；输入法清单发布之后验收；私密漏洞报告与 Dependabot alerts 由负责人开；"允许的 actions"先看 P5/S1 的分支 CI。"需要负责人确认"第 2、4、7、8 项随之改，[P4 设计](04-P4-performance.md) 2.2 第 1 条标已确认。同一提交给 P4/S5 的合并号 `5e58e844` 与它的记录的 CI | 第 11 节 |
| 2026-11-17 | L1 的旁支 `m12-e2e-l1-new-tab`（M6 的故事，在 M12 里发现；`12e868bf`、`1b01b1d9`、`d716d9a9`，只改注释的 `5ea4ade0`，合进 `main` 一个 `56537688`；合并见第 12 节）：P4/S4 合并 `main` 之后的分支 CI（run 43979118856）第一次 attempt，e2e 只红在 L1 (page)：Ctrl 点链接之后 `waitForEvent("page")` 等满 30 秒；`ci.yml` 自 2026-10-18 起 400 次里第一次 attempt 红过 3 次（一次在 `main` 上），重跑都绿；本机约 25 次一次，P2/S1 前后一样。核实的：标签页开了、载入了页面，Chromium 没为它的文档发 `Page.frameNavigated` 且 `Page.getFrameTree` 答 url 为 `""` 的初始文档时，Playwright 1.63 不带时限地等、不报这个页（审查 B 的协议日志 200 次：红 13/13，绿 0/187；M11/P2/S3 已查到，M11 决定 277），1.62.1 与 1.65 alpha 同样；同一修订的完整版 Chromium 200 次 0 红。推断的：headless shell 里标签页的导航与 DevTools 自动接上之间的竞态。改法只改测试：L1 在 window 上冒泡阶段最后拿到点击，核对没被取消、带修饰键、地址是真的、不是下载，再取消，地址在 `anotherTab` 开的标签页里打开；阅读视图的链接的单元测试记下每次点击的 `defaultPrevented`；e2e 不再看浏览器真的开了标签页。审查 A 0I 1M 2N 1Q、B 0I 1M 1N 1Q、C 0I 2M 4N 2Q，修复一轮两个提交；修复核对一轮三位都没有 Important、Minor，新 Nit 两条，一条由注释提交改，一条在记录的"更正"。门禁：改后 L1 (page) Chromium 200/200（实施与修复一轮各一次，审查 C 另一次），`make e2e` 576 过、1 跳过；分支 CI 43989817910、44001727074 都一次全绿。负责人 2026-11-17 第 7 问取代 M11 第 277 条；第 11 节加决定 541–547；第 10 节"e2e 的偶发挡发布"一行、05-lessons 第 6 节一条、P1 设计第 1 节"页面的监视"与决定 339 的后注随之改 | 决定 541–547；[旁支的审查记录](reviews/M12-e2e-l1-new-tab-review.md) |
| 2026-11-18 | 等锁帮手的旁支 `m12-lockwait-by-statement`（P3/S1 的测试，另改 M10 的一格；`7b3ac149`、`3d61fc8e`，修复一轮 `aee229af`、`974c7fa2`，只改注释的 `5fb535a8`、`9f322c1d`；合并见第 12 节）：旁支 `m12-submit-deadline-log` 的 `ad68ca6a` 的分支 CI（run 44078438315）第一次 attempt，bootstrap-7 红在 P3/S1 的 `TestTheLogReadsOneMomentAcrossARevert/MCP's_log`（读看到了 R），那个提交编译不到这个测试；只重跑失败的作业，为绿。核实的：`pgtest.WaitForTableLockWaits` 数任何在等 `page_revisions` 的后端，服务起来之后几秒之内 River 跑 `RunOnStart` 的清理，它 18 条语句里有 5 条也等这张表；本机改前没有负载 750 次红 3 次（`main` 450 次红 2 次，带探针的 300 次红 1 次），带负载 300 次 0 次，唯一带诊断的那一次写 R 时等着的只有清理的 `PurgeTrimmedChangesets`、读还在快照之前；一个先等上锁的普通 `SELECT` 作外来者，改前 100 格红 100 格（审查 B 98 格），改后 100 格全绿。推断的：CI 那一次与本机另两次也是清理。答复在一个快照里，不是产品的问题（READ COMMITTED 的产品变异两个测试都红）。改法只改测试：`pgtest.WaitForTableLockWaitsBy` 只数在跑 sqlc 的某条语句的后端，五格各等读停下的那一条（四格 `ChangesetFacts`，带路径的日志 `ListChangesetsOfPage`），不加长等待；M10 的 `TestVirtualRead/rename_snapshot` 同样等 `ChangesetFacts`（去掉过滤时它照绿，靠的是改名比读慢）；`asset_blobs` 的十处已由 `transferTeam` 先等完启动的任务，不改。改后本机 450 次 0 红。审查 A 0I 0M 2N、B 0I 1M 1N 1Q、C 0I 0M 4N 1Q，修复一轮两个提交；修复核对一轮三位都没有 Important、Minor，新 Nit 四条（去重三条），由两个只改注释的提交改，已推的说明的不准在记录的"更正"。门禁：两个组合根测试 `-race` 实施 100/100、修复一轮 60/60，`make test-go` 185 个包；分支 CI 44163333532、44176852719 都一次全绿。第 11 节加决定 548–554；05-lessons 第 3 节一条；总体设计 13.4 第 4 条与第 15 节；P3 设计 3.8.3 第 3 步与 P3/S1 的记录的后注 | 决定 548–554；[旁支的审查记录](reviews/P3-lockwait-by-statement-review.md) |
| 2026-11-18 | P5 的 S1（版本号与流水线的构建）实施、审查与修复（实施 9 个提交，修复第 1 轮 8 个、第 2 轮 7 个，一共 24 个，另一次合并 `main`；合并见第 12 节）：版本号的默认值五处（`buildinfo.go`、`Makefile`、Dockerfile 的两个 `ARG`、README）由 `TestTheDefaultVersionIsOneEverywhere` 核对；MCP 的 `serverInfo` 取 `buildinfo` 的版本，`make test-go-short` 以 `0.0.0-short` 盖测试的版本号；镜像加 `revision`、`created`、`description` 三个 label，`make image` 传它们、git 读不出时停下，`image-smoke.sh` 逐字核对；`ci.yml` 只在分支上跑，actionlint 由 `make lint-actions` 只在 `costs-1` 一份里跑；闸门 `deploy/release-gate.sh` 与说明 `deploy/release-notes.sh`；`release.yml` 在每次推送上以两种原生 runner 只读地演练构建与 image-smoke，tag 上闸门之后按摘要推送、拉回来 smoke；守卫读三份工作流（写权限只在 tag 的作业里，`run:` 里只许 `${{ matrix.* }}`、矩阵写死，actionlint 那一步在，跑 smoke 的步骤不跳过，演练不引用 secret、不登录）。三份审查 A 0I1M3N、B 0I2M4N、C 0I4M5N（Minor 去重 5 条）；核对两轮（第 1 轮 A 2N、B 1M5N、C 3N，第 2 轮 A 1N、B 2N、C 1N）。分支 CI 每次推送第一次 attempt：`ci.yml` 19 项与 `release.yml` 两个演练全绿（第 5 个提交红在 `make lint-actions` 自己的 `-format`，下一个提交改了）；演练 amd64 1:18–2:21、arm64 1:21–1:26，Lint workflows 2–3 秒；`docker/*` 没被"允许的 actions"挡住；没有推任何 tag、镜像，没有建 Release。第 7 节"总体设计的修订"第 6 条落了前一半，第 11 节加决定 555–563，"需要负责人确认"第 2 项记下 S1 的分支 CI；总体设计 13.5 第 6 条与第 15 节；06-environment；移交 M0-P6-image-release 一行 | [P5 设计](05-P5-deploy-release.md) 3.24、10.1 与 [S1 的审查记录](reviews/P5-S1-version-pipeline-review.md)（A 1M 3N、B 2M 4N、C 4M 5N；核对第 1 轮 A 2N、B 1M 5N、C 3N，第 2 轮 A 1N、B 2N、C 1N）；决定 555–563 |
| 2026-11-18 | 提交期限的旁支 `m12-submit-deadline-log`（M11 的故事，在 M12 里发现；`ad68ca6a`、`203fc064`，修复第 1 轮 `1f94312b`、`75c38196`，第 2 轮 `9845aa7e`；合并见第 12 节）：P4/S5 合并之后第一次 `main`（run 44022740779）第一次 attempt，`server (short)` 红在 M11 的 `TestTheSubmissionsDeadline/the_request_expired`，告警行的 `timeout_ms` 53 > 50；只重跑失败的作业，为绿。核实的：`timeout_ms` 是期限减提交的时刻；请求先到期的格，期限取自 `WithTimeout` 的真实时钟，假时钟却在造替身时就读了，两次取时之间 runner 停了 3.00–3.32 ms（由失败日志的 UUIDv7 与 53 定出），同样写法的另三格同因；COMMIT 两格 50 ms 的真实期限也从早读的钟起算；生产先定期限、后读钟，没有错（审查 A 沿代码核实）。推断的：停顿的原因（GC、调度或虚拟机被抢）。改法只改测试：两个循环都在 `Execute` 的前一行读假时钟；`timeout_ms`、`elapsed_ms` 断言由测试自己的时刻与请求的期限算出的精确值，行的时刻须等于测试的；请求先到期的格另断言钟自己的读数离期限不超过请求的超时，读钟的先后改回就红，不必等停顿。改前的探针四格 20/20 红、改后全绿；读钟之前睡 60 ms，COMMIT 两格改前 5/5 红、改后绿；精确值抓住了界放过的记 0、少记 1 ms、取整，以及挪动时刻与 `elapsed_ms` 记 0。读钟之后停 50 ms 以上六格仍红，是 50 ms 期限本身的余量，不改。审查 A 0I 0M 2N、B 0I 2M 3N、C 0I 2M 3N 1Q，修复两轮三个提交；核对两轮（第 1 轮 A 1M 1N、B 1N、C 2N，第 2 轮只请 A，没有新的）。四次分支 CI：44078438315 第一次 attempt 红在 bootstrap-7（P3/S1 的交错，交等锁帮手的旁支，决定 548），44163179005 第一次 attempt 红在 costs-3（M8 的代价守卫，EPYC 9V74，记成观察的偶发），都只重跑为绿；44175820202、44191830725 都一次全绿。第 11 节加决定 564–571；第 12 节 P4 一行、05-lessons 第 2、7 节各一条随之改 | 决定 564–571；[旁支的审查记录](reviews/M12-submit-deadline-log-review.md) |
| 2026-11-18 | P4 的 S1（性能实测的 Go 设施）实施、审查与修复（实施 8 个提交，修复第 1 轮 8 个、第 2 轮 3 个，核对第 2 轮之后只加测试的 1 个，一共 20 个，另一次合并 `main`；合并见第 12 节）：B1 的合成搜索语料（词表由仓库的文字数出，进 `testdata`；非重类的查询词确切地植入，重类的至少到 5%；三个测试在 `test-go-costs-3` 一行，两个不连库的也在 `short`）；升级的发起、检查、批准与 lint 摘要四个守卫和按页的日志的规模开关（手跑时只记不判界，客户端与写超时等得比期限久；下限 2 万页与 10 万个变更集）；lint 守卫的请求 12 秒、worker 6 秒（决定 475）；回收站两个基准的档位开关；`server/tools/perf/`：`stats`、`sample`（`/proc` 与 cgroup，逐行另记 `anon`、`shmem`、`file`）、`gctrace`（拼回被插断的行，认运行时的强制 GC）、`vault`、`perf prepare`、六个驱动（到期只停发，在途的至多等 60 秒，截断与失败分开数）、`stack.sh`（一轮一个目录）。试跑：每个驱动以最小的参数对真的 serve 各跑一次，四个守卫以 2 万页各一次，语料 1,000 页喂给今天与 `308eafc6` 的 `BenchmarkSearch` 各一次，全部 rc=0；试跑找到的四处与审查找到的改在驱动里。重类的四个词只在约 5% 的页（`中国`、`ai`、`kernel` 多是补放的，`linux` 自然约 5.3%），交 S3 判读。三份审查 A 0I1M1N1Q、B 0I6M5N1Q、C 0I4M4N1Q；核对两轮（第 1 轮 A 2N、B 2M4N、C 2M4N，第 2 轮 B 1N、C 3N）。变异：实施 58 条（56 条抓到，2 条等价的核对删了），修复第 1 轮 27 条、第 2 轮 15 条，起初活的都补了测试。分支 CI 三次第一次 attempt 19 项全绿；`costs-3` 三次落在三种机型上（EPYC 9V45、Xeon Platinum 8370C、8573C），新一行 12.2–16.4 秒，7763 上的数等合并之后两次 `main`。第 7 节"总体设计的修订"第 25 条落了；4.7.4 两句照实改；第 11 节加决定 572–579；总体设计 13.4 第 8 条与第 15 节；移交 M4-performance 一行 | [P4 设计](04-P4-performance.md) 3.10、10.1 与 [S1 的审查记录](reviews/P4-S1-perf-tools-review.md)（A 1M 1N 1Q、B 6M 5N 1Q、C 4M 4N 1Q；核对第 1 轮 A 2N、B 2M 4N、C 2M 4N，第 2 轮 B 1N、C 3N）；决定 572–579 |
| 2026-11-18 | 导入守卫的旁支 `m12-ingest-request-deadline`（M10 的故事，在 M12 里发现；`8fd158fd`，修复第 1 轮 `a1b7b7e5`；合并见第 12 节）：`5154e0de` 的 `main`（run 44232764121）第一次 attempt，`server (guards-sources)` 在 AMD EPYC 9V45 上红在 M10 的 `TestIngestOfMoreThanBatchSourcesTakesItsTime`：401 个来源的单个 REST 改名在测试配置的 4 秒请求期限答 500；只重跑失败的作业，为绿。核实的：产品没有变慢，这一格也不贴着 4 秒：7763 上 64 次最慢 2.99 秒，样本多的四种机型在 S2 前后差不到 0.02 秒；失败那一作业整份慢 1.7–3.2 倍，再前一次 `main` 的 guards-core 也在 9V45 上慢约 2 倍；M10 设计这一格由 4 秒的请求判，传进去的 11 秒自 `6d1549fc` 起是死参数。推断的：那段时间的 9V45 慢了。改在生产的 15 秒下、由 11 秒判整个请求（界 4 → 11 秒），期限由界推出、在起应用之前对证，五个额外形状都经它；计时的改名用 30 秒的客户端（决定 580–582）。审查 A 0I 1M 2N 1Q、B 0I 2M 3N 1Q、C 0I 1M 5N 2Q，修复第 1 轮一个提交；核对第 1 轮 A 0/0/0、B 0/0/2、C 0/0/2，不开第二轮（决定 583）。分支 CI 44242224718 一次全绿；44254273872 第一次 attempt 18 项绿，e2e 卡在装浏览器，只重跑 e2e：第二次 attempt 为绿（e2e 6:59：装浏览器 0:38，E2E 5:54）。13.4 第 8 条与第 15 节、05-lessons 两个子条、移交 M4-performance 随之改 | 决定 580–583；[旁支的审查记录](reviews/M12-ingest-request-deadline-review.md) |
| 2026-11-18 | P2 的 S2 实施、审查与修复（实施 10 个提交，修复第 1 轮 6 个，一共 16 个，另一次合并 `main`；合并见第 12 节）：英文的单复数（10 个 `*One` 键）；根下一律"笔记本的顶层"，根下的同名页写作"Notes（笔记本的顶层）"（决定 584）；英文的撤销一律 undo，被清理说"部分或全部"，别处已撤销之后再撤销的冲突先等这次改动的重读再说；回收站、预览与历史一件事只说一遍、照实说，H4 的两句；勾了会让任务项消失、附件写法的未建链接、附件的长名、单页的导出、失锁的编辑器各自说清；比较只差末尾换行时说出来；搜索的提示、"继续查找"与索引在建照最新一页；大小写 KiB、MiB；来源规则的说明不用行话；两份 `AGENTS` 模板说出升级期间的 `upgrade`。i18n 新加 23 个键、删 2 个，改文字的 en 22 个、zh-CN 27 个。审查没有 Important：替身的任务项与服务端在一半写法上不同（决定 585）；树没读到时导出仍说子页，没有一格守。三份审查 A 0I0M5N2Q、B 0I1M2N、C 0I1M6N1Q，修复第 1 轮 6 个提交（头 `62e40b25`）；核对第 1 轮 A 0/0/1、B 0/0/1、C 0/0/2，不开第二轮（决定 588）。变异：实施 114 条，活下来的五句中文补格之后都抓到；修复第 1 轮 15 条都抓到。实施门禁里 M10 的 `TestLintBusyReturnsBeforeCleanup` 红过一次，是那个测试的先后假设，另开旁支修（决定 586）。分支 CI：run 44242250765 的 e2e 四次卡在装浏览器、第五次绿；修复的 44266671504、合并 `main` 的 44279300477 第一次全绿。e2e 改到的五格 19.30 → 19.27 秒。第 7 节"总体设计的修订"第 20 条落了；总体设计 13.2 第 17 条、3.6 的 5 MiB 与第 15 节；移交 M5-polish、M8-history 各一行；05-lessons 两条；第 11 节加决定 584–588 | [P2 设计](02-P2-web-polish.md) 10.2、[S2 记录](reviews/P2-S2-text-review.md)；决定 584–588 |
| 2026-11-18 | P2 的 S3 实施、审查与修复（实施 6 个提交，修复第 1 轮 4 个，一共 10 个，另一次合并 `main`；合并见第 12 节）：名字进 `bdi`（共用件 `isolate`、`Isolated`，五处拼句）；历史与变更集页、长标题与它的对话框在 375 px 不超宽，新 Playwright 项目 `webkit-narrow` 只在 WebKit 上再跑 `@narrow` 的格（MC3、PG2、SE1），`tools/e2e-projects/check.mjs` 由 `make lint-web` 核对（决定 592）；焦点环、菜单与选项的当前项、次要文字与暗色 destructive 按钮悬停的对比度（决定 591）；搜索命中加粗到 700；33.1 在 Playwright 的 WebKit 26.6 上复现（页宽 704），`NativeSelect` 带 `contain-paint`（决定 589），WebKit 的下拉没有 3 px 的环进 v0.2、真的 Safari 上的两项交 P5/S3（决定 590）。审查没有 Important：三份审查 A 0I0M2N1Q、B 0I1M1N1Q、C 0I2M3N，修复第 1 轮 4 个提交（头 `f5d1220a`）；核对第 1 轮 A 0/0/0、B 0/0/0、C 0/0/1，不开第二轮（决定 594）。变异：实施 52 条（vitest 15、e2e 37），50 条抓到，一条等价、一条设计预先写明走不到；修复第 1 轮的反向对照都红。分支 CI：run 44252268978、44277971443 的 e2e 都卡在装浏览器一次、只重跑 e2e 绿；合并 `main` 的 44291042215 第一次 attempt 18 项绿；e2e 整 20 分钟都在装 Playwright 的浏览器（apt 的镜像慢），被时限取消，E2E 一步没有开始；只重跑 e2e，第 2 次绿，19 项全绿。e2e 多约 21.6 test-seconds（新的 `webkit-narrow` 三格 10.9 秒）。第 7 节"总体设计的修订"第 21 条落了（第 31 条，决定 593）；4.5、9.1 与附录 A 的 33.1；总体设计 13.2 第 31 条、13.4 第 3 条与第 15 节；移交 M5-polish 一行；05-lessons 两条；P5 设计 3.19 的人工入口；第 11 节加决定 589–594 | [P2 设计](02-P2-web-polish.md) 10.3、[S3 记录](reviews/P2-S3-narrow-bdi-contrast-review.md)；决定 589–594 |
| 2026-11-18 | lint 忙的测试的旁支 `m12-lintbusy-await-expiry`（M10 的故事，在 M12 里发现；`7abfd6ef`，修复第 1 轮 `d8e664b3`、`9079f9af`；合并见第 12 节）：P2/S2 推送之前本机 `make test`（头 `4c9f1212`，竞态检测，组合根 `-j6`），第 4 份红在 M10 的 `TestLintBusyReturnsBeforeCleanup` `:427`，新的同键调用者加入了已过期的 flight 1；单跑十次与整个 `make test` 重跑都过（决定 586）。核实的（P2/S2 审查 C 的 P2S2C-Q1，实施者与审查 B 复做）：过期由 `context.AfterFunc` 在另一个 goroutine 里忘掉 flight，测试等到 `pg_sleep` 被取消就让 bob 发起，两者没有先后；`expire` 开头睡 300 ms，改前确定地红、信息一字不差。`expire` 平常在 0.1 ms 之内，测试那一段 1.3–5 ms（审查 A、C 量的）。推断的：那一次是 `expire` 在负载下排晚，窗口到了毫秒级。产品与 `Runs` 的注释相符（窗口里来的调用者多一次 503），不改。改法只改测试，不加长等待：bob 发起之前等 `RunExpired`；carol 的跟随者发起之前等旧 worker 的 `RunFinished`（S61 第三种变异原来在 `finish` 慢时活下来，审查 A）；末尾的连接数限时轮询（被取消的连接由 puddle 在另一个 goroutine 里销毁，审查 B 在运行时看到）。同类：26 份交错测试（lint 一份整份读，其余按 grep 逐处读）与用同一个 `Runs` 的组合根测试；transfer 心跳的取消（`cancelSeen` 7 处调用与取消运行中的导入）是同一类，审查 B 拖慢 `stop` 三个测试全红，只改测试修不了，查明、不改，M12 收尾之前红一次就开旁支加观察点（决定 596）。审查 A 0I 2M、B 0I 2M 2N、C 0I 2M 4N 2Q，修复第 1 轮两个提交；核对第 1 轮 A 0/0/1、B 0/0/0、C 0/0/5，不开第二轮（决定 597）。门禁：本测试 `-race -count=20` 每个头 20/20，组合根第 4 份 `7abfd6ef`、`9079f9af` 各一次过。分支 CI：44279808743 第一次 attempt `e2e` 卡在装浏览器（apt 108 kB/s），只重跑 `e2e` 为绿；44292935807（`9079f9af`）19 项第一次 attempt 全绿。第 11 节加决定 595–597；第 12 节 P2 一行、05-lessons 第 3 节三个子条、P2/S2 记录一处后注随之改 | 决定 586、595–597；[旁支的审查记录](reviews/M12-lintbusy-await-expiry-review.md) |
| 2026-11-18 | e2e 装浏览器的旁支 `m12-e2e-browsers-install` → `m12-e2e-browsers-cache`（M0 的故事，在 M12 里发现；`aca36959`、`7c7865d5`，修复第 1 轮 `8641a14e`、`c926e8be`；反向对照的探针 `cd306960` 与撤销 `44ae77c7` 只在旧分支上，不进 `main`；合并见第 12 节）：e2e 在"Install the Playwright browsers"一步撞上作业的 20 分钟被取消：11-18 到约 12:30 UTC 至少 20 次，都在 AWS 的 runner 上，镜像 `us-east-2.ec2.archive.ubuntu.com` 取包 87–147 kB/s；另有 11-11 的 4 次（Azure 上解析不了 `azure.archive.ubuntu.com`）与 10-07 的 1 次。都只重跑 e2e。改法：E2E 一步自己 18 分钟（第 3 节第 11 条的界），装浏览器一步 30、作业 55；apt 的系统包按 Playwright 的版本与 ISO 周缓存，restore 与 save 分开；守卫 `TestTheE2EStepHasALimitOfItsOwn`。审查 A 0I 2M 2N、B 0I 3M 2N 1Q、C 0I 4M 3N 2Q，修复一轮两个提交；核对一轮 A 0I 0M 3N、B 0I 1M 1N、C 0I 0M 1N 1Q，新的 Minor 只涉及已推的说明，在记录的"更正"，不开第二轮。分支 CI：44278466271 全绿；探针 44283057396 只有 e2e 红，红在 E2E 一步自己的时限上，作业 failure；44286707271 全绿（AWS 上命中缓存）；44298257881 全绿（新分支没命中，装浏览器 25:06）。第 3 节第 11 条改；第 10 节一行；第 11 节加决定 598–603；第 12 节 P5 一行；06-environment 第 4 节、05-lessons 第 7 节各一条 | 决定 598–603；[旁支的审查记录](reviews/M12-e2e-browsers-cache-review.md) |
| 2026-11-19 | P2 的 S4 实施、审查与修复（实施 9 个提交，修复第 1 轮 10 个、第 2 轮 3 个、第 3 轮 2 个，一共 24 个；不合并 `main`；合并见第 12 节）：阅读视图就地更新：同一份 HTML 不动，只差附件地址时就地重签（每小时的重读不再换节点，M4-performance 第 13 条），内容变了按顶层块换（尾先比、再头比，中间按键贪心配，留着的节点不移动），语言与笔记本的模式变了不动 HTML，可写与否变了只改勾选框（第 48 条的 94 px 不再跳）；焦点留在同一个节点，或按键在映射到的块与同一处插进的近旁几块里找回，找不到给那一块（决定 605、606）；写入之前就在的音视频留在原处、全屏不退（决定 604）；6.2 的 `forbidden` 重读笔记本，焦点没有着落时给标题（决定 607）；mermaid 的 id 各自独立、一遍扫完；8.1 在 Playwright 的三种引擎里复现、修好（决定 610）；`reading-view.tsx` 拆成三份；Go 与网页共用的签名视图样例（决定 609）。审查 Important 1（`withOwnIds` 的代价，修好）：三份审查 A 0I5M2N2Q、B 1I4M1N1Q、C 0I3M5N（A-M1 与 C-M1 同一件），修复第 1 轮 10 个提交（头 `09c7518d`）；核对第 1 轮新的 Minor 3、Nit 3，修复第 2 轮 3 个提交（头 `c9bdfdb7`）；核对第 2 轮 A 0/1/1、B 0/0/2、C 0/0/0，修复第 3 轮 2 个提交（`1294383e`、`1e43b3cc` 只改注释）；核对第 3 轮 A 0/0/1、C 0/0/3。变异：实施 4.4.7 的 26 条、M4-12b、X6、X7 都红，e2e 七条里六条红（M4-12 只由 vitest 抓到），自己加的 13 条里四条等价；修复三轮的 36 条除两条等价之外都红。分支 CI：run 44370887722、44406706476、44434194881、44452711808 第一次 attempt 都 19 项全绿。PG15 十二格约 15–17 test-seconds（第 2、3 轮 runner 慢时 25–27）。第 7 节"总体设计的修订"第 22 条落了（另改第 26 条）；附录 A 的 8.1；总体设计 13.2 第 23、26 条、13.4 第 3 条与第 15 节；移交 M5-polish、M4-performance 各一行；P5 设计 3.19 的读屏清单；第 11 节加决定 604–611 | [P2 设计](02-P2-web-polish.md) 10.4、[S4 记录](reviews/P2-S4-reading-view-review.md)；决定 604–611 |
| 2026-11-19 | P5 的 S2（推送、签名、证明、SBOM、许可证与 Release）实施、审查与修复（实施 4 个提交，修复第 1 轮 3 个，收口 1 个，一共 8 个，另一次合并 `main`；合并见第 12 节）：镜像里有第三方许可证 `/licenses`（Go 47 个模块与 Go 本身，与二进制的 `go version -m` 逐个相同；npm 233 个包，带上工作区包 `@nervewiki/api-client` 的依赖与 tailwindcss，没有文本的 3 个；文件名不收代码；决定 612–614），由 `image-smoke.sh` 与 `tools/licenses/check.mjs` 核对；演练在 amd64 上只读地取出许可证、生成两份 SBOM，核对前端 SBOM 的包都在 `/licenses`；版本号的 tag 与 Release 写成 `deploy/release-version-tag.sh`、`deploy/github-release.sh`（Release 的状态读 REST 的列表，决定 615），`TestTheReleaseScripts` 对着假的 `docker`、`gh` 走 31 格；`release.yml` 加 `sbom`、`publish`、`draft`、`verify`、`release` 五个作业，`build` 先核对检出的是闸门的提交（决定 561）；守卫的表加五行，另核对 `publish` 的签名与三份证明、`verify` 的身份与两种架构、只有 `release` 公开、`uses:` 钉 SHA（决定 616），跑 smoke 的步骤与 `verify` 的步骤只有 name、env、run（决定 562）；镜像的 SBOM 的文档 `name` 是 `$IMAGE`，在 `sbom` 作业与演练里先核对（决定 617）。审查 Important 1（`openapi-fetch` 打包进前端却没有许可证，修好）：三份审查 A 1I2M1N1Q、B 0I2M4N1Q、C 0I4M6N1Q，修复第 1 轮 3 个提交（头 `0fe3d663`）；核对第 1 轮三位都只剩 Nit（各 1），照负责人 2026-11-19 的规矩不再开核对，`9a6b20ae` 改、主持者核对。变异：7.2 里 S2 的与实施者自加的都抓到，三位审查者照过的各成一条发现，修复之后都抓到。分支 CI 每次推送第一次 attempt：`ci.yml` 19 项与 `release.yml` 两个演练全绿（run 44451246879、44461178186、44484245001、44490478831）；演练加的几步 15–22 秒；`anchore/sbom-action` 没被"允许的 actions"挡住；没有推任何 tag、镜像，没有建 Release。第 7 节"总体设计的修订"第 6 条落了后一句；第 11 节加决定 612–617，决定 439、441 与"需要负责人确认"第 2 项加注；总体设计 13.5 第 6 条与第 15 节；06-environment；移交 M0-P6-image-release 一行 | [P5 设计](05-P5-deploy-release.md) 3.25、10.2 与 [S2 的审查记录](reviews/P5-S2-publish-release-review.md)（A 1I 2M 1N 1Q、B 2M 4N 1Q、C 4M 6N 1Q；核对第 1 轮 A、B、C 各 1N）；决定 612–617 |
| 2026-11-19 | P2 的 S5 实施、审查与修复（实施 21 个提交，修复第 1 轮 8 个、第 2 轮 1 个，合并 `main` 一次，17.4、17.7 两个，修复第 3 轮 3 个、第 4 轮 1 个，再合并 `main` 三次，一共 40 个；合并见第 12 节）：编辑器、附件、上传、粘贴与导入导出：上传的两个计时（停住从第一次上传进度起算，决定 620），取消晚于服务端时告知（按批记 `before`，决定 622），附件一节的播报区与手动复制的字段，"已不在"随恢复撤下，粘贴留着没有东西替代的选区，上传在途时离开先问，补全照服务端的表格写别名（照 goldmark 去掉容器的记号，决定 619），导入对话框的提示与双击，粘贴对话框先问，睡醒之后的到期，写下的尺寸（`data-nw-sized` 与 `base` 层的 `revert-layer`，决定 621），callout 标题里的媒体（rAF，决定 618），提示不随正文复制（决定 623），过期的下载点击时再签，重签被拒之后从失败处再签（决定 624）；新故事 AS6。审查 Important 1（排队等连接的上传被判停住，修好）：三份审查 A 1I4M1N2Q、B 0I3M3N、C 0I2M4N1Q（A-M1 与 C-M1 同一件），修复第 1 轮 8 个提交（头 `a0b108d9`）；核对第 1 轮 Nit 4，修复第 2 轮 1 个提交（`fa849645`）；合并 `main` 与 17.4、17.7 之后核对第 2 轮 A 0/3/0、B 0/0/1、C 0/1/0，修复第 3 轮 3 个提交（头 `d6594761`）；核对第 3 轮 A 0/1/0（错还在的被拒媒体被拖一下记录就丢了）、C 0/1/4（文档稿）、Q 1，修复第 4 轮 1 个提交（`d8027fb4`），A 复核关上，再合并 `main` 三次（`20376fe6`、`e26dfd32`、`27d56eac`）。变异：实施 74 条抓 68（四条等价，两条由别的测试守），修复第 1 轮 24 条抓 22（两条等价），17.4、17.7 与修复第 2–4 轮的都红。分支 CI：run 44354440004、44385655444、44423276368、44443687371、44475220489 第一次 attempt 都 19 项全绿，44487380423 第一次 attempt 红在 runner 的 Docker（起容器超时，与改动无关），只重跑失败的作业，第二次 attempt 19 项全绿；修复第 4 轮与合并的头 `20376fe6` 的 run 44495490042 第一次 attempt 19 项全绿；再合并两次之后的头 `27d56eac` 的 run 44505730967 第一次 attempt 19 项全绿。AS6 三格与 AS3 (page) 多出的约 2–3 test-seconds。负责人 2026-11-19 第 12 问（推送之前只跑相关的门禁，只剩 Nit 不再开核对，收官时整体跑一次全量）写进第 11 节、9.7 与 02-workflow 3.1；总体设计 13.2 第 7、12、21、27 条与第 15 节；移交 M5-polish 一行；05-lessons 两条；P5 设计 3.19 的读屏清单；第 11 节加第 12 问与决定 618–624 | [P2 设计](02-P2-web-polish.md) 10.5、[S5 记录](reviews/P2-S5-editor-assets-review.md)；第 12 问，决定 618–624 |
| 2026-11-20 | P2 的 S6 实施、审查与修复（实施 17 个提交，修复第 1 轮 8 个，收口 1 个，一共 26 个，另合并 `main` 一次；合并见第 12 节）：版本、回收站、历史、目录、后台任务与搜索的网页：在途的恢复与撤销进 store，预览与变更集页后退再前进时跟随它们（22.14、23.10）；`links` 事件等同一单元的树的读读完并显示（24.8，表项读完才删，决定 629），TB2 (page) 改回在 Big 自己的页上删（24.10）；离开笔记本之后回来先等一次树的读，卸下时没读过的也标，离开前读树失败的不先说（27.2，甲'，决定 626）；关着的原生选择用键盘一项项换时等读者离开或按回车（26.5，决定 627、630）；`not_trash_root` 只说一次，面包屑在 `Trail` 里交还焦点（22.19，决定 631）；路由状态的通知只说一次（21.3、22.2，`merged` 留在外壳，决定 628）；还原的说明再说与只在自己的版本上说（21.2、21.10，决定 632）；"显示未改动的行"只在折叠时（21.7）；后台也撤销不了只留"取消"（24.2、24.9）；删除、新建在途时去别处不被拉回（2）；地址里大写的 id（23.19）；页面树的参数上限（12.12）；命中按字素簇（26.18a）；目录不认的筛选（35.3）；"移到……"对话框的子树与高度在循环外算一次（深的形状中位数 1,008 → 190 ms，决定 625）。审查没有 Important：三份审查 A 0I6M3N2Q、B 0I3M4N1Q、C 0I2M6N1Q，修复第 1 轮 8 个提交（头 `07ca08bc`）；核对第 1 轮三位都只剩 Nit（A 1、B 2、C 2），照负责人 2026-11-19 第 12 问不再开核对，代码的两条在 `47e8871d` 改、主持者核对。变异：实施 56 条第一轮抓 54，补格之后全抓到；修复第 1 轮 29 条都抓到；核对第 1 轮活下来的两条由收口的两格抓到。分支 CI：run 44529879716、44547267899 与合并的头 `ffa02d6d` 的 44563702116 第一次 attempt 都 19 项全绿；收口 `47e8871d` 的 run 44555911871 第一次 attempt e2e 红三格（runner 上 PostgreSQL 的提交一时停住，身份的写到了期限答 500，与改动无关），只重跑 e2e，第二次 attempt 全绿。S6 改的四格在 CI 里照原样多 2.3–3.2 test-seconds，折算约 1。第 7 节第 24 条落了；总体设计 13.2 第 12 条与第 15 节；移交 M5-polish 一行；P5 设计 3.19 的读屏清单与人工入口；第 11 节加决定 625–632 | [P2 设计](02-P2-web-polish.md) 10.6、[S6 记录](reviews/P2-S6-versions-trash-history-search-review.md)；决定 625–632 |
| 2026-11-20 | P2 完成：六个 Step 都已合并（S1 `d1ade2ed`、S2 `6505c3bf`、S3 `e639b9f7`、S4 `ad5d78b5`、S5 `22e9f2bd`、S6 `e4d16a0a`），另有 lint 忙的旁支 `32431f25`；决定 378–429（设计）、506–509、584–594、595–597（旁支）、604–611、618–624、625–632。P2 设计 8.1 的完成标准逐条：第 1 条，六个 Step 各自合并、一次一个，守 0.1 的硬依赖，S6 在 P4/S2 的结论与 P4/S4 之后开；合并之前 7.3 的门禁全绿（S5、S6 照负责人 2026-11-19 第 12 问），审查与修复核对结束；每次合并之后第一次 `main` 的 CI 第一次 attempt 19 项全绿（S1–S5：run 43892544486、44289945428、44305876925、44465826335、44515487511；S6 的在它的记号 `4bb2e627`）。第 2 条，各 Step 的测试在没修的代码上红过，变异抓到或写明等价，探查与排查有记录（P2 设计 10.1–10.6）。第 3 条，附录 A 的 118 项（S1 22、S2 29、S3 13、S4 12、S5 21、S6 21）逐项对过移交 M5-polish 六行的"做完"与 M8-history 的 H4 一行，都修好、有合并号；推后进 v0.2 的部分在 P2 设计 10.1–10.6 与 4.4.8，收尾时进 v0.2 的 `M12-polish.md`；移到发布之后的人工验收的没有（8.1、33.1 都复现、修好，决定 610、589；33.1 在真的 Safari 上的一眼进人工清单，决定 590）。第 4–7 条在设计与各 Step 的文档提交里：第 7 节"总体设计的修订"第 19–24 条都落了（本提交补第 23 条的一句）；总体设计 13.2 第 1、7、12、17、21、23、26、27、31 条，13.4 第 3 条与 3.6；移交 M5-polish 六行，M8-history、M4-performance 各一行（仍是 `open`，收尾时关）；05-lessons（S1、S2、S3、S5）；README 不改。下一步：P4/S3、P5/S3，收尾时 `main` 上整体跑一次全量门禁（负责人第 12 问） | [P2 设计](02-P2-web-polish.md) 8.1、10.1–10.6；六个 Step 与 lint 忙的旁支的审查记录 |
| 2026-11-21 | P4 的 S3（服务端的实测）：被量的提交 `f1317bc3`，服务端的代码没改，另有三个设施的提交 `119611ca`、`125a765c`、`9def3138`（B3 上限处的形状与它的测试）；持锁约 20.3 小时（无条件约 14.8、条件与复核约 5.5；修复第 1、2 轮另约 14、40 分钟），机器独占（决定 634），省法取三条（决定 633）；合并见第 12 节。本机对 7763 的比：回收站与撤销族 2.046、升级族 2.257。B1 过（吞吐 170 行/秒、从空建 29 分钟；尾巴有候选、对照 `308eafc6` 没有回归，体积 3.3–4.6 倍）；B2 普通至多 318 MiB，病态 k = 16 2,519 MiB 超、复核 2,385、2,474 也超，k = 8 2,525 MiB 第一次超、复核过（决定 639），早答 0；B3 上限处至多 810 MiB 过（修复第 2 轮重量）；B4 照比推断删除 17.5–18.1 秒（同口径，复核也超）、移动 29.5–30.6（本机一次 500）、改 `vault` 的名 27.9–29.3，都超，改被 59 页链着的页 0.47 过；插值删除的主行约 48,900 个节点到 15 秒（决定 635、638）；B5 规则一层前四个过，`MaxJobItems` 6.1–7.4 秒没过（45,000 至多 4.45 秒）；硬期限 7763 加 2 秒 13.79、14.54、4.82 过，第一行推断 16.47、16.88 过 15 秒；守卫的状态下规则一层本机约 5.4–5.9 秒，记为已知限制（决定 644）；一页 100 万行历史的删除与 30 万页的升级推断过期限，只量（决定 645）；B6 规则至多 6.3 秒、B7 推断批准 7.33、发起 6.31 秒、B8 导入推断 9.1 分钟、B10 `index` 至多 3.125 秒，都过。负责人 2026-11-21 第 13 问：B4 开 P4/S7，`MaxJobItems` 降到 45,000 开 P4/S6，B5 硬期限写明余量，B2 改判约 2.6 GiB。审查与修复：审查 A 2I 5M 5N 1Q、B 3I 2M 3N 1Q、C 2I 5M 5N 1Q，全部接受（[S3 记录](reviews/P4-S3-server-measures-review.md)）；修复三轮：第 1 轮补量 3.7 第一行，第 2 轮 B2 的窗口重算、设施的提交 `119611ca` 与 B3 在上限处重量（至多 810 MiB，过），第 3 轮设施的第二、三个提交 `125a765c`、`9def3138`（测试分得出 2a–2c、三个凭证一致、不靠时机）；核对第 1 轮三位的原发现都关上（新的 A 0I 1M 2N、B 0I 2M 4N、C 0I 1M 2N，在第 3 轮改了），核对第 2 轮 A、C 全关，B 新 Nit 2、C 剩 Nit 1，主持者核。第 7 节"总体设计的修订"第 9 条落了，第 11 节加第 13 问（带三条注）与决定 633–651；4.7.1 的 B2、B5 两行加注；总体设计第 14 节九行、13.4 第 8、9 条与第 15 节；移交 M4-performance、M5-performance、M7-transfer、M8-history 各一行；05-lessons 三条、06-environment 一句 | [P4 设计](04-P4-performance.md) 3.11、3.12、10.3 与 [S3 记录](reviews/P4-S3-server-measures-review.md)；第 11 节第 13 问；决定 633–651 |
| 2026-11-21 | P4/S7（B4 的树写）设计已审查（三份审查：A 1I4M8N1Q、B 1I4M6N2Q、C 1I13M7N1Q；两次修订，核对两轮：第 1 轮三位的原发现全关，新的 A 1M3N、B 4N、C 1M5N，第 2 轮 A、C 各核自己的 Minor 都关上，Nit 主持者核）。A：键与目标合起来至少 4,000 个的到达改读宽读，平常 `LinksReachedWide`、`force_generic_plan` 下 `LinksReachedWideGeneric`，`Store.Links` 经 sqlc 的 `PlanCacheMode` 选；B：单个改名、移动的树单元里观察者用参与者读到的链接与解析。推断改后 7763 上移动、改名约 7 秒、删除约 11–12 秒（× 1.25 约 9、14–15 秒）；改写了页的大子树只有 A 的余量（主行约 13 秒），记为已知限制。组合根的守卫约 1 万个节点、按语句名数调用，计时只兜期限；`costs-2` 的时限照约两倍的规则提到约 60 分钟。量在 P4/S6 之后：两次导入到同一页之下，改前（S7 的合并基点）、改后交错着跑；编辑者负载下改前、改后各一次只记。第 7 节"总体设计的修订"加第 29 条；决定 652–667 写进第 11 节 | [P4/S7 设计](04-P4-S7-tree-writes.md)、[P4/S7 设计的审查记录](reviews/P4-S7-design-review.md) |
| 2026-11-21 | P4 的 S6（`MaxJobItems` 降到 45,000）：八个提交（常量与守卫、文字与契约、`perf` 的导入上限形状与 `-part k/n`、`arrays` 基准的界、`e2e/perf` 的大形状分包，审查的修复三个）；合并见第 12 节。常量 60,000 → 45,000，导入的节点上限随之；守卫的界不动（决定 670）；`perf vault` 的导入上限形状缩到 44,910 个节点，`-part k/n` 把 S3 的 vault 分成 34,355 与 25,555 个节点两份（P4/S7 用，决定 658）；`e2e/perf` 的 `flat50k`、`flat100k` 每包 25,000 页，`folder50k` 两包进同一个文件夹（决定 668）。在原规模上两轮（`89d9c701`，每 5 秒记负载）每格 ≤ 5 秒，至多 3,683 ms，过；第一次与锁外的负载重叠，作废（决定 674）；用户删深的最上层本机 5.8–8.1 秒（改前 10.2–14.0）。S3 的 50,000 量在批次 D 的尾巴上，新的一段上低约 17%，仍取 45,000、不补量，收官时列给负责人（决定 672）。分支 CI 五次全绿，两次 `guards-core` 落在 7763 上，最大的一步 6.93–6.97 秒（60,000 时 10.24 秒）。审查 A 1M5N1Q、B 1M4N1Q、C 2M5N1Q，全部接受；修复两轮（`e3566f2e` 读服务端文字的测试与内容的钉、`14808583` 注释与默认档、`b447dd28` 折行与 `folder50k` 的第一包），核对第 1 轮三位的原发现全关（新的 A 1N、B 2N、C 1M1N），第 2 轮 C 核 C2-M1 关上，Nit 主持者核。第 7 节"总体设计的修订"加第 30 条，第 11 节加决定 668–674；4.7.1 的 B4、B5 两行加注；总体设计 13.1 第 31 条、第 14 节一行与第 15 节；移交 M4-performance 一条；S3 记录与 P4/S7 设计记录各补一次 `main` 的 CI；05-lessons 一条、06-environment 一条 | [P4 设计](04-P4-performance.md) 3.12、3.13、10.4 与 [S6 记录](reviews/P4-S6-max-job-items-review.md)；决定 668–674 |

---

## 附录 A：打磨的逐项去处

编号是 [M5-polish](handoffs/M5-polish.md) 的条目与 I3 拆出的子项；分类照 I3 第 9 节，例外在 4.5。

### 进 v0.1 的（A 31 项、第 48 条、B 81 项；任务的 T2、T4 在 P3/S3，4.6.6）

| Step | A | B | 一起做的 C、D |
|---|---|---|---|
| P2/S1 焦点与共用组件 | 7 | 6.3、9.1、11.7、12.3、16.1、18.6、19.3、21.14、22.17、23.14、23.16、24.3、26.4、28.4、28.5、35.2、38.1、38.3、49（与 24.3 同一个改法） | C：23.17、28.2（随 23.16） |
| P2/S2 文字 | — | 6.1、9.5、15.3、16.6、19.1、21.8、21.11、22.11、22.12、22.13、22.16、23.1、23.3、23.4、23.11、23.12、23.13、24.7、26.11、26.13、26.14、35.1、38.6、38.8、40、41.1、41.2、41.3 | H4 的界面一句（移交 M8-history 第 4 条） |
| P2/S3 375 px、`bdi` 与对比度 | — | 12.4、26.9、26.12、26.17、32.1、32.2、32.4、35.8、38.10、38.11、38.12 | D：32.3；M：33.1 的排查（复现了，按 B 在 S3 修，决定 589）；另加菜单与选项的高亮（P2 设计审查 B 的 M4，主持者的决定 420） |
| P2/S4 阅读视图 | 6.2、11.1、17.1、17.8、48（C 改判为 A） | 3、8.1（S4 复现，第 9 个提交修）、8.2、12.2、29 | C：11.6 的失焦部分；D：14d |
| P2/S5 编辑器、附件、上传、粘贴与导入导出 | 12.8、15.1、15.2、16.3、16.4b、16.11、17.2b、17.4、17.6、17.7、18.1、18.2、19.4、20.3、38.7 | 16.4c、16.10、20.1、20.2、28.1、38.9 | — |
| P2/S6 版本、回收站、历史、目录、后台任务与搜索 | 2、12.12、21.7、22.14、23.10、23.19、24.2、35.3 | 21.2、21.3、21.10、22.2、22.19、24.9、26.2 的前一半、26.5、26.18a、27.2 | D：24.8、24.10、26.15 |
| P3/S1 小的正确性 | 10.1、10.2、25.1（与 25.2） | 30.2、30.4 | C：30.5（与 30.4 定一种约定） |

### 发布之后的人工验收（4.12）

- **M 类**（14 项）：5、8.4、11.8、12.11、13.3、16.12、17.9、18.5、18.8、26.18b、27.3、33.1（P2/S3 复现、修好，真的 Safari 上再看，决定 589、590）、33.2、39。
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
- **P2 只做了一半的与没有并进的**（P2 设计 2.3、4.4.8）：22.16 的重开会话（S2 只改说法）、23.4 的链到那一项（要服务端点名上级）、21.10 的标签页（S6 只改说的话）；S3 没有并进的两处同类（变更集页面包屑的 `current`、回收站预览 `h1` 的长名）；退出登录不为在途的上传问；回收站预览"在后台恢复"的开始也有在途的窗口；S4 之后仍在的限制。

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
