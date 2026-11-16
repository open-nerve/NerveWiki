# M12/P5 部署文档与发布流水线的设计：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | [05-P5-deploy-release.md](../05-P5-deploy-release.md) 的初稿（2026-11-16，1560 行，工作区里未提交；快照是设计工作目录里的 `p5-rev1/p5-draft-v1.md`）与设计者的附注 `p5-draft-notes.md`。对照 `main` `86565dd6` 的代码、[00 号文档](../00-M12-design.md)（决定到 360）、v0.1 总体设计与 P4 设计的初稿（`p4-draft.md`，只读） |
| 审查方式 | 三位审查者并行，按领域分：<br>A S1 的代码：3.2–3.7、3.9，变异的 v、m、l、g、n、v-gate 各行，2.1 第 6、7、8、10、11 条。在自己的 worktree 里经 `heavy.sh` 原样与盖了版本号各跑一遍 `go test -short`；一个只记日志的临时测试核对四个正则与 YAML 的解码；把闸门原样取出、对着假 `gh` 与临时仓库跑 3.9 的每一格加补的几格，做了六个变异；`awk`、`grep -E` 在几种 locale 与实现下比对（证据 `p5-review-a/`）。<br>B 流水线与发布：3.8、3.10–3.14、3.20、3.22，2.1 第 1–5、8、12–15 条，附注的"不确定"第 1–9 条。照行号读代码；本机 gh 2.103.2 的帮助与二进制；本机对称地核对跨架构的 `docker create` 与 `docker cp`；两种架构、两种 cgo 的 `go list -deps`（证据 `p5-review-b/`）。<br>C S3 与完整性：3.15–3.19、2.1 各条、第 8、9 节、2.2、7.4。在本机的 Compose 5.6.1 上跑初稿的 `compose.yaml` 与它的变异；经 `heavy.sh` 照初稿的"安装"用本机的镜像装了一遍、`down -v` 收掉；对照 P4 设计的初稿（证据 `p5-review-c/`）。<br>三位都没有跑 actionlint、没有推送；B 以不用沙箱的方式试 `git ls-remote` 被权限分类器拒了，没有换办法再做；之后为读 gh 的源码用 WebFetch 试过两次，都是 "Socket is closed"，没有读到，改用本机 gh 二进制里的字符串核对（修订核对 B 的 N15） |
| 日期 | 2026-11-16（审查、三次修订、两轮修订核对） |
| 结论 | A：Important 1、Minor 8、Nit 10、Q 0。B：Important 0、Minor 6、Nit 12、Q 2。C：Important 1、Minor 11、Nit 11、Q 2。全部接受，照主持者的修订说明（第一轮，`p5-revision-1.md`，另补一条 NOTIFY 的决定）改。修订直接写进设计，第一次修订之后 1901 行，第二次之后 1913 行，第三次之后 1915 行；每条改在哪里另列在 `p5-revision-1-done.md`（第二、三次修订各起一节）。对 00 号文档的修订写成脚本 `apply_p5_design.py`（第二次修订之后 66 处带标签的改动），在 `86565dd6` 上与 P2、P3 设计的脚本按三种次序叠着跑过，在 `48bbfc99`（P3 的设计已提交）上 `--check` 与写入都过、与 P2 的脚本两种次序都过。两轮修订核对见文末：第 2 轮三位都没有 Important 与 Minor，三条 Nit 在第三次修订里改了，设计已审查。P5-1 到 P5-23 的正式编号由主持者在提交时给（P3 用了 361–377，P2 接 378 起，P5 在 P2 之后） |

## 发现与处置

节号是修订后的：原 3.13–3.22 依次变成 3.14–3.23，新的 3.13 是版本号的 tag 与 Release 的两个脚本。

