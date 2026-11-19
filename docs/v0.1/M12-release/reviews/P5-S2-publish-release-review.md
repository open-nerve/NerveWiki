# M12/P5 S2（推送、签名、证明、SBOM、许可证与 Release）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m12-p5-s2`。实施的 4 个提交，从 `main` 的 `ab766f02` 开始，审查时在 `e5ecae06`：<br>镜像里的第三方许可证 `/licenses` `d0876318`；演练取出许可证、生成两份 SBOM `785d48be`；版本号的 tag 与 Release 的两个脚本、`TestTheReleaseScripts` `54099f3e`；`sbom`、`publish`、`draft`、`verify`、`release` 五个作业与守卫 `e5ecae06`。<br>修复第 1 轮 3 个提交，头 `0fe3d663`：`/licenses` 收齐、不收代码、对着二进制与前端的 SBOM 核对 `dcaa7fe3`（决定 612–614）；守卫核对 `publish` 的签名与证明、`verify` 的校验与钉住的 action `e6f535aa`（616）；Release 的状态读 REST 的列表、bash 3.2、假命令记下每个参数的边界 `0fe3d663`（615）。<br>收口 1 个提交 `9a6b20ae`：核对第 1 轮的三条 Nit。<br>合并 `main` 一次：`2ea85cd5`（`305eec44`：P2/S4 与它的文档；带来的是 `web/`、`e2e/`、`docs/`、`tools/e2e-projects/check.mjs`、`server/internal/bootstrap` 两个文件里的注释与 asset 模块的 `signed_view_test.go` 和它的 testdata，没有冲突）。<br>分支共 8 个提交（不计合并与本提交），相对 `ab766f02` 12 个文件，加 1,716 行、减 26 行：`.github/workflows/release.yml`、`Makefile`、`knip.jsonc`、`deploy/Dockerfile`、`image-smoke.sh`、`release-version-tag.sh` 与 `github-release.sh`（新，可执行）、`platform/buildinfo/release_test.go`、`tools/licenses/` 的 `go.sh`（新，可执行）、`npm.mjs`、`check.mjs`、`README.txt`（新）；不加依赖，契约、迁移与网页不动，`docs/` 在本提交 |
| 规格 | [P5 设计](../05-P5-deploy-release.md)第 0 节 S2 一行，2.3 的决定 437–442，3.9 第 6 条，3.10–3.15，3.21–3.23，6.2，6.4，7.2，第 9 节"P5/S2"（决定 561、562）；[00 号文档](../00-M12-design.md) 4.10、4.11 |
| 审查方式 | 三位审查者（Opus）并行，各在自己分离的工作树里（A `nerve-wiki-m12-p5-rev-a`、B `nerve-wiki-m12-p4s5-rev-c`、C `nerve-wiki-m12-side-rev`，都挪到 `e5ecae06`），重的经 `heavy.sh`，不碰实施者的工作树；`gh` 只读；不推送、不登录、不提交：<br>A `/licenses` 与演练的三步（`d0876318`、`785d48be`）：普通的克隆里 `make image-smoke`，取出 `/licenses` 与本机直接跑两个脚本的输出比；`vite build --sourcemap` 读出打包的 117 个包与清单比；两个脚本在桩上的形状；镜像的变异 x1、x3 与 only-pgx-and-react；syft v1.54.0 与 sbom-action v0.24.3 的源码读 SBOM 的文档 `name`。<br>B `release.yml` 的五个作业与守卫（`e5ecae06`）：逐个作业对 3.14 与 4.10 的权限表，次序与重跑对 3.23 的失败表走一遍；3.21 的 SHA 以 `git ls-remote`、钉住的提交上的 `action.yml` 以 `gh api` 读；身份对 `release-notes.sh`；守卫的变异 36 个（备份不在 `.github/workflows/`）；一处 shell 的行为以假的 `cosign` 在本机跑。<br>C 两个脚本、提交、变异与文档（`54099f3e` 与四个提交）：逐个提交跑 6.2 那一组；`git archive` 的副本里脚本的变异；假的 `gh` 加开关的探针；`bash:3.2` 的容器；真的 `docker buildx imagetools inspect`（匿名、只读）与 `gh release view`；gh v2.103.2 的源码；四次运行的步骤时间与 `main` 的对照 |
| 日期 | 2026-11-19（实施、审查、修复、核对、收口、合并 `main`） |
| 结论 | Important 1：内嵌前端打包进去的 `openapi-fetch` 不在 `/licenses`（P5S2A-I1）。Minor 8：许可证文件名的正则收代码（P5S2A-M1）；清单完整没有东西核对（P5S2A-M2）；`publish` 的签名与证明、`verify` 的校验可以悄悄失效，守卫照过（P5S2B-M1、M2）；三处检查没有格（P5S2C-M1）；bash 3.2 上正式版本的草稿失败（P5S2C-M2）；"假命令在真的会失败的地方失败"两处不属实（P5S2C-M3）；一次查找的临时错误会被当作没有 Release、再建一个草稿（P5S2C-M4）。Nit 11：A 1、B 4、C 6；Q 3：A、B、C 各 1。三位都确认：写权限只在要写的作业，`run:` 里只有矩阵的表达式，十一个 action 钉 SHA、与 3.21 相同、都是最新版，次序与 4.10 一致，两处身份逐字；演练只读（artifact 0 个）；SBOM 的文档 `name` 照 syft 的源码；四个提交各自为绿。修复第 1 轮 3 个提交；核对第 1 轮：第一轮的都关上，新的只有 Nit 3 条（A、B、C 各一）。照负责人 2026-11-19 的规矩只剩 Nit 不再开核对：三条在 `9a6b20ae` 改，主持者核对。没有推任何 tag、镜像，没有建 Release，没有写 GHCR |

## 审查的发现与处置