### A：S1 的代码

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| M12P5A-I1 | Important | actionlint 放进本机的 `make lint-go`：设计者在本机跑它被分类器拒过，规矩是不换个办法再做；S1 之后逐个提交的 lint、推送之前的门禁、README 让贡献者跑的 `make check` 都要在本机跑它，与决定 308 的先例不合 | 主持者定（= C-Q1）：只在 runner 上跑。`make lint-actions` 以自己的 `runner-only` 判断起头、判断与干活在同一个 shell 行；`ci.yml` 的 `costs-1` 一份在 Lint 之后跑它，出错写成公开的注解；本机的门禁不含它；现有两份过不过由 S1 第 5 个提交推上分支之后的 CI 回答（3.5、6.1、6.4、7.3；2.1 第 10 条；P5-6） |
| M12P5A-M1 | Minor | g6"只看 `conclusion`、不看 `status`"是等价的变异：没完成时 `conclusion` 是 `null`，照样拒 | 照改：g6 改成"`[ "$conclusion" != failure ]` 就放行"，`in_progress` 一格抓；原来的写法挪进 7.2 末段的"等价的"（7.2） |
| M12P5A-M2 | Minor | CHANGELOG 标题里的 `.` 有没有转义，表里没有一格看得出；bash 3.2 下 `${base//./\\.}` 的结果没人核对过 | 照改，取了 A 的第二种：闸门照说明脚本的 `awk index` 按字面数标题，不再转义；表里另加 `## 0x1x0（未发布）` 一格，7.2 加 g10（改回不转义的 `grep -cE`）（3.6、3.9、7.2） |
| M12P5A-M3 | Minor | 闸门的测试表没写明三件事：放行的格设不设 `GITHUB_SHA`；"两次运行"的答复次序；拒绝的格只写"1"，前面的早退会盖住后面的检查 | 照改：表前一句"每格只有它要拒的那一条不过，其余照 rc 放行一格"；拒绝的格断言标准错误里那一条的字样；放行的格设 `GITHUB_SHA` 为 tag 指的提交，另留一格不设；"两次运行"从新到旧排。7.2 加 g11（与 tag 对象比）、g12（去掉 `sort_by`）（3.9、7.2） |
| M12P5A-M4 | Minor | 闸门放过"空的一节"，说明脚本却拒它；拒的时候版本号的 tag 已经复制出来，`v0.1.0` 那时只能出 `v0.1.1` | 照改：闸门第 4 条用一个全零的假摘要跑一次说明脚本，两者走同一段代码；表里两格"这一节是空的"，7.2 加 g13（3.6、3.9；P5-5；2.1 第 24 条） |
| M12P5A-M5 | Minor | 两个脚本的可执行位只在推 tag 时才用到，测试以 `bash <脚本>` 跑，看不出缺了它 | 照改：测试直接执行脚本，PATH 照旧受控；7.2 加 g14（去掉可执行位）；S2 的两个脚本同样（3.9、3.13、7.2；P5-1） |
| M12P5A-M6 | Minor | 守卫查"不把 tag 名拼进 `run:`"用的是带固定空格的字面串，`${{github.ref_name}}` 与 `${{ github.ref }}` 都漏得过 | 照改，取了 A 的"更简单的写法"：`run:` 里的 `${{ … }}` 只许 `matrix.` 起头的，三份工作流都核对；w8 另加不带空格与 `github.ref` 两种（3.9、7.2） |
| M12P5A-M7 | Minor | 守卫只读闸门那两步的 `run` 文字；一步加 `continue-on-error: true` 或 `if:`，`make vuln` 红了闸门照放行 | 照改：`release.yml` 的作业与步骤都不许 `continue-on-error`，`gate` 的步骤都不写 `if`；7.2 加 w10（两种写法）、w14（3.9、7.2） |
| M12P5A-M8 | Minor | 2.1 第 11 条"守卫之后漏一处就红"说过了头：守卫只看四份默认值与 README | 照改：四份默认值与 README 漏一处就红；示例与注释照清单改，生成物由 `make gen-check` 核对（2.1 第 11 条） |
| M12P5A-N1 | Nit | 测试的子进程环境写不全：`GIT_CONFIG_GLOBAL=/dev/null` 之下 `git commit` 以 128 退出，`git init` 的默认分支是 `master` | 照改：加四个 `GIT_AUTHOR_*`、`GIT_COMMITTER_*`，`git init -b main`、`push origin HEAD:main`（3.9） |
| M12P5A-N2 | Nit | g3 要改两处：只换端点时选出的仍是红的 `ci.yml`，变异活下来 | 照改：g3 写明端点与 `.path` 的条件一起改，mut.py 用 `also`（7.2） |
| M12P5A-N3 | Nit | v-gate 只写"PATH 的桩"：只桩 `pnpm` 时会在本机真跑 `govulncheck`；它只是一次手做的核对 | 照改：写明 `go`、`pnpm` 两个桩，是实施时手做的一次，CI 不常驻；2.1 第 8 条照实写（3.22、7.2；2.1 第 8 条） |
| M12P5A-N4 | Nit | P5-4 说 `created` 是提交的时间，没有东西核对它 | 照改：`image-smoke.sh` 与 `git -C "$repo" log -1 --format=%cI` 整个比；7.2 加 l4（3.4、7.2；P5-4） |
| M12P5A-N5 | Nit | 列工作流用 `*.yml`，新加的 `x.yaml` 漏得过 | 照改：守卫 `os.ReadDir` 整个目录；actionlint 不给文件，自己找 `.yml` 与 `.yaml`，`-verbose` 列出读的文件（3.5、3.9） |
| M12P5A-N6 | Nit | 说明的测试里 `IMAGE`、`GITHUB_REPOSITORY` 要取与生产不同、彼此不同的假值 | 照改；7.2 加 n5（写死镜像名）；闸门的测试同样（3.9、7.2） |
| M12P5A-N7 | Nit | `deploy/Dockerfile:1-3` 的用法注释要写上 `REVISION`、`CREATED` | 照改（3.4） |
| M12P5A-N8 | Nit | `toolchain_test.go` 的 `only` 是闭包、只认"恰一个"，不能照抄 | 照改：`version_test.go` 有自己的 `matches(t, file, re, n)`（3.2） |
| M12P5A-N9 | Nit | tag 指的提交要有它自己的 `ci.yml` 运行；合并提交与记合并号的文档提交一起推时，`v0.1.0` 在闸门被拒 | 照改：3.23 第 4、7 步写明合并提交单独推；闸门的报错提一句；2.1 第 24 条改 00 号文档 4.11 第 6 步（3.6、3.23） |
| M12P5A-N10 | Nit | 两句措辞：`TestGetReportsStampedVersion` 钉住的是 `Get()` 把 `version` 传给 `fromSettings`；"受影响的测试"漏了 `cmd/nervewiki/main_test.go:31` 与 `bootstrap/api_test.go:64` | 照改（第 1 节、3.3） |

### B：流水线与发布

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| M12P5B-M1 | Minor | `release` 一步把 `gh release view` 的任何失败都当作"没有 Release"：一时读不到会再建一个草稿（草稿不占 tag，可以有两个），与 P5-12 的原则相反 | 照改：只有标准错误含 `release not found` 才算没有，别的让脚本失败；7.2 加 r5（3.13；P5-12） |
| M12P5B-M2 | Minor | 版本号的 tag 不改指与 Release 的草稿、已发布两支，正常的 rc 一次也走不到，没有反向对照 | 照改：两步写成 `deploy/release-version-tag.sh`、`deploy/github-release.sh`，`TestTheReleaseScripts` 用假的 `docker`、`gh` 把每一支各跑一格；7.2 加 r1–r7；守卫核对那几步恰是调这两个脚本（新的 3.13、3.9、3.14、7.2；P5-20） |
| M12P5B-M3 | Minor | 演练的 pnpm store 缓存与 `ci.yml` 共用一个键却只装一部分，会让 web、e2e 两项取回半个 store；3.20 写 `sbom` 也用缓存，与 3.13、P5-7 不一致 | 照改，取了第一种：演练与 `sbom` 都不缓存 store；3.21 的表去掉 `actions/cache`（3.14、3.15、3.21；P5-7） |
| M12P5B-M4 | Minor | 给用户的 `gh attestation verify` 只带 `-R`，比 `verify` 作业与同一份说明里的 cosign 命令都弱 | 照改：说明、5.4 与 `verify` 作业都带 `--cert-identity`，与 cosign 的身份逐字相同；n3 盖住两条命令（3.7、3.14、7.2；2.1 第 5 条） |
| M12P5B-M5 | Minor | 发布的构建所用的 BuildKit 是浮动的 tag（`moby/buildkit:buildx-stable-1`），没有钉住 | 照改：演练与 `build` 的 `setup-buildx-action` 写同一个 `driver-opts: image=moby/buildkit:<版本>@sha256:…`，守卫核对两处相同；3.21 加一行；3.23 第 1 步的手动核对加 BuildKit。版本与摘要 S1 实施时取（设计时不联网）（3.8、3.9、3.21、3.23；P5-18；2.1 第 17 条） |
| M12P5B-M6 | Minor | `publish` 里也跑第三方的 action（`docker/login-action`、cosign-installer），3.13 的注释与 P5-8 说得过头 | 照改：`publish` 的登录改成 shell 的 `docker login`，守卫核对它不用 `docker/login-action`；注释与 P5-8 照实写"第三方的只有 cosign-installer"（3.14、3.9；P5-8） |
| M12P5B-N1 | Nit | "S1 合并之后、S2 之前推了 tag，摘要被推上去"不对：S3 合并之前没有 `CHANGELOG.md`，闸门就拒 | 照改（3.8） |
| M12P5B-N2 | Nit | "七个作业（演练不跑）"数错了 | 照改：tag 上九个作业实例（3.23 第 5 步） |
| M12P5B-N3 | Nit | 2.2 第 1 行只说 S1 的 `docker/*`；S2 的演练也用 `anchore/sbom-action` | 照改（2.2；2.1 第 14 条） |
| M12P5B-N4 | Nit | 2.2 第 4 行比 00 号文档多了"不能删与改指"，失败表却给负责人"删掉 `v0.1.0` 的 git tag"的选项 | 照改：第 4 行照 00 号文档写"只有负责人与主持者能推"；失败表写明删 tag 要负责人（2.2、3.23） |
| M12P5B-N5 | Nit | `n -ge 2` 分不出"镜像一份、前端一份" | 照改：按两份 SBOM 的文档 `name` 分，去重之后一份 `@nervewiki/web`、一份以 `$IMAGE@sha256:` 起头（syft 的写法是推断，S2 的演练记下）（3.12、3.14） |
| M12P5B-N6 | Nit | `go.sh` 的 `go list -deps` 没写 `CGO_ENABLED=0` | 照改（3.11） |
| M12P5B-N7 | Nit | 头的表写"八个 action"，3.20 说 12 个 | 照改：统一为十二个，写明是设计者读过一次、审查没能复核（头、3.21） |
| M12P5B-N8 | Nit | `gh attestation verify` 不登录时以 4 退出；Release 的说明也要写"先 `gh auth login`" | 照改（3.7、3.18；2.1 第 5 条） |
| M12P5B-N9 | Nit | "rc 之前只剩……"漏了 `sbom` 作业的跨架构 `docker create` 与 `diff -r`；B 在本机对称地核对过 | 照改：写"已在本机对称地核对"（3.15） |
| M12P5B-N10 | Nit | 工作流的注释说不留构建记录，演练却每次上传 `rehearsal-files` | 照改，取了"省掉"：演练不传 artifact（= B 的过度设计）（3.8、3.15） |
| M12P5B-N11 | Nit | 第 2 步请负责人做设置，第 3 步直接推 rc，中间没有核对 | 照改：新的第 3 步以 `gh api` 只读地看规则集、immutable releases 与包，读不到的请负责人回一句（3.23、2.2） |
| M12P5B-N12 | Nit | 守卫可以顺手断言每个 `actions/checkout` 都是 `persist-credentials: false` | 照改：三份工作流都核对；7.2 加 w11（3.9、7.2） |
| M12P5B-Q1 | Q | `verify` 要不要挪到公开 Release 之前 | 主持者定：挪。见"主持者的决定" |
| M12P5B-Q2 | Q | `v0.1.0` 的 `verify` 因流水线自己的错红了、产品没有问题，要不要也出 `v0.1.1` | 主持者定：不出。见"主持者的决定" |