编号照审查者的报告：P5S2 + 审查者（A、B、C）+ 级别与序号；核对第 1 轮的新发现写作 `C1-` 加级别与序号。审查时的代码位置是 `e5ecae06` 的，核对第 1 轮是 `0fe3d663` 的。修复照主持者的修复说明（`p5s2/fix-1.md`），收口照 `p5s2/finish.md`，在前一个头之上加提交，普通推送、不改写已推的历史，每个单独为绿。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P5S2A-I1 | 重要（`/licenses`） | `npm.mjs` 照设计 3.11 与决定 439 列 `pnpm --filter @nervewiki/web licenses list --prod`：pnpm 不往工作区包 `@nervewiki/api-client` 里走，它的生产依赖 `openapi-fetch` 0.17.0（MIT）打包进了前端的主块（sourcemap 读出），镜像与附件里却没有它的许可证。压缩去掉了所有许可证注释，`/licenses` 是这份声明唯一能在的地方；前端的 SBOM 里有它，`make image-smoke` 照过 | 决定 612，`dcaa7fe3`：`--filter "@nervewiki/web..."`，今天 232 项，加 tailwindcss 共 233 行，`openapi-fetch@0.17.0`、`openapi-typescript-helpers@0.1.0` 在内。退回旧的 filter 由 `tools/licenses/check.mjs` 的 npm 三格（l10）、`image-smoke.sh` 的 `openapi-fetch` 一行（f1）与 Check the SBOMs 各自抓到。核对 A：关上（第一轮读出的打包的 117 个包都在新的清单里） |
| P5S2A-M1 | 中（`/licenses`） | 文件名的正则后缀 `([.-].*)?` 收代码：cytoscape 的 `license-update.mjs` 进了镜像，INDEX 写 `LICENSE,license-update.mjs`；桩上一个根目录里只有 `license.go` 的模块让 `go.sh` 以 0 退出，"没有许可证文件就失败"在这种形状上不成立 | 决定 613，`dcaa7fe3`：两侧同一条，排除 13 种代码与数据的扩展名，另收 `unlicense`；`check.mjs` 的桩有 `license.go` 与 `license-update.mjs` 两种，l1、l3、l8 红。核对 A：关上（`/licenses` 里没有一个代码或数据扩展名的文件；`license.go` 的模块失败） |
| P5S2A-M2 | 中（`/licenses`） | `image-smoke.sh` 只抽查 pgx 与 react：两侧清单删到只剩 pgx、Go 本身与 react（only-pgx-and-react），`make image-smoke` 照过；I1 就是这样漏过去的 | 决定 614，`dcaa7fe3`：`go.sh` 对着 `go version -m <程序>` 比模块与 Go 的版本；演练与 `sbom` 作业的 Check the SBOMs 对着前端 SBOM 比；`image-smoke.sh` 加 `openapi-fetch` 一行与行数的下限。镜像的 f1–f5 都红（f3、f4 让 `make image` 失败）。核对 A：关上 |
| P5S2A-N1 | 细节（注释） | `go.sh` 的"子目录里的是第三方代码自己带的，随它的模块另算"：抄进子目录的第三方代码不是另一个模块，不会另算 | `dcaa7fe3`：照审查 A 的建议写成事实（只看模块的根目录；今天链接进来的包所在的子目录里没有另外的许可证，审查 A 核过）。核对 A：关上 |
| P5S2A-Q1 | 问 | Tailwind 的 preflight 与默认主题编进前端的 CSS，它是 devDependency，不在 `/licenses`；vite 的 modulepreload polyfill 也在主块里 | 决定 612：固定收 tailwindcss（`npm.mjs` 的 `SHIPPED`，没装就失败；l11、l12 红）；polyfill 不收（"接受与推后的"）。核对 A：关上 |
| P5S2B-M1 | 中（守卫） | 守卫对 `publish` 只核对"版本号的 tag 是最后一步"：Version tag 一步加 `if: always()`（b5）、删掉 Sign（b6）、Sign 加 `if: false`（b7b）、删掉 Provenance（b7），都照过，`0.1.0` 可以在没签名、没证明时出现 | 决定 616，`e6f535aa`：`publish` 的步骤都不写 `if`；恰一步逐字的 `cosign sign --yes "$IMAGE@$DIGEST"`，`DIGEST` 取索引的摘要；恰三步 `actions/attest`，摘要、推送与 `sbom-path` 的集合都核对。b5–b7b 与自加的 p1–p5 都红。核对 B：关上（另写的 n3、n10 也红） |
| P5S2B-M2 | 中（守卫） | "两种架构上核对过才公开"可以悄悄失效：`verify` 的校验步骤写 `if`（b8、c4）或 `shell: bash {0}`（b9：`cosign` 失败、`jq` 对空的输入以 0 退出，这一步绿）、矩阵只剩 amd64（b11）、`draft` 自己公开 Release（b19），都照过 | 决定 616，`e6f535aa`：`verify` 的每个跑命令的步骤同跑 smoke 的只有 name、env、run，没有 `set +…`，作业与工作流不写 `defaults.run.shell`；矩阵恰是 amd64、arm64，`strategy` 与 `build` 的相同；只有 `release` 公开 Release。b8、b9、c4、b11、b19 与自加的 v1、v2 都红。核对 B：关上（另写的 n7 只改 `fail-fast`，也红；它的消息见 P5S2B-C1-N1） |
| P5S2B-N1 | 细节（守卫） | `cosign verify` 的身份没有守卫：换成 `--certificate-identity-regexp '.*'`（b10）照过；`identity=` 一行的值也不核对（b16，rc 上 fail-closed） | 决定 616，`e6f535aa`：`identity=` 一行与唯一一步 `cosign verify` 的身份逐字，带 Actions 的签发者，整个 `verify` 里没有 `-regex`；b10、b16、b16b、n1、n1b 红。核对 B：关上 |
| P5S2B-N2 | 细节（守卫，决定 561） | 只读作业级的 `env.SHA`：两步各自写 `SHA: ${{ github.sha }}`（b23、b24）或 HEAD 的核对一步 `if: false`（b12），都照过，又回到决定 561 要避开的推断 | `e6f535aa`：HEAD 的核对一步的键恰是 name、run，Index 一步恰是 id、name、run；b12、b23、b24 红。核对 B：关上 |
| P5S2B-N3 | 细节（守卫，决定 562） | 关掉 `-e` 的检查只认行首的 `set`：`true; set +e`（b15）、`shopt -ou errexit`（b15b）照过；注释与报告读起来比实际宽 | `e6f535aa`：也认 `;`、`&`、`\|`、`{`、`(` 之后的 `set` 与 `shopt -o`，注释写明 `{ …; } \|\| true`、`BASH_ENV`、`trap` 照 562 不追；b15、b15b、b15d–b15f 红。核对 B：关上（`if :; then set +e; fi`、`builtin set +e` 照过，与注释相符，照 562 不追） |
| P5S2B-N4 | 细节（注释） | `release.yml` 三处注释把"tag 上镜像的 SBOM 的 `name` 是 `$IMAGE`"写成事实，它由 syft 的源码推出 | `e6f535aa`：三处加"推断：`sbom` 作业先核对（或"这一步先核对"），rc 上证明"。核对 B：关上 |
| P5S2B-Q1 | 问 | 没有测试核对 `uses:` 钉 SHA；`publish` 的第三方 action 按前缀放行，`sigstore/cosign-installer@v4`（q1）照过，那个作业有 `id-token: write` 与 `packages: write`。S1 起就这样 | 决定 616，`e6f535aa`：三份工作流的每个 `uses:`（步骤的与作业的）要 `<owner>/<repo>[/path]@<40 位十六进制>`，今天 52 个都钉了；q1–q1d 红。核对 B：关上（另写的 n2、n8、n9 也红） |
| P5S2C-M1 | 中（测试） | 三处检查没有一格让它单独变红：未知的动作（c1）、草稿的 `edit` 失败（c2）、`upload` 失败（c3），各去掉一处 27 格全绿；报告与 `54099f3e` 的说明却说"每一支都有测试" | `0fe3d663`：假的 `gh` 加 `FAKE_GH_EDIT_FAILS`、`FAKE_GH_UPLOAD_FAILS`，加三格；c1–c3 红。等价的、上游已挡住的几处列在"接受与推后的"；说明的不准见"更正"。核对 C：关上 |
| P5S2C-M2 | 中（可移植） | `github-release.sh` 的 `"${prerelease[@]}"` 在 bash 3.2（macOS 的 `/bin/bash`）的 `set -u` 下、数组为空时报 unbound variable：正式版本的草稿失败，`make test-go-short` 在那里红。runner 上是 bash 5，发布不受影响 | `0fe3d663`：两处写成 `${prerelease[@]+"${prerelease[@]}"}`，`paste -sd' ' -`；两个脚本另 grep 过 bash 4 的写法。`bash:3.2` 的容器里 31 格全过，退回时红（只手跑，不进 CI）。核对 C：关上（同一个容器里重跑 31 格） |
| P5S2C-M3 | 中（替身与说法） | "假命令在真的会失败的地方失败"有两处不属实：`imagetools create -t` 对已在的 tag 真的会改指；gh v2.103.2 的 `release create --draft` 不查已有的草稿（`create.go:503-511`）。假的在这两处失败，是绊线 | `0fe3d663`：`fakeDocker`、`fakeGHRelease` 的注释分开写照真的与绊线；P5 设计 3.25 第 5 条照此写。注释里"真的会再建一个草稿"仍写成了事实，见 P5S2C-C1-N1。核对 C：关上 |
| P5S2C-M4 | 中（决定 441 的前提） | gh v2.103.2 的 `view`、`edit`、`upload` 经 `FetchRelease` 并行查 REST（只找已公开的）与 GraphQL（找草稿），两次都失败、其中一次是 not found 时报 `release not found`：第 1 次尝试建了草稿之后重跑 `draft`、GraphQL 一时答 502，`state()` 当作没有，再建一个草稿，正是决定 441 要防的 | 决定 615，`0fe3d663`：`state()` 读 REST 的列表，任何失败都失败，多于一个也失败；假的 `gh` 答三页、每页各跑一次查询，加"列表答 502""同一个 tag 两个 Release"两格；s1–s6 红。核对 C：关上（另以八种探针核过分页、半途失败、前后缀与 `null`） |
| P5S2C-N1 | 细节（提交说明） | `e5ecae06` 说"the rehearsal of the previous commit wrote name=nervewiki"：前一个提交 `54099f3e` 没有单独推过，读出它的是 `785d48be` 的演练（run 44451246873） | 已推，不改写，见"更正"。核对 C：关上 |
| P5S2C-N2 | 细节（草稿） | 草稿 3.25 第 6 条说 `release-version-tag.sh` 的写法同代码块：它多了三处；实施报告 8.3 把代码块原有的 `manifest unknown` 一支算作自己加的 | 本提交：3.25 第 6 条写出多的三处；报告的那一句见"更正"。核对 C：关上 |
| P5S2C-N3 | 细节（草稿漏的） | 00 号文档第 7 节修订第 6 条的"后一句随 P5/S2"与两份文档头的状态；10.2 没有 `image` 一项的 Image smoke 一步（7.5）与它的基点；第 8 节第 3 条与 3.22 仍写 w1–w14 | 本提交照写。核对 C：关上 |
| P5S2C-N4 | 细节（数） | 草稿 10.2 的"演练加的三步约 22 秒"只是 `785d48be` 的（头 15 秒）；06-environment 草稿的"根分区从 23G"是 24G，"每次多留几百 MB 到 1 GB"是平均约 0.5 GB | 本提交照实写；见"更正"。核对 C：关上 |
| P5S2C-N5 | 细节（rc 才证明得了的） | 第 9 节"发布"的几条可以照 C 的探针升级：GHCR 对不存在的 tag 的答复、`{{json .Manifest}}` 的 `.digest`、`gh release view` 读不存在的 tag、`FetchRelease` 找得到草稿（源码，不只由二进制推出） | 本提交：第 9 节"发布"与 3.13 照写，仍要 rc 证明的写明。核对 C：关上 |
| P5S2C-N6 | 细节（替身） | 假的 `gh`、`docker` 以 `"$*"` 记调用，参数的边界丢了：`--title Nerve Wiki "$tag"`（c15）27 格全绿，真的 gh 会把 `Wiki` 当作附件 | `0fe3d663`：两个假命令以 NUL 分隔记"个数、各参数"，读回时含空白或引号的照 Go 的写法加引号；c15 红。核对 C：关上 |
| P5S2C-Q1 | 问 | 小决定的编号从哪里起，哪几条编号 | 主持者大体照 C 的建议（`fix-1.md`；C 原说决定 561 的两处守卫并进 561，这里也并进 616）：守卫新加的各条并进 616，SBOM 的 `name` 先核对是 617；`GH_REPO`、读各包的 `package.json`、562 写成三个键不编号（"实施中定的"）；号接在 P2/S4 的 604–611 与 P2/S5 之后。核对 C：关上 |
| P5S2A-C1-N1 | 细节（测试；核对第 1 轮） | 去掉 `go.sh` 里 `go list` 之后的 `sort -u`，桩与镜像都照过，`go/INDEX.tsv` 成了 157 行（`rivershared` 重复 19 次）：真的 `go list -deps` 每个包一行，假的 `go` 每个模块一行；比较的一步自己 `sort -u`，许可证文件不缺 | `9a6b20ae`：假的 `go` 照真的每个包一行，每个模块三行、按依赖的次序（不排序）；去掉 `sort -u`、只 `sort`、只 `uniq` 三种都让第 1 格红，改之前的桩上都照过。主持者核对 |
| P5S2B-C1-N1 | 细节（守卫的消息；核对第 1 轮） | 只有 `strategy` 的其余部分与 `build` 不同时（n7，只改 `fail-fast`），消息是"verify runs on [amd64 arm64], want [amd64 arm64], its strategy build's"，两个列表一样，看不出哪里不对 | `9a6b20ae`：架构与 `strategy` 分两句报，后一句打出两边的 `strategy`；n7 报"verify's strategy is map[fail-fast:true …], build's map[fail-fast:false …]: want build's"，b11 两句都报。主持者核对 |
| P5S2C-C1-N1 | 细节（推断写成事实；核对第 1 轮） | "gh release create --draft 会再建一个草稿"写成了事实：核过的只有 gh 不查已有的草稿（源码）；接口会再建一个，是设计 3.13 的前提（审查 B 的 M1），没有核过。四处：`release_test.go` 的注释、`0fe3d663` 的说明、草稿 3.25 第 5 条与"更正"第 2 条 | `9a6b20ae`：注释改为"gh release create --draft 不查已有的草稿（它的源码，v2.103.2）；GitHub 再建一个是 P5 设计 3.13 的前提，只有 tag 的运行证明得了"；本提交的 3.25 第 5 条与下面的"更正"照此写；`0fe3d663` 的说明见"更正"。主持者核对 |

## 主持者的决定

负责人可以改判；写进 [00 号文档](../00-M12-design.md)第 11 节"2026-11-19：P5/S2 的决定 612–617"。都在修复第 1 轮定（`fix-1.md`）：612–615 改设计 3.11、3.13 与决定 439、441，616、617 是设计之外、别的 Step 要照做的守卫与核对（审查 C 的 Q1 与它的"报告第 9 节第 4 条"）。实施中定的不编号（下一节）。

- **612 npm 一侧的许可证取 `@nervewiki/web` 与它用的工作区包的生产依赖，另固定收 tailwindcss**（审查 A 的 I1、Q1）：`npm.mjs` 列 `pnpm --filter "@nervewiki/web..." licenses list --prod --json`。只写 `@nervewiki/web` 时 pnpm 不往工作区包里走，带不带 `--prod` 都一样（审查 A：不带时 330 项，也没有 `openapi-fetch`）；Dockerfile 本来就复制了 `api-client` 的 `package.json`、`pnpm install --filter @nervewiki/web...`，改的只是这一行。tailwindcss 是 devDependency，它的 preflight（8,489 字节的源文件）与默认主题编进 `assets/index-*.css`，只留一行版权注释：随产物发出的就要收，`SHIPPED` 写明理由，从 `web/apps/web/node_modules` 读（pnpm 为前端装的那一份），没装就失败。vite 的 modulepreload polyfill 是构建工具的运行时片段，通常不算，不收（"接受与推后的"）。今天 233 行（232 项加 tailwindcss），`dcaa7fe3`。
- **613 许可证文件的名字不收代码，另收 `unlicense`**（审查 A 的 M1）：两侧同一条 `^((un)?licen[cs]e|copying|notice|patents)([.-].*)?$`（不分大小写），再排除以 js、cjs、mjs、jsx、ts、cts、mts、tsx、go、json、map、sh、py 结尾的名字（`go.sh` 的 `code`、`npm.mjs` 的 `CODE`）。只列扩展名而不把后缀限定成 `.md`、`.txt` 一类：`LICENSE-MPL`、`LICENSE-APACHE`、`COPYING.LESSER` 这样的写法都要收；`unlicense` 照决定 438 的"宁多不少"收。今天的差别只有 cytoscape 的 `license-update.mjs` 不再收；Go 一侧 48 行与改前逐字节相同（`dcaa7fe3`）。
- **614 两侧的清单对着发出去的东西核对**（审查 A 的 M2）：靠"脚本这样写所以完整"是推断，I1 就是这样漏的。
  - Go：`go.sh <程序> <输出目录>` 收完之后，`go list` 的模块要恰是 `go version -m <程序>` 的 `dep` 行，程序的第一行要是 `<程序>: <收的 Go 的版本>`，不等就失败、列出两边各多的，构建失败（server 阶段）。
  - npm：演练与 `sbom` 作业的 Check the SBOMs（两处逐字相同）要前端 SBOM 里 `@nervewiki/*` 之外的每个 `名@版本` 都在 `/licenses/npm/INDEX.tsv`；不在的 `@types/` 只打出名字、不算缺。今天是 `@types/trusted-types`，dompurify 的可选依赖：只有类型声明，构建丢掉；pnpm 的 `licenses list --prod` 不走各包的可选依赖（`--optional` 只列工作区自己的，0 项），收它要自己解析可选依赖，平台相关的可选依赖会让两种架构的 `/licenses` 不同。
  - 本机一侧的底线：`image-smoke.sh` 加 `openapi-fetch` 一行（带它的理由）与两份清单至少 40、200 行（今天 48、233）。