### C：S3 与完整性

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| M12P5C-I1 | Important | 样例没有设 `trusted_proxies`：宿主机上的代理经端口转发进来，所有客户端在服务看来都是 compose 网络的网关这一个 IP，按 IP 的限流全实例共用一个桶；子网是 Docker 当场分配的，用户填不出固定的值 | 照改（主持者：信任钉住的整个子网）：`compose.yaml` 给默认网络钉 `10.213.47.0/24`，`serve` 设 `NWIKI_SERVER__TRUSTED_PROXIES` 为它；`deploy/README.md` 的"安装"写明为什么；演练加一格（带 `X-Forwarded-For` 的登录，日志里没有那条 WARN，会话记下转发来的 IP）（3.17、3.18、3.20；P5-16；2.1 第 22 条） |
| M12P5C-M1 | Minor | 反向对照 d2 不成立：插值与 schema 覆盖整份文件，不带 `--profile migrate` 时 `migrate` 里同样的错照样失败 | 照改：d2 改为 `migrate` 的 `secrets` 写成 `[jwtx]`；`Makefile` 的注释照实写（3.17、7.2；2.1 第 22 条） |
| M12P5C-M2 | Minor | 升级演练取旧镜像的命令取错、前后矛盾：`git log -1 -- server/migrations/` 取到只改了测试的提交，它已有全部 59 条迁移；M11 收尾以来没有新的迁移 | 照改：M12 有新迁移时取 `5e8af5b5..HEAD` 里最早一条新迁移的父提交；没有时（设计者取了 C 推荐的前一种）从 `5e8af5b5` 升，记录写明没有待执行的迁移（3.20；P5-17） |
| M12P5C-M3 | Minor | `blobs/` 的备份与恢复没有设计：镜像里没有 shell 与 `tar`，`docker compose cp` 放回的属主是宿主机的用户 | 照改，取了绑定目录：`./data:/data`，第一次装时 `chown 65532:65532`，备份与恢复在宿主机上打包、改属主；`pg_dump` 写 `exec -T`；演练断言属主与字节（3.17、3.18、3.20；P5-16） |
| M12P5C-M4 | Minor | 样例的库角色是 PostgreSQL 的超级用户（`POSTGRES_USER` 总是），P5-16 只说"表的所有者" | 照改（主持者）：超级用户只在容器里用；`deploy/compose-initdb.sh` 建 `NOSUPERUSER` 的 `nervewiki` 作库与表的所有者，服务与迁移用它；`pg_trgm` 由它建靠 trusted 的扩展，S3 的演练第一遍就核对（3.17、3.18、3.20；P5-16） |
| M12P5C-M5 | Minor | `compose.yaml` 头的注释里，升级没有先停服务 | 照改：注释与"升级与回退"都先 `docker compose stop serve`（3.17、3.18） |
| M12P5C-M6 | Minor | 对 00 号文档的改动没有写出新文字；13.5 第 6 条的文字与 00 的"总体设计的修订"第 6 条不同而没有记 | 照改：2.1 每条写出 `86565dd6` 上的行号、旧文字与新文字（由脚本 `--markdown` 生成，与脚本一字不差）；第 18 条改"总体设计的修订"第 6 条；改到决定 322 的在原文里注明"原来是……"（2.1；第 8 节第 4 条） |
| M12P5C-M7 | Minor | 总体设计第 14 节的 `NOTIFY` 一行，P4 与 P5 两份设计都要改 | 主持者定：归 P5/S3，P4 的第 8 节不再改它（已告诉 P4 的设计者）。2.1 第 20 条改"总体设计的修订"第 9 条；第 8 节第 5 条写出锚点与新文字（P5-23） |
| M12P5C-M8 | Minor | 总体设计还有四处指着"README 的部署一节"（`:923` 两处、`:929`、`:1311`） | 照改：S3 改为 deploy/README；2.1 第 19 条记为"总体设计的修订"第 7 条的补充（第 8 节第 5 条） |
| M12P5C-M9 | Minor | 移交的对照漏了 M4-performance 第 17、18 条（与第 14 条） | 照改（第 2 节的表、3.18 的对照表、第 8 节第 7 条） |
| M12P5C-M10 | Minor | "三个包没有许可证文本"进了已知限制，却没有移交的去处 | 照改：第 9 节 v0.2 的 `M12-release.md` 列出三个包；对照表的来源加它；2.1 第 23 条改 00 号文档 4.13（P5-10） |
| M12P5C-M11 | Minor | 部署文档要的数与 P4 设计交出的对不上：小机器一档与 `reindex` 的 WAL P4 不量；P4 给的每 1,000 页的库、树的 JSON 压缩比与 `GOMEMLIMIT` 的建议值表里没列 | 照改：`reindex` 沿用 M8 的数；表里补上 P4 给的三样；第 9 节照 P4 记录的"给部署文档"对表；2.1 第 21 条改 00 号文档 4.8（3.18）。小机器一档第一次修订写"推算"，第二次修订改为取 P4 修订里另量的一组（P4-27，修订核对 C 的 M12） |
| M12P5C-N1 | Nit | 第 8 节第 5 条的两个行号：第 11 节最后一段在 `:931`，13.5 第 5 条在 `:1289` | 照改（第 8 节第 5 条） |
| M12P5C-N2 | Nit | `trusted_proxies` 的写法已有：逗号分隔的 CIDR | 照改（3.17） |
| M12P5C-N3 | Nit | 第 8 节第 4 条把头的"状态"写死，会盖掉别的 Phase 的内容 | 照改：状态由主持者照提交时的状态写，脚本不改它（第 8 节第 4 条） |
| M12P5C-N4 | Nit | `CHANGELOG.md` 一句读起来像两处都要 reindex | 照改：只有带切罗基文标题的试用库要 reindex；10.2 只是锚点变了（3.19） |
| M12P5C-N5 | Nit | `image-smoke.sh:154-167` 不建页 | 照改：另写 `POST /api/v0/notebooks/{id}/pages`（3.20） |
| M12P5C-N6 | Nit | 6.4 说 S3 改了 Dockerfile 或 `image-smoke.sh`，它没改 | 照改：S3 改的是 `.dockerignore`，并为演练构建镜像（6.4） |
| M12P5C-N7 | Nit | 演练起的 PostgreSQL 与服务跨好几步开着，不只构建是重的 | 照改：三遍各写成脚本，整个经 `heavy.sh`；构建旧提交之前看 `df -h /`（3.20） |
| M12P5C-N8 | Nit | 在仓库里照样例试，会生成没被忽略的 `deploy/.env` 与私钥，私钥进构建上下文 | 照改：`.gitignore` 与 `.dockerignore` 加 `deploy/.env`、`deploy/secrets/`、`deploy/data/`、`deploy/backups/`（3.16） |
| M12P5C-N9 | Nit | 顺带要改的说法：13.5 第 1 条、06-environment 第 1 节与 `:49` | 照改（第 8 节第 5、6 条） |
| M12P5C-N10 | Nit | GHCR 留下的摘要清不清该放 v0.2 的 `M12-release.md`，不是 v0.1.1 的 | 照改（第 9 节、3.23；2.1 第 23 条） |
| M12P5C-N11 | Nit | 备份的命令写明 `docker compose exec -T` | 照改（3.18） |
| M12P5C-Q1 | Q | actionlint 进 `make lint-go` 之后本机的门禁怎样过 | 主持者定（= A-I1）：只在 runner 上跑 |
| M12P5C-Q2 | Q | 要不要把"P2 的部分收尾时补"定为默认，S3 只等 P4/S3 与 P5/S2 | 主持者定：是。见"主持者的决定" |

## 重叠

- M12P5A-I1 与 M12P5C-Q1 是同一处（本机的 actionlint），由主持者的第一条决定收口。
- M12P5A-M2、M12P5A-M4 都在闸门第 4 条：由"按字面数标题、再照说明脚本取一次这一节"一起收口。
- M12P5B-M1 与 M12P5B-M2 都在 Release 的那一步：写成脚本之后，M1 的"只认 not found"是脚本的一支、有它的一格（r5）。
- M12P5B-N10 与 B 的过度设计是同一处（演练上传的 artifact）。
- M12P5C-M7 与主持者补的 NOTIFY 决定是同一处。

## 主持者的决定

主持者在修订说明里定下的处置（作者照做；负责人可以改判）。正式编号 430–452（P5-k 是 429 + k），主持者在提交时给的：

| # | 决定 | 出处 | 写在设计的哪里 | 正式编号 |
|---|---|---|---|---|
| 1 | actionlint 只在 GitHub 的 runner 上跑，不进本机的 `make lint`：照决定 308 的先例，给它一个只在 runner 上跑的目标（判断与干活在同一个 shell 行），`ci.yml` 里一项跑它；本机的门禁、逐个提交的 lint 不含它；实施与审查都不在本机跑 | A-I1 = C-Q1 | P5-6；2.1 第 10 条；3.5、6.1、6.4、7.3 | 435（P5-6） |
| 2 | `verify` 挪到公开 Release 之前：`draft` 只建草稿、传附件，`verify` 之后由 `release` 作业公开；`verify` 失败时只留下没公开过的草稿；镜像的版本号 tag 仍在 `verify` 之前出现 | B-Q1 | P5-11；2.1 第 3 条；3.14、3.9、3.23 | 440（P5-11） |
| 3 | 流水线自己的错不出 `v0.1.1`：`v0.1.0` 的 `verify` 因校验命令或流水线本身的错红了、产品没有问题时，修流水线、照说明里的命令手动校验、记进收尾记录；手动公开那个草稿是对外的一步，先问负责人；产品的问题照 4.11 出 `v0.1.1` | B-Q2 | P5-21；2.1 第 16 条；3.23 的失败表 | 450（P5-21） |
| 4 | P5/S3 只等 P4/S3 与 P5/S2，不等 P2 全部合并；P2 的部分在收尾时补进 `CHANGELOG.md` | C-Q2 | P5-22；2.1 第 9 条；第 0 节、3.18、第 9 节 | 451（P5-22） |
| 5 | 总体设计第 14 节的 `NOTIFY` 一行归 P5/S3 改，P4 的第 8 节不再改它 | C-M7（主持者补的一条） | P5-23；2.1 第 20 条；第 8 节第 5 条、第 9 节 | 452（P5-23） |
| 6 | 3.21 的十二个 SHA 只对照了调查记录与现有工作流，设计里不写"已核对"，P5/S1 实施时以公开仓库核对 | B"先说两件事"第 1 条 | 头、3.21；2.1 第 12 条 | 不编号（处置） |
| 7 | 说明与 `deploy/README.md` 写明用 cosign v3；`cosign verify` 能否分辨签名与证明，rc 时记 `--output json` 核对 | B 未核实的第 3、4 条 | 3.7、3.18、3.23 第 5 步；2.1 第 5 条 | 不编号（处置） |
| 8 | `compose.yaml` 的样例设 `trusted_proxies`，信任钉住的整个子网 | C-I1、C 未核实的第 2 条 | P5-16；3.17、3.18 | 并进 P5-16 |
| 9 | 样例的库角色不用超级用户，`pg_trgm` 由非超级用户建，实施时核对 | C-M4 | P5-16；3.17、3.20 | 并进 P5-16 |
| 10 | B 的 M2 把两段移进脚本、用假的 `docker` 与 `gh` 测试；B 的 M3 演练的缓存不与 `ci.yml` 共用键；B 的 M5 钉 BuildKit；C 的 M3 设计 `blobs/` 的备份与恢复；C 的 M11 列出缺的数；三位的过度设计照建议删短 | 修订说明"Minor 与其余" | 见上面各条 | 并进 P5-7、P5-16、P5-18、P5-20 |