- **615 Release 有没有、是不是草稿读 REST 的列表**（审查 C 的 M4）：`gh api --paginate "repos/$GH_REPO/releases?per_page=100" --jq '.[] | select(.tag_name == "<tag>") | .draft'`：空是没有，`true` 是草稿，`false` 是已公开；任何失败都失败，同一个 tag 多于一个或读不懂也失败（"多出来的先在 GitHub 上删掉"）。`tag` 进查询之前已过正则，改变不了查询。两点核对（实施者，修复第 1 轮）：列表含草稿：GitHub 文档"Only users with push access will receive listings for draft releases"，`draft`、`release` 两个作业是 `contents: write`，"它算 push access"是推断，rc 上证明；`--paginate` 与 `--jq` 合用时每一页各算一次查询（gh v2.103.2 `pkg/cmd/api/api.go` 的分页循环每页调一次 `processResponse`；`--slurp` 不能与 `--jq` 同用），所以查询写成逐个元素的，每页各出零到几行；某一页失败时 gh 以非 0 退出（核对 C 读源码另核）。附件仍由 `gh release view --json assets` 读：它一时说 not found 时，"附件恰是三个"以"读不到附件"失败，失败即安全（`0fe3d663`）。
- **616 守卫另核对的几条**（实施报告第 8 节第 1、2 条；审查 B 的 M1、M2、N1–N3、Q1；审查 C 建议并成一条）：
  - `publish`：最后一步是版本号的 tag；第三方的 action 只有 cosign-installer（按前缀判，大小写不同的写法也挡）；步骤一个也不写 `if`（同 `gate`）；恰一步逐字的 `cosign sign --yes "$IMAGE@$DIGEST"`，`DIGEST` 取 `steps.index.outputs.digest`；恰三步 `actions/attest`，`subject-digest` 都是它、`push-to-registry: true`，来源证明与两份 SBOM 各一步。
  - `verify`：每个跑命令的步骤只有 name、env、run（同决定 562 对跑 smoke 的步骤）；矩阵恰是 amd64、arm64，`strategy` 与 `build` 的相同；每一条 `gh attestation verify` 带 `--cert-identity "$identity"`，来源证明与 SBOM 各至少一条；`identity=` 一行与唯一一步 `cosign verify` 的 `--certificate-identity` 逐字是 tag 上的 `release.yml`，带 Actions 的签发者，整个作业里没有 `-regex`、`-regexp`。
  - 只有 `release` 公开 Release：别的作业的 `run` 不含 `github-release.sh publish`、`--draft=false`、`gh release`。
  - 决定 561：`build` 在构建之前核对 `HEAD` 是 `needs.gate.outputs.sha`，`publish` 核对镜像的 `revision`；两步只有 name、run（Index 加 id），不写自己的 `env`、`if`。
  - 三份工作流的每个 `uses:`（步骤的与作业的）都是 `<owner>/<repo>[/path]@<40 位十六进制>`（总体设计 13.5 第 2 条）；P5/S3 与以后改工作流的照它写。
  - 理由：4.10 的次序（版本号的 tag 出现时已签好、证明好；两种架构上核对过才公开）只由守卫在 rc 之前看得见，步骤级的 `if`、`shell`、删掉一步或少一种架构都让它悄悄失效（05-lessons 第 7 节：跳过与照绿都等于没有）。有意绕开的写法照 562 不追（"接受与推后的"）。`e5ecae06`、`e6f535aa`。
- **617 SBOM 的文档 `name` 在不推送的地方先核对**（实施报告第 8 节第 6 条；审查 C 的 Q1）：演练要镜像的那份是 `nervewiki`、`sbom` 作业要 `$IMAGE`，前端的都要 `@nervewiki/web`。syft 换了命名时，tag 的运行停在只读的 `sbom` 作业，不到 `verify` 才红（那时版本号的 tag 已经出现，3.23 的表里只能删草稿、出 rc.N+1）。镜像的那份是 `$IMAGE` 而不是设计推断的 `$IMAGE@sha256:…`，是 3.12 预先准许的偏离，写进 3.25 第 1 条，不另编号：`785d48be` 的演练读出 `nervewiki`；syft v1.54.0 的 `Describe()` 取 `reference.Parse(UserInput).Name()`，不带 tag 与摘要，`DocumentName` 先取它，sbom-action 原样交 `image`（审查 A 读源码另核），所以 tag 上是 `ghcr.io/open-nerve/nervewiki`，rc 上证明（`e5ecae06`）。

## 实施中定的

实施报告第 7–9 节、修复第 1 轮的报告第 9 节与收口（三位审查者各在自己的范围里判过，主持者接受）：

- **`github-release.sh` 要 `GH_REPO`**：设计写"仓库取 `GH_REPO`"，不设时 gh 照检出的 remote 去猜；另核对 tag 的写法与说明文件不空，传过之后与公开之后各再读一次。
- **`npm.mjs` 的名字与版本读每个包自己的 `package.json`**：pnpm 的 `versions` 与 `paths` 今天按位置对得上，但那不是它写下的契约。
- **决定 562 写成"只有 name、env、run 三个键"**：照 562 说的"可照 actionlint 那一步的整个比"，比设计列的 `shell`、`defaults.run.shell`、`set +e` 严：`if`、`working-directory`、`continue-on-error`、`timeout-minutes`、`id` 都不许，以后给 smoke 一步加时限要先改守卫（审查 B 核过）。S1 的"不写 `if`"并进它；S1 的 b9、k4、fn1 现在由这一条抓到。
- **`build` 的 HEAD 核对是单独一步"The commit is the gate's"**，在"The tree is clean"之后、Tools 之前（设计写"Labels 之前"，这样失败得更早）；`verify` 的 Tools 多查 `gh`。
- **`TestTheReleaseScripts` 比 3.13 的表多的格**：版本号的 tag 多 `manifest unknown` 一格（那一支是代码块原有的，多的只是格）；Release 的附件目录少一个与多一个分开，另有已是草稿却有别名的附件、没有说明、tag 的写法、没有 `GH_REPO`、公开之后仍是草稿、已公开却缺附件；修复第 1 轮再加五格（"列表答 502"代替"view 答 502"），共 31 格。
- **`image-smoke.sh` 比代码块多核对的**：`README.txt` 在、`LICENSE` 与仓库的逐字节相同、npm 一侧每行三列都在，pgx 与 react 两行带着文件名。
- **`tools/licenses/check.mjs`**（修复第 1 轮）：审查 A 建议的"桩上加两格"落成常驻的格，`make lint-web` 调用（与 `tools/*/check.mjs` 同一类），对着假的 `go`、`pnpm` 跑 `go.sh` 与 `npm.mjs`，10 格；要 bash 与 Node，不要 Go 与构建；`Makefile`、`knip.jsonc` 各一行。x2 由它的一格常驻。收口时假的 `go` 照真的每个包一行（P5S2A-C1-N1）。
- **`go.sh` 的参数改为 `<程序> <输出目录>`**，Go 的版本也比；不同的模块以 `comm` 列出：busybox 的 `diff` 默认出统一格式、不出 `<`、`>`（`bash:3.2` 的容器里看到的），推送之前 amend 进了 `dcaa7fe3`（旧号 `f7fef524`、`db77ee98` 没推过）。
- **Check the SBOMs 的核对在 `LC_ALL=C` 之下 `sort`、`comm`**：本机 en_US.UTF-8 下 `comm` 报"not in sorted order"。只豁免"不在清单里的 `@types/`"：前端 SBOM 里 33 个 `@types/`，32 个经 `licenses list --prod` 已在清单里（`@types/d3*` 等是 mermaid 的生产依赖）；最初写成整个 `@types/` 不核对，本机只核对了 200 个，改了。
- **假的 `gh` 答三页**，这个 tag 的混在别的 tag 之间，查询由 `jq` 真跑：错的查询由行为抓到（s3–s5），不只靠比调用。
- **假命令记参数用 NUL 分隔的"个数、各参数"**：`fix-1.md` 说 `printf '%q '` 或每个参数一行；`%q` 的转义在 Go 一侧不好对，每行一个遇到空参数与换行有歧义。
- **第 4 个提交推送之前 amend 过一次**（`7e62461b` → `e5ecae06`，旧号没推过）：第 2 个提交的演练读出镜像 SBOM 的 `name`，照 3.12 改 `verify` 的那一行。

## 接受与推后的