其余 Minor 与全部 Nit 照审查者的建议改。

### 设计者的取舍 P5-1 到 P5-23（主持者的决定，负责人可以改判）

设计的 2.3 写着同样的二十三条，接受设计的文档提交把它们写进 00 号文档第 11 节（第 8 节第 4 条）。P5-20 到 P5-23 是这一轮新加的。

| 临时号 | 取舍 | 依据 | 正式编号 |
|---|---|---|---|
| P5-1 | 发布的守卫都是 `platform/buildinfo` 的 Go 短测试；脚本由测试直接执行 | 初稿；A-M5 改了执行的方式 | 430 |
| P5-2 | MCP 的版本由盖了版本号的 `make test-go-short` 守住 | 初稿；A 经 `heavy.sh` 核对盖了版本号的全部短测试 | 431 |
| P5-3 | README 写的默认值进版本号的守卫 | 初稿 | 432 |
| P5-4 | 镜像的三个 label；`created` 是提交的时间，image-smoke 逐字核对 | 初稿；A-N4 | 433 |
| P5-5 | tag 只认两种写法；`CHANGELOG.md` 的标题写法；正式版本要日期，这一节不能是空的 | 初稿；A-M4 | 434 |
| P5-6 | actionlint 只在 runner 上跑，`-shellcheck=` | 主持者的决定 1 | 435 |
| P5-7 | 不用构建的缓存与 pnpm 的 store 缓存；演练断言原生架构 | 初稿；B-M3 | 436 |
| P5-8 | SBOM 与许可证在只读的 `sbom` 作业里；`publish` 用 shell 登录 | 初稿；B-M6 | 437 |
| P5-9 | SBOM 的内容（syft 从 amd64 的一份；`pnpm sbom --prod`） | 初稿 | 438 |
| P5-10 | `/licenses` 在 Dockerfile 里收集；Go 一侧缺文件让构建失败 | 初稿；C-M10 补了去处 | 439 |
| P5-11 | Release 先草稿，核对过才公开；推荐 immutable releases | 初稿；主持者的决定 2 | 440 |
| P5-12 | 版本号的 tag 不改指；只认 not found | 初稿；B-M1 推到 `gh release view` | 441 |
| P5-13 | 只签索引的摘要 | 初稿 | 442 |
| P5-14 | 索引带三个注解 | 初稿 | 443 |
| P5-15 | rc 失败时的收拾 | 初稿；主持者的决定 2 改了草稿的那一支 | 444 |
| P5-16 | compose 样例：非超级用户的角色、绑定 `./data`、钉子网与可信的代理 | 初稿；C-I1、C-M3、C-M4，主持者的决定 8、9 | 445 |
| P5-17 | 升级的演练从哪里升上来 | 初稿；C-M2 | 446 |
| P5-18 | syft、cosign 与 BuildKit 的版本显式写 | 初稿；B-M5 | 447 |
| P5-19 | 闸门不等 CI | 初稿 | 448 |
| P5-20 | 版本号的 tag 与 Release 的两步写成脚本、对着假的 `docker`、`gh` 测试 | B-M2 | 449 |
| P5-21 | 流水线自己的错不出 `v0.1.1` | 主持者的决定 3 | 450 |
| P5-22 | P5/S3 不等 P2 全部合并 | 主持者的决定 4 | 451 |
| P5-23 | `NOTIFY` 一行归 P5/S3 | 主持者的决定 5 | 452 |

## 设计者在修订中另做的取舍

修订时与修订说明或审查者的字面不同、或多做了的：