- **接受**：
  - vite 的 modulepreload polyfill 随主块发出，是构建工具的运行时片段，不收它的许可证（审查 A 的 Q1）。
  - 前端 SBOM 里不在 `/licenses` 的 `@types/` 只打出名字、不算缺（决定 614）。
  - 只有 `UNLICENSE` 以外的 REUSE 式 `LICENSES/` 目录的模块让构建失败：失败而不是放过（审查 A 的 M1 第 2 点）。
  - 守卫不核对 `actions/attest` 的 `subject-name` 与 `verify` 的 `runs-on`；`verify` 的 cosign-installer 写 `if: false`（核对 B 的 n11）、`identity=` 之后另赋一次别的工作流（n5）：写错了 rc 上 `verify` 红，fail-closed。
  - 照决定 562 不追的有意写法：`{ smoke; } || true`（b14）、HEAD 与 revision 的核对后加 `|| echo`（b17、b18）、来源证明一条 `|| true`（b21）、`if :; then set +e; fi` 与 `builtin set +e`（核对 B 的 n4、n4b）、`BASH_ENV`、`trap`。守卫的注释写明只认行首与 `;`、`&`、`|`、`{`、`(` 之后的 `set`。
  - `build` 的矩阵只剩 amd64（b11c）：`sbom` 的 Digests 读不到 arm64 的摘要就失败；删掉"The version tag is the index"（b13）影响小，后面的校验都按 tag 走；去掉 `draft` 的 `GH_REPO`：脚本第一步就拒。
  - 脚本里等价的、上游已挡住的变异（审查 C）：`create` 失败不管（之后 `upload` 失败）；参数个数（`set -u` 先失败）；`publish` 多一个参数；rc.0 与前导 0（闸门的正则先拒）；`-s` 改 `-e`（`release-notes.sh` 不交空节）；`not found` 不分大小写（旧的 `gh release view` 写法上的，修复之后那段代码已删）；`isDraft` 读不懂（gh 只答 true 或 false；修复之后读列表，`null` 也失败，核对 C）；读附件失败；去掉 `grep` 的 `-i`；jq 读不懂；`create` 的参数不加引号。`go.sh` 的 `[ -n "$modules" ]` 与 `|| fail`（核对 A）。
  - 附件仍由 `gh release view --json assets` 读：一次查找的临时错误会说 not found，在"附件恰是三个"那里以"读不到附件"失败（核对 C）。
  - b32（`create` 一处退回 `"${prerelease[@]}"`）在 bash 5 上照过：只有 bash 3.2 抓得到，`bash:3.2` 的容器只手跑，不进 CI。
  - `release-version-tag.sh` 的 `not found` 子串偏宽（凭证助手的 "executable file not found" 等）：这些情形下 `create` 多半同样失败，没造出 `create` 成功而改指的情形（审查 C 未核实的第 4 条）。
- **推断、没核实的**（只有推 tag 才证明得了的，P5 设计 3.23 与第 9 节"发布"）：
  - tag 上镜像的 SBOM 的 `name` 是 `ghcr.io/open-nerve/nervewiki`（决定 617）；
  - REST 的 Release 列表对 `contents: write` 的令牌含草稿（决定 615）；GitHub 收下同一个 tag 的第二个草稿（设计 3.13 的前提；修复之后脚本不再去建它）；
  - runner 上 gh 的版本（arm 的 runner 的软件表是 2.86.0）与它的 `attestation verify` 的参数；`publish` 的 shell `docker login` 写进 `~/.docker/config.json`、`actions/attest` 与 cosign 由此读凭证；重跑的作业能不能下载前一次尝试的 artifact；`cosign verify` 对着 referrers 里也有 `actions/attest` 的 bundle 时怎样；`imagetools create` 只给一个来源时原样复制（审查 B 未核实的五条）；
  - `sbom` 作业两种架构的 `/licenses` 逐字节相同（本机 arm64 的镜像与 CI amd64 的演练文件数、字节数相同）；
  - 登录之后读自己的私有包时，GHCR 对不存在的 tag 的答复（匿名读公开的包已核：`not found`）；
  - macOS 上 BSD 的 `paste`，与 `check.mjs` 在 macOS 的 bash 3.2、BSD 的 `comm`、`sort` 上：没有跑过（审查 C 未核实的第 1 条、核对 C 未核实的第 1 条）。
- **推后、交出去的**：
  - 合并之后两次 `main` 的组合根、short 的 Test 一步与 `main` 上的演练（设计 7.5、第 8 节第 1 条），主持者在记合并号的文档提交里补。
  - 合并之前看一眼 `vuln.yml` 在 `main` 上最近一次的结论（设计 6.4）。
  - 跳过的矩阵作业不展开名字（分支上 `build`、`verify` 两个），只是外观，不改。

## 更正

已推的提交说明不改写；不准处写在这里：

- `54099f3e` 的 "they are scripts with a test for every branch"：三处检查（未知的动作、草稿的 `edit` 失败、`upload` 失败）没有格，`0fe3d663` 加了（P5S2C-M1）。
- `54099f3e` 的 "The fakes fail where the real ones would: create over a tag that is there, a second Release of a tag"：这两处是绊线：`imagetools create` 对已在的 tag 真的不失败、会改指；`gh release create --draft` 不拦，不查已有的草稿（gh v2.103.2 的源码），GitHub 会不会再建一个是设计 3.13 的前提，没有核过（P5S2C-M3、C1-N1）。
- `e5ecae06` 的 "the rehearsal of the previous commit wrote name=nervewiki"：读出它的是 `785d48be` 的演练（run 44451246873），前一个提交 `54099f3e` 没有单独推过（P5S2C-N1）。
- `0fe3d663` 的 "Five more cases of the Release"：其中"列表答 502"一格代替了原来的"view 答 502"，净多 4 格（18 → 22）；"gh would make a second draft" 把设计 3.13 的前提写成了事实（P5S2C-C1-N1）。
- 实施报告 8.3 把 `release-version-tag.sh` 的 `manifest unknown` 一支算作自己加的：代码块原有，多的只是那一格（P5S2C-N2）；8.4 与草稿第 1 版 3.25 第 5 条的"假命令在真的会失败的地方失败"，改为 P5 设计 3.25 第 5 条的两种。
- 实施报告第 5 节"S1 之前的 `main` 是 68–80 s"不是基点自己的数：`ab766f02` 81 秒、`105ff7c8` 59 秒（P5S2C-N3）；草稿第 1 版的"演练加的三步约 22 秒"只是 `785d48be` 的，`e5ecae06` 15 秒；"根分区从 23G"是 24G；"每次多留几百 MB 到 1 GB"是平均约 0.5 GB（P5S2C-N4）。

## 变异与反向对照