1. **闸门的第 4 条两样都做了**：A-M2 给了"加一格"与"改用 `awk index`"两种，A-M4 给了"渲染一次"与"共用代码"两种。取了按字面数标题（不转义，消掉 bash 3.2 的疑问）加跑一次说明脚本（就是共用那段代码），`0x1x0` 一格也加了，作为 g10 的抓手。
2. **作业改名**：主持者的决定 2 要一个"小作业"公开 Release。发布的作业改为 `publish`（镜像：索引、证明、签名、版本号）、`draft`（Release 的草稿与附件）、`verify`、`release`（公开），读起来是"草稿 → 核对 → 公开"。Release 的两步共用一个脚本 `deploy/github-release.sh`，`draft` 与 `publish` 两个子命令，附件名由 tag 算出，rc 由 tag 判。
3. **两个新脚本放在 S2**：它们只给 S2 的作业用；S2 多一个提交（6.2 第 3 个）。
4. **`runner-only` 不复用 `ci-only`**：`ci-only` 的提示说"把依赖的清单发给外部的服务、结论看 `vuln.yml`"，对 actionlint 不对，另写一个只差提示的。actionlint 在仓库根、不给文件地跑，`-verbose` 让作业日志写出读了哪几份。
5. **守卫多管了几件**：`run:` 里只许 `matrix.` 的表达式、`persist-credentials: false`、BuildKit 两处相同、调的脚本、`publish` 不用 `docker/login-action`，都对三份或 `release.yml` 一起核对；现有 `ci.yml`、`vuln.yml` 先在没改过的树上跑一遍，报出的同一个提交改（6.1 第 7 个提交）。
6. **compose 样例的写法**：子网取 `10.213.47.0/24`，不在 Docker 默认的地址池里；超级用户另用 `NWIKI_DB_ADMIN_PASSWORD`；初始化脚本以 `psql -v` 传密码，不拼进 SQL 的文本。
7. **`verify` 按文档 `name` 分 SBOM**：去重之后比，重跑多出的证明不让它红；syft 给镜像的文档名是推断，S2 的演练先记下。
8. **小机器一档**：第一次修订写"推算"，不请 P4 加一组量（C-M11 的两种里取了不加 P4 工作量的一种）；P4 的修订自己加了一组（P4-27），第二次修订照主持者改为取它。`reindex` 沿用 M8 的数。
9. **v-gate 仍是手做的一次**（A-N3 的两种里取了"实施时核对一次"），2.1 第 8 条照实写 CI 里没有东西常驻地守着它。
10. **2.1 的旧文字与新文字由脚本生成**：`apply_p5_design.py --markdown` 打出每处的标签、`86565dd6` 上的行号、旧文字与新文字，设计照抄，两者一字不差；"需要负责人确认"的两行、第 11 节的取舍、第 12、13 节的行在第 8 节第 4 条说明。
11. **失败的表多两支**：重跑的作业下载不到前一次尝试的 artifact 时（B 未核实的第 2 条），`publish` 失败可以 Re-run all jobs（版本号的 tag 还没出现），`draft` 失败只能删草稿、rc.N+1。
12. **rc 的步骤多一步**：核对负责人的设置（B-N11），步骤从八步变成九步；设置的接口名是推断的，读不到就请负责人回一句。
13. **`.gitignore`、`.dockerignore` 多加了 `deploy/data/`、`deploy/backups/`**：绑定的附件目录与备份的位置同样不该进上下文。

## 审查者核对过、没有问题的

- **A**：
  - 3.2 的四个正则在 `86565dd6` 上的匹配数与值；`TEST_VERSION` 不会被认成默认值；仓库里 `0.1.0-dev` 的出处与第 1 节、2.1 第 11 条一致；
  - `go.yaml.in/yaml/v3` 解码 `on`、`permissions: {}`、`if:`；
  - 盖版本号的短测试：94 个包都过，`NWIKI_TEST_VERSION` 不被当成配置键；
  - `TestMCPServerInfoIsTheBuild` 只落进第四份；`mcpOptions` 的拆法与调用者；
  - 闸门照原稿在表里每一格得到写的结果，两个 `grep -E` 与说明的 `awk` 在几种 locale 与实现下一样；3.4 的 `jq`；
  - actionlint v1.7.12 是最新的 tag；6.1 的次序。
- **B**：
  - 两种架构的 Go 模块相同（两种 cgo 都是同样的 47 个）；Go 的许可证文件与初稿相同；npm 一侧两种架构相同；
  - 跨架构的 `docker create` 与 `docker cp` 不要仿真；`gh release view <tag>` 看得到草稿；先草稿后公开在 immutable releases 下是对的；`gh attestation verify` 的选项；gh 的 `release` 各写法；
  - `imagetools create` 单个来源原样复制；`pnpm sbom` 的选项；
  - 权限与 `if` 与表一致；`gate` 与 `vuln.yml` 的步骤一致；`image-smoke.sh` 的 `cleanup` 不删镜像；`ci-wait.sh` 的取法；
  - 重跑与整次重跑的推理；默认 shell 下管道失败不会悄悄通过；问负责人的时机。
- **C**：
  - `compose.yaml` 的环境变量名、`.env` 的变量不进配置、`migrate` 在 profile 里、`run --rm -T serve users create`、私钥的权限、具名卷第一次挂上的属主、`config -q` 的两种反应、`down -v` 只删本项目、`/readyz`、`stop_grace_period` 不短于 38 秒；
  - 守卫用 `strings.Contains`，p1、p2 抓得到，d1 在两个服务里都抓得到；
  - 根 README 的各行号、要随之改的三处注释、00 号文档"总体设计的修订"第 7 条的五个锚点、附录 A 的八项、两份清单的路径、`0.1.1-dev` 的清单；
  - 3.18 的大纲覆盖 4.8；两张对照表的口径；其余移交的对照；
  - 组合根的分份与 7.4 的推算；06-environment 新的一条。

## 未采纳

审查者给了两种做法、主持者或作者取了一种的，没有取的那一种：

- **A-I1 的第二种**（S1 开工之前问负责人本机能不能跑）：主持者取了只在 runner 上跑。
- **A-M6 的正则写法**：取了"只许 `matrix.`"，更短、更严。
- **A-N3 的短测试**：取了实施时手做一次。
- **B-M2 的退路**（写成已知的盲区、rc 上有意 Re-run all jobs 一次）：取了脚本与假的 `docker`、`gh`。
- **B-M3 的只取不存、独立的键**：取了不缓存。
- **B-M6 的"只改文字"**：取了 shell 登录。
- **B-N4 的另一种**（写明删 tag 要负责人绕过规则集、第 4 行保留"不能删与改指"）：取了照 00 号文档的写法，失败表写明删 tag 是负责人的事。
- **C-I1 的另一种**（代理也放进 compose）：只在 `deploy/README.md` 里提一句。
- **C-I1 的"只信任网关 `/32`"**：主持者取了整个子网。
- **C-M2 的 `19b2ae6f^`**：取了 `5e8af5b5`。
- **C-M3 的具名卷加 `db` 镜像的 `tar`**：取了绑定目录。
- **C-M4 的接受超级用户**：主持者取了不用超级用户。
- **C-N3 的"写出整句新文字"**：取了脚本不改头的状态、由主持者写。

没有一条审查者的建议被判为错的。

## 新的问题

没有要负责人现在定的。实施时要看的（多数是三位都标为"未核实"的，只有分支 CI 或 rc 看得到）：

- 现有 `ci.yml`、`vuln.yml` 过不过 actionlint（S1 第 5 个提交推上分支之后）。它们的 `run:` 里只有 `matrix.leg`（修订核对 A 读过）。
- 十二个 action 的 SHA 与 BuildKit 的版本、摘要（S1 实施时以公开仓库核对）。
- arm 的 runner 有没有 `openssl`（S1 的分支 CI，演练的 Tools 一步）。
- syft 给镜像的 SBOM 的文档 `name`（S2 的演练）。
- 钉住的子网与宿主机自己的网段（局域网、VPN）重叠时 Docker 怎样反应（修订核对 C 未核实的一条；`deploy/README.md` 让用户先看 `ip route`）。
- 只有 rc 看得到的：GHCR 怎样存 cosign v3 的签名、`cosign verify` 验过的是不是签名那一份（`--output json`）；重跑的作业能不能下载前一次尝试的 artifact；`imagetools inspect` 对不存在的 tag 的错误信息；tag 的运行的 `head_branch` 是不是 tag 名；`GITHUB_SHA` 是提交还是 tag 对象（脚本两种都对）；`DOCKER_BUILD_RECORD_UPLOAD` 的名字；immutable releases 与规则集的只读接口名。
- P4 的设计照主持者的话删掉第 8 节里 `NOTIFY` 那一句；P4 记录的"给部署文档"一节与 3.18 的表对得上（主持者核对）。