- **设计 7.2 里 S2 的**（实施者，头 `e5ecae06`；脚本的用 `cellmut.py`，守卫的用 `gmut.py`，镜像的用 `lxmut.sh`，备份都在 `~/.cache`、不在 `.github/workflows/`，先在没改过的树上跑同样的命令）：w9、w13–w16（w16b）、r1–r7、x1、x3 都抓到；x2 在桩上（第 1 个提交之前手做，修复第 1 轮起由 `check.mjs` 的一格常驻）。
- **实施者自加**：守卫的 44 个（表的权限、`if`、`needs` 与表之外的作业；smoke 的 12 个；`publish` 的 5 个；`release`、`verify` 与决定 561 的 6 个）都由该红的那一条抓到；`TestTheReleaseScripts` 的 r8–r29，27 格每格至少一个变异让它红；镜像的 l1–l4（照 S1 修复之后的 `make image` 改了锚点）、x1b、x3b–x3d、x4 都抓到；S1 改工作流的 71 个对着新的守卫与 `release.yml` 再跑，都抓到（`b4b-gate-head-commit` 的锚改写）。
- **先红后绿**：脚本不在时 `TestTheReleaseScripts` 27 格全红；守卫的新表对着第 3 个提交的 `release.yml` 报 12 条。
- **审查 A**：x1、x3 抓到；x2 的桩原样以 1 退出；只有 `license.go` 的桩以 0 退出（M1）、only-pgx-and-react 照过（M2）。
- **审查 B**：36 个：b1–b4、b15c、b20、b22、c1–c3、b24c 抓到；b5–b7b（M1），b8、b9、b11、b13、b19、c4（M2），b10、b16（N1），b12、b23、b24（N2），b15、b15b（N3），q1（Q1）照过；b11c fail-closed，b14、b17、b18、b21 有意削弱、照 562 不报，b16b 写坏了、作废。
- **审查 C**：r1–r7 与实施者的 r11、r15、r18、r21、r24 重跑，照报告；守卫抽 w9、w13–w16 等，都抓到；c1–c3（M1）、c15（N6）照过；c4–c16 里等价的列在"接受与推后的"。
- **修复第 1 轮**（实施者）：
  - `/licenses`：`check.mjs` 的 l1–l13 都红，修之前的两个脚本上 10 格全红；镜像的 f1–f5（普通的克隆）都红；Check the SBOMs 的完整性一段（本机照工作流的原文、`bash -e` 跑）对着旧的 230 行 INDEX 红、列出 `openapi-fetch@0.17.0`、`openapi-typescript-helpers@0.1.0`，对着新的 233 行绿。
  - 守卫：审查 B 的 b5–b24、q1 与另加的 p1–p5、v1、v2、b15d–b15f、q1b–q1d、n1、n1b，共 33 个，修之前都照过、修之后都红；实施时的 44 个、S1 的 71 个照旧红。
  - 脚本：c1–c3、c15、s1–s6 都红；实施时的 30 个 28 个红，r5 的代码已删（由 s1 代替），r15 的锚点随写法改了（r15b 红）；b32 在 bash 5 上照过、bash 3.2 上红。
- **核对第 1 轮**：A 的 sortu（去掉 `sort -u`）照过（P5S2A-C1-N1）；B 的 31 个：第一轮照过的 16 个都红，对照 b1、b3、b4 照旧红，新写的 12 个里 n1–n3、n6–n10 红，n4、n4b、n5、n11 照过、不报；C 的 c3、c15 红，另以八种探针核过 `state()`。
- **收口**（`9a6b20ae`）：n1（去掉 `sort -u`）、n1b（只 `sort`）、n1c（只 `uniq`）在新的桩上红、旧的桩上照过；n7（`verify` 的 `fail-fast: true`）红在新的消息上；修复第 1 轮的 33 个守卫变异照旧红。
- 每次跑完工作树干净，`.github/workflows/` 里恰是三份，没有备份留在被改的文件旁边。

## 核对

- **核对第 1 轮**（头 `0fe3d663`）：第一轮的都关上（A 5 条、B 7 条、C 11 条）。新的只有 Nit 3 条（P5S2A-C1-N1、B-C1-N1、C-C1-N1）。A 在普通的克隆里重跑 `make image-smoke`，取出 `/licenses` 与第一轮的逐个比，读 run 44484245064 的日志；B 重跑第一轮的 19 个、新写 12 个；C 在 `bash:3.2` 里重跑 31 格，另写八种探针，三个修复提交各跑 `platform/buildinfo`、`make lint`、`make knip`（只跑相关的）。
- 只剩 Nit：照负责人 2026-11-19 的规矩不再开核对，三条在 `9a6b20ae` 改，主持者核对。

## 门禁与 CI

- **逐个提交**（6.2 那一组：`go build ./...`、`go vet ./...`、`make lint`、`make test-go-short`、`make gen-check`、`make knip`，之后核对没有已跟踪的改动；经 `heavy.sh`，在干净的 worktree 里逐个 `checkout --detach`）：实施的 4 个都是 0（`make test-go-short` 48–49 秒），修复第 1 轮的 3 个都是 0（各 50 秒）。审查 C 另跑了实施的 4 个（47–50 秒），核对 C 抽了修复的 3 个（`platform/buildinfo`、`make lint`、`make knip`），都是 0。不改网页，没有逐个跑 `make test-web`。
- **推送之前**（经 `heavy.sh`，每项之前判 `df`）：

  | 头 | 门禁 | 结果 |
  |---|---|---|
  | `e5ecae06` | `make lint gen-check knip build`；`make test-go-short`；`make test-go`、`make test-web`、`make e2e`；`make image`、`make image-smoke`（普通的克隆）与镜像的变异 | 都是 0：22 秒；53 秒（`main` 的 `ab766f02` 47 秒）；191 个 `ok`、3,673 秒，197 个文件、4,801 个测试，582 个通过、1 个照旧跳过；`make image` 12 秒（构建缓存是热的），"image-smoke: nervewiki-p5s2:head 通过（版本 0.0.0-p5s2，提交 e5ecae061b29…，modified=false）"，label 都对 |
  | `0fe3d663` | 同上；另 `bash:3.2` 的容器 | 都是 0：11 秒；49 秒；191 个 `ok`、3,680 秒，197 个文件、4,801 个，582、1；`make image` 15 秒，镜像 71,499,968 字节（arm64），`/licenses` 293 个文件、532,441 字节，与本机直接跑两个脚本的输出 `diff -r` 相同；`bash:3.2` 里 31 格全过 |
  | `9a6b20ae` | 照负责人 2026-11-19 的规矩只跑相关的：`make lint`、`make knip`、`make test-go-short`、`platform/buildinfo`、`tools/licenses/check.mjs`；没改 Dockerfile，不跑 `make image` | 都是 0：5 秒；1 秒；48 秒、94 个 `ok`；1.2 秒；10 格 |
  | `2ea85cd5`（合并 `main`） | `make lint`、`make test-go-short` | 都是 0：9 秒；49 秒、94 个 `ok` |

  修复第 1 轮不改组合根与网页，照修复说明另跑了 `make test-go`、`make test-web`、`make e2e`；收口只改测试的桩与守卫的消息，合并带来的 Go 只有两处注释与 asset 模块的一个测试，都不重跑，CI 上整套跑过。