## 修订核对第 1 轮

三位核对者各核对自己那一份的发现与第一次修订的设计、本记录与 00 号文档的脚本（`p5-check1-{a,b,c}.md`；证据在 `p5-check1-{a,b,c}/`）。A 把修订后的闸门原样取出、照 3.9 的新写法搭驱动跑了 26 格与 g1–g14，把 `make lint-actions` 的配方放进临时的 Makefile、PATH 最前放一个只记参数的假 `go`（没有跑 actionlint）；B 把 `release-version-tag.sh` 原样取出、对着假的 `docker` 跑了七格，在 `48bbfc99` 的 00 号文档副本上 `--check`、写入、再写入；C 把新的 `compose.yaml` 与 `compose-initdb.sh` 原样取出、镜像换成本机的，经 `heavy.sh` 装、迁移、带 `X-Forwarded-For` 登录、备份、`down -v`、恢复了三遍。主持者的第二次修订说明（`p5-revision-2.md`）：C 的 M12 留 P4-27；C 的 N14 SSE 一行也归 P5/S3；其余照核对者的改法改，Nit 全收。修订直接写进设计，每条改在哪里另列在 `p5-revision-1-done.md` 的"第二次修订"一节。

| 核对 | 原有的发现 | 新的 | 结论 |
|---|---|---|---|
| A | 19 条：I1、M1–M4、M6–M8、N1–N10 关上；M5 部分关上（3.9 还写着把脚本复制进临时目录） | Nit 2（N11、N12） | 没有新的 Important 与 Minor；2.1 里 A 那一块的 16 个标签与 `--markdown` 一字不差；`--check` 在 `48bbfc99` 上 64 处都过 |
| B | 20 条：M1–M6、N1–N12 关上，Q1、Q2 照决定落实；未核实的第 3、4 条部分落实 | Minor 1（M7）、Nit 3（N13–N15） | 没有新的 Important；`48bbfc99` 上 64 处写入、再写入不改；权限表的次序与 3.9 一致 |
| C | 25 条：24 条关上；M11 部分关上（P4 的修订正给小机器一档补量） | Minor 2（M12、M13）、Nit 4（N12–N15） | 没有新的 Important；新样例装与恢复都通：网关 `10.213.47.1`、会话记下转发来的 IP、`rolsuper=f`、`pg_trgm` 由 `nervewiki` 建得了、恢复之后属主 65532、字节相同 |

| 编号 | 级别 | 发现 | 处置 |
|---|---|---|---|
| M12P5A-M5（尾巴） | Minor | 3.9 的两个测试还写着把脚本复制进临时目录；复制成固定的 0755 时 g14 照过 | 照改：执行仓库里的 `deploy/*.sh`（绝对路径），工作目录设为临时的克隆或目录，不复制；写明这是 g14 成立的前提（3.9、3.13） |
| M12P5A-N11 | Nit | actionlint 的规则不是我们的，但"本机拒绝"是我们的代码，没有反向对照 | 照改：7.2 加 a1（去掉 `$(runner-only)`）、a2（判断挪到单独一行，`make -i`），都只用 PATH 里的假 `go`、不跑 actionlint；3.22 那一行照改 |
| M12P5A-N12 | Nit | 3.9 的 PATH 只列 `bash`、`git`、`jq`、`awk`，脚本还用 `dirname`、`grep`、`head`、`sed`、`cat` | 照改：假的 `gh`、`docker` 的目录在最前，其后接真的 `PATH`（3.9） |
| M12P5B-M7 | Minor | rc 第 5 步要"在本机以负责人的令牌或一次只读的工作流"记 `cosign verify --output json`：前者正是 00 号文档把核对放到 runner 上的理由，后者要第四份工作流或 `workflow_dispatch`，守卫会红 | 照改：`verify` 的 Signature 一步自己 `--output json`、以 `jq` 打出每份的 `critical.type`，只打出、不断言；第 5 步改为读 `verify` 的日志；`v0.1.0` 那一行只留"cosign 由负责人"；"默认把验过的载荷打到标准输出"写明是推断（3.14、3.23） |
| M12P5B-N13 | Nit | 主持者 Q1 引入的性质没有变异行 | 照改：7.2 加 w15（`release` 的 `needs` 去掉 `verify`）、w16（去掉 `--cert-identity`） |
| M12P5B-N14 | Nit | 00 号文档"次序与重跑"同一句里"`imagetools` 得到同一个摘要"写成事实、"只有产品的问题才出下一个 rc"与决定 324 不合 | 照改：脚本加 `3l-rerun-digest`、`3m-rerun-who` 两个标签（2.1 第 3 条） |
| M12P5B-N15 | Nit | 本记录头的"审查方式"漏了 B 用 WebFetch 的两次 | 照改（头） |
| M12P5C-M12 | Minor | P5 写小机器一档"推算、不要 P4 量"，P4 的修订正在批次 E 加一组（P4-27） | 主持者定：留 P4-27。`21a`、3.18 的那一行、3.18 第 1 条与第 9 节改为"P4/S3 批次 E 的小机器一组（P4-27），`GOMEMLIMIT=1536MiB`"；P4 里引的"P5 设计 3.17"现在是 3.18，主持者已告诉 P4 的设计者（2.1 第 21 条） |
| M12P5C-M13 | Minor | 三遍演练用 `sudo`，本机没有不要密码的 sudo；属主 65532 的 `data/` 删不掉 | 照改：3.20 写明演练借已有的 `postgres:18.7-trixie` 当 root 做 `chown`、`tar` 与清理（C 的 `drill.sh` 的写法），记录写明这是与文档唯一的出入；`deploy/README.md` 列出没有 sudo 时的写法（3.18、3.20） |
| M12P5C-N12 | Nit | 以 `postgres` 恢复之后 `pg_trgm` 的所有者变成 `postgres`；以 `nervewiki` 恢复同样成功 | 照改：恢复写 `pg_restore -U nervewiki`，与新装一致（3.18 第 4 条、3.20） |
| M12P5C-N13 | Nit | 备份之前先 `mkdir -p backups` | 照改（3.18 第 4 条） |
| M12P5C-N14 | Nit | P4 的修订（P4-26）把 `NOTIFY` 与 SSE 两行都交给 P5/S3；P5 只领了 `NOTIFY` 一行，SSE 一行改完后半句成了赘语 | 主持者定：SSE 一行也归 P5/S3。第 8 节第 5 条写出新文字；P5-23、2.1 第 20 条与脚本的 `19a`、`20a` 带上它 |
| M12P5C-N15 | Nit | "非超级用户建 `pg_trgm`"已在 18.7 上核对过，几处的"推断"可以改 | 照改：3.17、P5-16 与本记录"新的问题"照实写；演练那一格照留 |
| C 未核实的一条 | — | 钉住的子网与宿主机自己的网段重叠时，Docker 未必拒绝，可能让路由失效 | 写进 3.17（推断）；`deploy/README.md` 让用户第一次 `up` 之前看 `ip route`（3.18 第 2 条） |