- **根分区**：开工 24G；实施的两次 `make image` 与 13 个镜像的变异之后 17G，Docker 的构建缓存从约 10G 到 17.6G（共用的，没动），平均每次构建约 0.5 GB；`make test-go` 当中一度 15G；修复第 1 轮开始与结束 18–19G；收口与合并 19G。没有低于 15G。
- **分支的 CI**（`ci.yml` 19 项、`release.yml` 两个演练；每次都是第一次 attempt，没有重跑）：

  | 推送 | 头 | `ci.yml` | `release.yml` | 备注 |
  |---|---|---|---|---|
  | 1 | `785d48be`（6.2 第 2 个之后） | 44451246879：19 项全绿 | 44451246873：两个演练绿（amd64 2:14、arm64 1:19） | 镜像的 SBOM 的文档 `name` 是 `nervewiki`；`anchore/sbom-action` 没被"允许的 actions"挡住 |
  | 2 | `e5ecae06` | 44461178186：19 项全绿 | 44461178210：两个演练绿（1:57、1:19） | 实施的头；tag 的七个作业跳过；Lint workflows"Found 0 errors in 3 files" |
  | 3 | `0fe3d663` | 44484245001：19 项全绿 | 44484245064：两个演练绿（1:40、1:24） | 修复第 1 轮；web 的 Lint 一步里 `check.mjs` 10 格过；`vuln.yml` 44484244964 绿 |
  | 4 | `9a6b20ae` | 44490478831：19 项全绿 | 44490478847：两个演练绿（amd64 1:59、arm64 1:29） | 收口 |

  - 演练 amd64 的 Check the SBOMs（run 44484245064）："nervewiki-0.0.0-rehearsal.56-image.spdx.json：name=nervewiki，56 个包""nervewiki-0.0.0-rehearsal.56-web.spdx.json：name=@nervewiki/web，235 个包""前端 SBOM 里 @nervewiki/* 之外的 233 个包，不在 /licenses/npm/INDEX.tsv（233 行）的只有 @types/：@types/trusted-types@2.0.7"；Licenses 一步 293 个文件、没有文本的 3 个、532,441 字节，附件的包 74,693 字节。两份的最后一行都是"image-smoke: nervewiki:rehearsal 通过（版本 0.0.0-rehearsal.56，提交 0fe3d66397dc…，modified=false）"。
  - 演练加的几步在 amd64 上 15–22 秒（`785d48be` 22 秒，`e5ecae06`、`0fe3d663` 15 秒；推断 1–2 分钟）；arm64 不跑它们。
  - `ci.yml` 的 `image` 一项的 Image smoke 一步（设计 7.5）：`785d48be` 82 秒、`e5ecae06` 81 秒、`0fe3d663` 86 秒；`main` 上 `ab766f02`（还没有许可证的两层）81 秒、`105ff7c8` 59 秒：推断的"多 5–15 秒"与 runner 之间的差别分不开。
  - short 一份的 Test 一步：`785d48be` 2:54、`e5ecae06` 2:52（`main` 的 `ab766f02` 3:16、`105ff7c8` 2:53）；`platform/buildinfo` 本机 1.2–1.4 秒（`ab766f02` 1.16–1.23 秒），`TestTheReleaseScripts` 约 0.1 秒。组合根八份的 Test 一步两次都在 450 秒之下（最长 `e5ecae06` 的 bootstrap-1，378 秒）；S2 不改组合根。
  - `release.yml` 的运行 artifact 都是 0 个；costs-1 的 Lint workflows 读三份、0 错。
- **合并 `main`**（`2ea85cd5`，`305eec44`）：没有冲突，只在本地，与本提交一起推送，分支 CI 是本提交的。
- **本提交的分支 CI**：`5c2b0ff1` 的 `ci.yml` run 44496522481 十九项全绿、`release.yml` run 44496522497 两个演练绿（amd64 2:06、arm64 1:24），第一次 attempt。S2 合并为 `24e7b316`，之后第一次 `main` 的 `ci.yml` run 44500978151 第一次 attempt 19 项全绿（`web` 5:54，Test 5:06；e2e 8:31，E2E 一步 6:09，594、1；image 1:55），`release.yml` run 44500978182 两个演练绿（amd64 1:53、arm64 1:26），tag 的作业都跳过。记号 `14eb2095` 之后的 `main`：`ci.yml` run 44505636150 第一次 attempt 18 项绿，e2e 在装 Playwright 的浏览器时被 30 分钟的时限取消（AWS us-west-2），E2E 一步没有开始，只重跑 e2e，第二次绿（e2e 9:14，E2E 一步 7:48）；`release.yml` run 44505636179 两个演练绿（amd64 1:44、arm64 1:26）。算作记号之后的第二次。
- **没有推任何 tag、镜像，没有建 Release，没有写 GHCR**：tag 上的作业在分支上都跳过；仓库的 tag 与 Release 都是 0 个，`orgs/open-nerve/packages/container/nervewiki` 是 404（实施者与三位审查者各核过）。`docker/login-action`、`actions/attest`、`sigstore/cosign-installer`、`actions/download-artifact` 与写权限只在 tag 的作业里，还没跑过，推 rc 之前照 00 号文档"需要负责人确认"第 2 项核对。

## 环境

实施、审查与核对各在自己的工作树里：实施者 `nerve-wiki-m12-p5-s2`（逐个提交的核对在 `~/.cache/m12-p5-s2/wt-verify`，镜像与它的变异在同一处的普通的克隆里，用完删了）；审查与核对 A `nerve-wiki-m12-p5-rev-a`、B `nerve-wiki-m12-p4s5-rev-c`、C `nerve-wiki-m12-side-rev`。`TMPDIR`、`GOTMPDIR`、日志与假的 `go`、`pnpm`、`gh`、`docker` 在 `~/.cache` 下各自的目录，用完删了。本机没有跑 actionlint、govulncheck 与 syft；`gh` 只读（运行、作业、日志、artifact、tag、Release、包与公开仓库里的源码）；没有推送 tag 与镜像，没有登录 GHCR。镜像都按名字删了（含 `bash:3.2`），smoke 的容器由脚本的 trap 清掉；`bash:3.2` 的容器以 root 写出的三个目录借本机已有的 `postgres:18.7-trixie` 起一个容器删掉，没有拉新镜像；共用的 Docker 构建缓存与 Go 的构建缓存没有动。本机 aarch64、8 vCPU。要留的在设计工作目录的 `m12/p5s2/`（`impl-report.md`、`fix-1-report.md`、`finish-report.md`、`tools/`、`msg/`、`review/`）；实施者的日志与变异表在 `~/.cache/m12-p5-s2/`，合并之后删。