这一轮作者多做的与取舍：

1. **`cosign verify` 的类型只打出、不断言**：B 只要求记下并打出类型。cosign v3 怎样列 `actions/attest` 的 bundle 没有核实，断言可能在 rc 上误报；rc 读了再定。
2. **3.9 的 PATH 接真的 `PATH`**：A-N12 给了"逐个 `LookPath`"与"接真的 `PATH`"两种，取了后一种，更短；假的命令在最前，照样盖住真的 `gh`、`docker`。
3. **脚本另改两处**：第 11 节 P5-16 一条写明 C 在 18.7 上装过、恢复过；第 13 节一行写进两次修订与修订核对的计数、"`NOTIFY` 与 SSE 两行归 P5/S3"与"小机器一档由 P4 另量"。改完之后 66 处，在 `48bbfc99` 的新副本上 `--check` 与写入都过，写入之后再跑什么也不写；与 P2 的脚本两种次序都过。
4. **"新的问题"删了三条**：`run:` 里的表达式（A 读过，只有 `matrix.leg`）、`pg_trgm`（C 核对过）、`secrets/` 属于 root（样例的注释已改成"只给部署的用户"）；加了子网与宿主机网段的一条。

## 修订核对第 2 轮

三位核对者各核对自己第一轮的新发现与第二次修订的设计、本记录"修订核对第 1 轮"一节与 00 号文档的脚本（`p5-check2-{a,b,c}.md`；证据在 `p5-check2-{a,b,c}/`）。A 从修订稿重新取出闸门，以绝对路径就地执行（不复制）跑 26 格与 g14，`make lint-actions` 的 a1、a2 用 PATH 里只记参数的假 `go` 做（没有跑 actionlint）；B 在 `48bbfc99` 的新副本上 `--check`、写入、再写入，与第一轮写出的副本逐处比；C 把 P2、P4、P5 三个脚本在 `48bbfc99` 的副本上按五种次序叠跑，照 3.20 没有 sudo 的写法另跑一遍 `chown`、`tar` 备份与恢复、借容器删目录。主持者的第三次修订说明（`p5-revision-3.md`）：三条 Nit 照改，这一轮之后不再请审查者。修订直接写进设计，每条改在哪里另列在 `p5-revision-1-done.md` 的"第三次修订"一节。

| 核对 | 上一轮的 | 新的 | 结论 |
|---|---|---|---|
| A | M5 的尾巴、N11、N12 关上：就地执行时 g14 让 26 格全变，说明脚本 0644 时经闸门的调用让 10 格变；a1、a2 照表抓到 | 无 | 没有新的发现；2.1 里 A 那一块的 16 个标签与 `--markdown` 一字不差；`--check` 在 `48bbfc99` 上 66 处都过；`ci.yml`、`vuln.yml` 的六处检出今天就是 `persist-credentials: false` |
| B | M7、N13–N15 关上 | Nit 1（N16） | 没有 Important 与 Minor；`48bbfc99` 的新副本上 66 处写入、再写入不改，与第一轮写出的副本只差第二次修订说过的几处 |
| C | 第一轮的新发现（M12、M13、N12–N15）与 M11 关上；子网与宿主机网段一条照旧未核实（会碰宿主网络，没试） | Nit 2（N16、N17） | 没有 Important 与 Minor；三个脚本五种次序叠跑都过，P5 再跑拒写；没有 sudo 的写法跑通，恢复之后属主 65532 |

| 编号 | 级别 | 发现 | 处置 |
|---|---|---|---|
| M12P5B-N16 | Nit | Signature 一步的 `jq` 在默认的 `bash -e` 下读不懂就让这一步失败（元素不是对象时以 5 退出，输出不是 JSON 时解析失败），与"只打出、不断言"不符 | 照改：`jq … || { echo "cosign 的输出不是预期的形状："; cat …; }`，校验仍由 `cosign verify` 的退出码断言（3.14 的 Signature 一步与要点） |
| M12P5C-N16 | Nit | 第 8 节第 5 条列"另四处"含第 14 节 `:1311`，同一条下面 SSE 一行又整句替换 `:1311` 所在的那一句：照前者先改，后者的锚点就找不到 | 照改：第 8 节第 5 条只列第 11 节三处，注明第 14 节那一处由 SSE 一行的整句替换完成；脚本的 `19a` 同样写明"不单独改"（2.1 第 19 条） |
| M12P5C-N17 | Nit | 恢复那一遍挪开的旧 `data/` 属主仍是 65532、`blobs/` 是 750，用户删不掉，清理只写了删 `data/` | 照改：3.20 的清理借 `postgres:18.7-trixie` 删 `data/` 与挪开的旧目录；恢复一步写明旧目录留到清理时删 |

这一轮作者多做的：

1. **脚本另改一处**：第 13 节一行写进三次修订与两轮修订核对的计数。改完仍是 66 处，在 `48bbfc99` 的新副本上 `--check` 66 处都匹配，写入 66 处，写入之后再跑什么也不写。
2. **设计的头**：状态写"设计已审查"，日期写三次修订、两轮核对。

## 主持者对第三次修订的核对

第 2 轮只有三条 Nit，没有 Important 与 Minor，所以第三次修订之后不再请审查者。主持者把修订前后的设计逐行比过（改前的快照 `p5-design-commit/p5-draft-before-r3.md`）：B 的 N16 的 `jq … || { …; }` 与 3.14 的说明、C 的 N16 的第 8 节第 5 条与 `19a`、C 的 N17 的清理与恢复一步都照处置改了，别处没有动。脚本在提交时的 `main`（P2 设计已在其上）上 `--check` 66 处都匹配。正式编号 430–452 在提交时给（上面两张表）。
