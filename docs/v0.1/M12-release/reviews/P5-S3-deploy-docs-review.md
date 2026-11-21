# M12/P5 S3（部署文档与发布说明）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m12-p5-s3`，从 `main` 的 `11d1674c`（P4 完成）开始，不合并 `main`。<br>实施 5 个提交，照设计 6.3 的次序，审查时在 `dd0d6804`：`deploy/compose.yaml` 与 `compose-initdb.sh`、两份 ignore、`make deploy-check` 与 CI 的 `image` 一项、守卫读 `compose.yaml` `453e7459`；`deploy/README.md` 与根 README 的"部署"缩成入口、守卫的 README 一项换成 `deploy/README.md` `913d82e1`；`CHANGELOG.md` 的 0.1.0 一节与 `SECURITY.md` `221e90bc`；读屏清单与发布之后的人工验收入口 `a18350cb`；照三遍演练改的 `dd0d6804`。<br>修复第 1 轮 3 个，头 `d1802a0b`：`deploy-check` 的五条断言与停机的宽限 `0e20f1bd`（决定 693、694）；三份审查的文档 `3fef47f3`；同机演练找到的 `d1802a0b`（692）。<br>修复第 2 轮 2 个，头 `22d6a822`：停机的宽限 60 秒与健康检查的断言认参数 `1f37cc2e`（694）；文档 `22d6a822`。<br>分支共 10 个提交（不计本提交），相对 `11d1674c` 新加 `deploy/compose.yaml`、`deploy/compose-initdb.sh`（可执行）、`deploy/README.md`、`CHANGELOG.md`、`SECURITY.md`、`docs/v0.1/M12-release/manual/` 的 `README.md` 与 `screen-reader-checklist.md`，改 `.gitignore`、`.dockerignore`、`Makefile`、`.github/workflows/ci.yml`、`README.md`、`deploy/runtime-grants.sql` 的注释、`server/configs/config.yaml` 的注释、`pgtest/image_test.go`、`bootstrap/runtime_role_test.go` 的注释；不改服务端的代码，不加依赖，契约、迁移与网页不动，`docs/` 里 `manual/` 两份之外的设计、记录与移交在本提交 |
| 规格 | [P5 设计](../05-P5-deploy-release.md)第 0 节 S3 一行，3.16–3.20，3.22，3.24，3.25，6.3，6.4，7.1–7.3，第 8、9 节；[00 号文档](../00-M12-design.md) 4.8、4.12、4.13，第 3 节第 7、10 条，第 7 节"移交的落实""总体设计的修订"第 7、9 条；负责人 2026-11-17 第 8–10 问、2026-11-21 第 13 问 |
| 审查方式 | 三位审查者（Opus）并行，只读，不碰实施者的工作树；重的经 `heavy.sh`，根分区 19–20G：<br>A compose、演练与检查：在分离的 worktree 里读 `deploy/compose.yaml`、`compose-initdb.sh`、`deploy/README.md` 第 2、4、5 节、`deploy-check`、守卫与两份 ignore，读三遍演练的日志与脚本；自己经一次 `heavy.sh` 做了四组小实验（官方镜像的临时服务器只听套接字；`compose.yaml` 副本上的 8 个变异；新卷上每约 0.35 秒看一次初始化；文档里几条 shell 命令的语义）。<br>B 部署文档的内容对代码与配置：`git show` 读代码、配置与文档，数对 P4 10.3、10.4、S7 设计第 9 节与 10.6–10.9、负责人第 13 问；WebFetch 读 PostgreSQL 18.7 的发布说明。<br>C 发布说明、清单、对照表与草稿：`CHANGELOG.md`、`SECURITY.md`、`manual/` 两份、实施报告的两张对照表与文档草稿；对着分支的 `CHANGELOG.md` 跑 `release-notes.sh`；`gh run view` 读 CI。<br>核对第 1 轮三位各核自己的发现与三个新提交（A 另经一次 `heavy.sh` 跑 `deploy-check` 与两个变异）；核对第 2 轮 A 核 A2-M1，C 核 C2-M1、C2-M2，Nit 由主持者核 |
| 日期 | 2026-11-21（实施、审查、修复第 1 轮、核对第 1 轮）；2026-11-22（修复第 2 轮、核对第 2 轮、本提交） |
| 结论 | Important 3：同机演练照写的做会在正式的目录里改名 `data/`（P5S3A-I1）；"导入上限的"vault 指的是今天一次导不进来的那一份（P5S3B-I1）；"刚删了马上恢复还要慢一些"与出处相反（P5S3B-I2）。Minor 16：A 2、B 7、C 7；Nit 23：A 4、B 8、C 11；Q 2（B、C 各 1）。实施者另记五个 Nit。修复第 1 轮 3 个提交；核对第 1 轮三位的原发现全关，新的 A 1M 2N、B 2N、C 2M 4N；修复第 2 轮 2 个提交；核对第 2 轮 A2-M1、C2-M1、C2-M2 关上，其余 Nit 主持者核。三位都确认：库角色不是超级用户、表与 `pg_trgm` 归它；秘密经 `secrets` 与 600 的 `.env` 进容器；数与 P4 10.3、负责人第 13 问逐行对得上；`CHANGELOG.md` 的一节进 Release 的说明没有相对链接；分支 CI 的数逐项对上。没有推任何 tag、镜像，没有建 Release，没有写 GHCR |

## 审查的发现与处置

编号照审查者的报告：P5S3 + 审查者（A、B、C）+ 级别与序号；核对第 1 轮的新发现写作 `A2-`、`B2-`、`C2-` 加级别与序号。审查时的行号是 `dd0d6804` 的，核对第 1 轮是 `d1802a0b` 的。修复照主持者的说明（`p5s3/fix-1.md`、`fix-2.md`），在前一个头之上加提交，普通推送、不改写已推的历史，每个单独为绿。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P5S3A-I1 | 重要（演练） | "演练"一节：第 1 步不拷备份；`export COMPOSE_PROJECT_NAME` 之后"照原样用"，而恢复块的第一行是 `cd /srv/nervewiki`：照贴，`mv data data.old` 改名的是正式的附件目录，之后的备份打的是旧副本，`data.old` 一删就丢；没有 `unset` | 决定 692、704，`3fef47f3`：同机一段写成四步三块（`p`、`d`，备份拷进演练的 `backups/`，端口 18080、子网 `10.213.48.0/24`，不先停、不挪开，最后 `unset`），恢复块去掉 `cd`。照它在同一台机器上、正式的一套起着时真走了一遍（下面"演练"第 4 遍），找到两处，`d1802a0b`。核对 A：关上 |
| P5S3A-M1 | 中（文档） | `/srv/nervewiki` 普通用户建不了，文档其余步骤又假定部署的用户拥有它；`mv` 不带 sudo 只在拥有父目录时成立 | 决定 701，`3fef47f3`：`sudo install -d -o "$(id -u)" -g "$(id -g)" -m 755`，开头写明以拥有它、在 `docker` 组里的用户执行。核对 A：关上 |
| P5S3A-M2 | 中（检查） | `deploy-check` 只查写法：去掉 `profiles`、健康检查去掉 `-h`、只改子网、`ports` 去掉 `127.0.0.1:`、宽限改小都照过，前两条演练也照过；注释说"升级的次序由演练核对"，演练从不走到 | 决定 693，`0e20f1bd`：`jq` 在 `config --format json` 上核对五条，每条一个变异（a1–a5）、各报自己的那一句；注释照实改。核对 A：关上（另做的只改 `TRUSTED_PROXIES` 红在 a2 那一句） |
| P5S3A-N1 | 细节（注释） | 健康检查的 `-h 127.0.0.1` 是 `--wait` 成立的前提，没有注释 | `0e20f1bd`：注释写明临时服务器只听套接字、`--wait` 靠它、`deploy-check` 核对。核对 A：关上 |
| P5S3A-N2 | 细节（文档） | 改密码只写了原则，没写怎样生效 | `3fef47f3`：`exec db psql -U postgres -c '\password nervewiki'`、改 `.env`、`docker compose up -d`；`postgres` 的密码只管 TCP。第 4 遍在正式的一套上走过（同一条命令的非交互写法）。核对 A：关上 |
| P5S3A-N3 | 细节（文档） | `mkdir -p -m 700` 对已有的目录不改权限；`backups/` 里 root 的 tar 用户删得掉，要 root 的只有 `data/` | 决定 700，`3fef47f3`：`mkdir -p backups && chmod 700 backups`；第 4 遍先以 775 建好、块之后 700。核对 A：关上 |
| P5S3A-N4 | 细节（演练） | `up -d serve` 之后紧跟的 `curl …/readyz`，演练先等就绪、不记进日志 | `3fef47f3`、`d1802a0b`：安装、恢复、升级、演练各写"不是 200 就过几秒再看"；第 4 遍记下：正式的立即看是 000，演练第 1 次连接被重置、1 秒后 200。核对 A：关上 |
| P5S3B-I1 | 重要（数） | "导入上限的 vault"的数出自 P4/S3 那一份（45,000 页、9,910 个文件夹、5,000 个附件，59,911 个节点），S6 之后一次导不进来；读者会照它留 WAL 与时间、导的时候整包失败 | `3fef47f3`：写出那一份的形状与"今天要分两次导"；上限处按节点数推算（WAL 约 41 GiB，导入约 3 分钟、7763 上约 7 分钟），标"推算"；"导入的上限"一条的 45,000 不动。核对 B：关上（复算过） |
| P5S3B-I2 | 重要（数） | "刚删了马上恢复、自动 vacuum 还没跑的库上，这两种操作还要慢一些"与出处相反：13.79、14.54 秒量的正是这种状态 | `3fef47f3`：改为"这两个数量的正是刚删了马上恢复、自动 vacuum 还没跑、没有统计的状态"。核对 B：关上 |
| P5S3B-M1 | 中（数） | 默认配置没写容器多大，"按约 3 GiB 留"与 `GOMEMLIMIT` 的"不低于 3 GiB"合不上 | `3fef47f3`：按约 3 GiB 留给服务，容器约 3.5 GiB（`GOMEMLIMIT=3072MiB`）。核对 B：关上 |
| P5S3B-M2 | 中（数） | "每次删除多约 0.7–0.9 秒"是本机的增量，放在"最低配置上"之下 | `3fef47f3`：每次多约 1.3–1.9 秒（本机约 0.65–0.94 秒）。核对 B：关上 |
| P5S3B-M3 | 中（与代码不符） | `reindex` 的代价写成无条件的：它重建链接索引，搜索的行只在缺了或分词的版本变了时排队，由服务起来之后的后台任务建；26 分钟、290 GB 只在分词的版本变了时 | `3fef47f3`：照代码重写，进度的查询 `page_search_queue WHERE bulk`；"容量"的两行加条件。核对 B：关上 |
| P5S3B-M4 | 中（与代码不符） | "配置不对时服务记 WARN"只在 `trusted_proxies` 为空时成立；设了却不含代理的地址时什么也不记 | `3fef47f3`：分开两种情形（WARN 每个进程一次），给核对的查询 `SELECT DISTINCT host(ip) FROM auth_sessions`。核对 B：关上 |
| P5S3B-M5 | 中（说法） | "验证过的写法"也盖住了没在 nginx 上跑过的 `gzip` 几行 | 决定 702，`3fef47f3`：写明验证过的是哪几行，`gzip` 几行没跑过、事件流不在 `gzip_types` 里。核对 B：关上 |
| P5S3B-M6 | 中（文档） | "不用 compose"的建库语句没有 `OWNER`：PostgreSQL 15 起不是超级用户的表的所有者建不了表与 `pg_trgm` | `3fef47f3`：`CREATE DATABASE nervewiki OWNER <表的所有者> …` 并写理由。核对 B：关上 |
| P5S3B-M7 | 中（文档） | PostgreSQL 大版本：用 compose 的写法没写，只换 tag 会在同一个卷里另起空集群；dump/restore 之后的 `ANALYZE` 没提 | `3fef47f3`：照写，`pg_upgrade` 官方镜像里没有（没有走，照审查与镜像的写法）。核对 B：关上 |
| P5S3B-N1 | 细节（数） | 根 README `:141`、`config.yaml:95` 把界 2,662 MiB 写成"实测" | `0e20f1bd`、`3fef47f3`：最坏约 2.6 GiB 堆（2,662 MiB；最坏实测 2,592 MiB）。核对 B：关上 |
| P5S3B-N2 | 细节（数） | PostgreSQL 自己的进程"0.1–0.3 GiB"下限没有出处 | `3fef47f3`：空闲几十 MiB，至多约 0.3 GiB。核对 B：关上 |
| P5S3B-N3 | 细节（文档） | `MAINTAIN` 的表不止"11 张与 `page_search`" | `3fef47f3`：不写数，加升级的 `upgrade_nodes`。核对 B：关上 |
| P5S3B-N4 | 细节（数） | 旧 README 里 M8 的 `max_wal_size` 16 GB 只少约 3% 与看建索引进度的查询没挪 | `3fef47f3`：挪进"容量"的 WAL 一行与"升级与回退"。核对 B：关上 |
| P5S3B-N5 | 细节（文档） | "第一个管理员"：实例没有管理员账户 | `3fef47f3`："第一个账户"，关了创建工作区的用 `workspaces create --admin`。核对 B：关上 |
| P5S3B-N6 | 细节（文档） | "停顿保存的一段编辑"容易读成每停一下一个版本 | `3fef47f3`：打开编辑器到关上，其间的保存都在一个版本里。核对 B：关上 |
| P5S3B-N7 | 细节（注释） | `compose.yaml:50` 指错了节 | `0e20f1bd`：指"为什么设 trusted_proxies 与钉子网"。核对 B：关上 |
| P5S3B-N8 | 细节（文档） | 附件可压缩不必写"推断"（生成器的性质）；小版本换 tag 时服务会断一下 | `3fef47f3`：照改，小版本先停 `serve`。核对 B：关上 |
| P5S3B-Q1 | 问 | 停机的"至多 38 秒"没算 lint 的收尾（至多 7 秒），最坏 45 秒，过 40 秒的宽限 | 主持者定（决定 694），`0e20f1bd`：算进去，45 秒、宽限 50 秒；修复第 2 轮照 A2-M1 改为 52 秒、60 秒。核对 B：关上 |
| P5S3C-M1 | 中（已知限制） | P3 交给发布说明的 GB18030 一条漏了，同一格的"其余"两条（U+2800、事件流的上限）没写也没写理由 | `3fef47f3`："导入的名字的编码"加 GB18030，"两条边界"写另两条；对照表加 P3 第 9 节两行。去处见 C2-M1。核对 C：关上 |
| P5S3C-M2 | 中（已知限制） | P2 4.4.8（S4 之后仍在的限制）没进 `CHANGELOG.md`，也没进对照表 | `3fef47f3`："阅读视图的就地更新"；对照表一行，去处 `M12-polish.md`。核对 C：关上 |
| P5S3C-M3 | 中（已知限制） | 决定 514 的浏览器一侧不在 `CHANGELOG.md`；deploy/README 写"会很慢"没量过 | `3fef47f3`、`22d6a822`：两处都写"可能仍慢（推断）"与改进之前量到的 19 秒与崩溃。核对 C：关上 |
| P5S3C-M4 | 中（已知限制） | S7 第 9 节第 1 条"有编辑历史的大子树第一次删除就一样慢"被写窄成"反复删除" | `3fef47f3`：子树里的历史越多越慢，页编辑过多次的大子树第一次删除就可能过（推断）。核对 C：关上 |
| P5S3C-M5 | 中（已知限制） | `CHANGELOG.md` 与 deploy/README 第 8 节互不包含，各自暗示完整；H4 的变通与"可见"没进发布说明；移动、改名改写页的 13 / 16 秒不在 `CHANGELOG.md` | 决定 705，`3fef47f3`：互相指，另起"不能立即永久删除"，13 / 16 秒并进"最低配置上离 15 秒的期限近"。核对 C：关上 |
| P5S3C-M6 | 中（清单） | 读屏清单 B 第 2 项（8.4）的场景抄错：8.4 是带锚点打开时缓存的视图再读一次 | `3fef47f3`：照 M6/P3 改写，写明接受的行为与要记的。核对 C：关上 |
| P5S3C-M7 | 中（草稿） | 第 15 节的草稿写成表行、内容是交付物 | 本提交照相邻各行写列表项。核对 C：关上 |
| P5S3C-N1–N5 | 细节（发布说明） | `:22`、`:26` 的空格；内存另起一条；任务记录少了后台的恢复与撤销；"本 Release 的说明"；大图写的是缺的改法 | `3fef47f3` 照改。核对 C：关上 |
| P5S3C-N6 | 细节（外观） | Release 页上 CHANGELOG 一节的 `###` 与脚本接的 `##` 层级倒过来 | 决定 706：不改，记一句。核对 C：关上 |
| P5S3C-N7–N9 | 细节（清单） | 输入法清单是 23 步；负责人第 4 问是 11-13；MCP 清单 I 一节另有 Caddy 样例 | `3fef47f3` 照改；清单 I 一节本身在本提交。核对 C：关上 |
| P5S3C-N10、N11 | 细节（草稿） | 00-M12"文档与代码不符之处"末两条的落实没有草稿；3.26 第 9 条的出处写错 | 本提交照写。核对 C：关上 |
| P5S3C-Q1 | 问 | P3 设计要发布说明把决定 376 的那一条写成"相对今天的回归"，v0.1.0 是第一个版本 | 主持者定不写（决定 695）。核对 C：关上 |
| P5S3A2-M1（与 P5S3B2-N1 同一件） | 中（停机；核对第 1 轮） | lint 的工作者停两次：`run` 的 `defer a.stopLint` 一次，`close` 又一次，各自新开 7 秒；最坏 52 秒，过 50 秒的宽限；"一起调长"的例子 82 秒过 80 | 主持者定改式子、不改代码（决定 694），`1f37cc2e`、`22d6a822`：52 秒、宽限 60 秒、例子 90 秒，断言与它的变异随之；`close` 不再等第二次进 v0.2 的 `M12-release.md`。核对 A：关上 |
| P5S3A2-N1 | 细节（演练；核对第 1 轮） | 上次留下的演练目录让 `mkdir` 失败、`cd` 不做，第 4 步相对路径的 `rm -rf data` 删别处；另一台机器一段没说放进 `backups/` | `22d6a822`：先删留下的目录，`data/` 写绝对路径，备份放进 `backups/`（`chmod 700`）。主持者核对 |
| P5S3A2-N2 | 细节（检查；核对第 1 轮） | 健康检查的断言按字面判，`-h` 放到后面的合法写法误红 | 决定 693，`1f37cc2e`：认 `pg_isready` 的参数里有 `-h 127.0.0.1`，不论次序；换了次序的变异照过，换主机、不在开头的红。主持者核对 |
| P5S3B2-N2 | 细节（文档；核对第 1 轮） | "不设时只说没有地址"漏了图片 | `22d6a822`：没有下载地址，图片照样直接给出，别的附件只给元数据。主持者核对 |
| P5S3C2-M1 | 中（去处；核对第 1 轮） | "两条边界"进了已知限制，对照表却写"不移交"，与第 3 节第 10 条不合 | 主持者定给去处（决定 696）：U+2800 进 `M12-polish.md`，事件流的两个上限进 `M12-release.md`，收尾时写；对照表照写。核对 C：关上 |
| P5S3C2-M2 | 中（草稿；核对第 1 轮） | 草稿 E 的 M4-performance 一行仍写修复轮已改正的两个数 | 本提交照 C 的写法（约 6 万个节点的 vault 约 55 GiB，上限处推算约 41 GiB；分词的版本变了时 `reindex` 约 290 GB）。核对 C：关上 |
| P5S3C2-N1–N4 | 细节（核对第 1 轮） | `CHANGELOG.md` 的"打开很慢"没标推断；报告第 10 节与草稿两处不一致；3.26 第 8 条；草稿 F 的锚点与 G 的提交 | `22d6a822` 与草稿照改（报告第 10、11 节换成草稿的全文，以草稿为准）。主持者核对 |
| 实施者自记 1–5 | 细节（推送之后自己重读） | "配置"的"至少要有"多列了两项；"不用 compose"的 `@db:5432`、`runtime-grants.sql` 从哪里来；`CHANGELOG.md:26` 的空格；`:51` 的版本的清除 | `3fef47f3` 照改（必须的只有数据库地址与私钥；`<数据库的主机>`、默认网桥的网关（推断）；raw 的地址；活着的页的版本一直留着）。核对 B、C：关上 |

## 主持者的决定

负责人可以改判；写进 [00 号文档](../00-M12-design.md)第 11 节"2026-11-22：P5/S3 的决定 691–706"，各条的理由与出处在那里，与设计不同之处在 [P5 设计](../05-P5-deploy-release.md) 3.26。691–696 是主持者在说明里定的（`brief.md`、`fix-1.md`、`fix-2.md`），697–706 是实施里定、主持者在文档提交时认的。

- **691** `/dev/shm` 只在 deploy/README 写一句，`compose.yaml` 不改默认。
- **692** 同机演练重写，并在同一台机器上真走一遍（A-I1）。
- **693** `make deploy-check` 以 `jq` 核对五条，写在 `Makefile`（A-M2、A2-N2）。
- **694** 停机至多 52 秒、宽限 60 秒、例子 90 秒，不改代码；`close` 不再等第二次进 v0.2（B-Q1、A2-M1、B2-N1）。
- **695** 发布说明不写"回归"（C-Q1）。
- **696** "两条边界"的去处：U+2800 进 `M12-polish.md`，事件流的两个上限进 `M12-release.md`（C2-M1）。
- **697** `CHANGELOG.md` 的一节不写相对链接。
- **698** 安装与恢复写 `up -d --wait db`。
- **699** 升级块写出备份的两行。
- **700** `backups/` `chmod 700`，`.env` 先 600 再写（A-N3）。
- **701** 部署的目录建给部署的用户，`mv data data.old` 不要 sudo（A-M1）。
- **702** 两份代理样例加 JSON 的压缩，nginx 的几行写明没跑过（B-M5）。
- **703** 人工验收的入口另收 P2/S5 的三项。
- **704** 同机演练的变量、端口 18080 与子网 `10.213.48.0/24`。
- **705** `CHANGELOG.md` 另起几条，与 deploy/README 第 8 节互相指（C-M1–M5）。
- **706** Release 页上的标题层级不改（C-N6）。

## 接受与推后的

- **推后到收尾**：v0.2 的 `M12-release.md` 写 `close` 在 `run` 已经停过 lint 时不再等（694）与事件流的两个上限，`M12-polish.md` 写 U+2800（696）；`CHANGELOG.md` 的日期（4.11 第 6 步）；P2 已完成，"P5/S3 之后才合并的 P2 的已知限制"今天没有，收尾时核一遍。
- **只有 rc 走得到的**：deploy/README 的"拉镜像并校验""用 GHCR 的镜像装"与 `curl` 取 `raw.githubusercontent.com/…/v0.1.0/deploy/…` 的两个地址（rc 时用 `v0.1.0-rc.1` 的地址试一次）；包设为公开之后用 GHCR 的 rc 再装一遍（设计 3.20 末条、4.11 第 5 步）。
- **接受，不改**：
  - 守卫对 `deploy/README.md` 只认带反引号的 `` `postgres:18.7-trixie` ``；借镜像当 root 的四处 `docker run … postgres:18.7-trixie` 不在守卫里，升 PostgreSQL 时漏改只是多拉一个镜像。
  - 两份代理样例的压缩几行没在真的 nginx、Caddy 上跑过；MCP 客户端清单 I 一节照样例起 nginx 时第一次跑到（本提交在清单里写了要看的）。
  - 演练没有走"不用 compose"的两个角色的写法：`runtime-grants.sql` 由 `runtime_role_test.go` 常驻核对，命令是从根 README 原样挪的。
  - `deploy/image-smoke.sh:223` 的 `docker stop -t 40` 不改：烟测里没有请求、任务与 lint 的工作者，停机在几秒内。

## 变异

每组先在没改的树上跑同样的命令（都 rc=0），从 `.mutbak` 复原，跑完没有留下 `.mutbak`。`make deploy-check` 一类不是 Go 的测试，`mut.py` 记作 `FAILED(unnamed)`；修复轮起由一个帮手判：`make -s deploy-check` 失败**且**输出里有那一条的文字才算抓到，所以每个都写"报的"。

| # | 变异 | 结果，报的 |
|---|---|---|
| d1、d1b | `migrate`、`serve` 的 `${NERVEWIKI_VERSION:?…}` 写错名（设计 7.2） | 红：`required variable NERVEWIKI_VERSIO is missing a value` |
| d2 | `migrate` 的 `secrets` 写成 `[jwtx]`（设计 7.2） | 红：`service "migrate" refers to undefined secret jwtx` |
| d2′ | 同 d2，不带 `--profile migrate` | 照过（设计写明：那个参数是必要的） |
| d3 | `serve` 的 `secrets` 写成 `[jwtx]` | 红 |
| d4 | `depends_on` 写成 `dbx` | 红：`depends on undefined service "dbx"` |
| d5 | `stop_grace_periodx`（schema） | 红：`false schema` |
| d6、d7 | `${NWIKI_PUBLIC_URL:?`、`${NWIKI_DB_ADMIN_PASSWORD:?` 写错名 | 红 |
| d8 | `networks.default` 下没闭合的 `[` | 红：`go-yaml load error` |
| p1、p1b | `compose.yaml` 的镜像改回 18.6、initdb 的 `builtin` 改成 `icu`（设计 7.2） | `TestContainerMatchesTheOtherDatabases` 红 |
| p2、p2b | `deploy/README.md` 的 `` `postgres:18.7-trixie` `` 改成 18.6、去掉反引号（设计 7.2） | 同上红 |
| a1 | 去掉 `profiles: [migrate]`（修复第 1 轮） | 红：`服务恰是 db、serve，migrate 只在 profile migrate 里` |
| a2 | 只改 `subnet` | 红：`serve 的 NWIKI_SERVER__TRUSTED_PROXIES 等于 networks 的子网` |
| a3 | 健康检查去掉 `-h 127.0.0.1` | 红：`db 的健康检查经 TCP：pg_isready -h 127.0.0.1` |
| a4 | `ports` 去掉 `127.0.0.1:` | 红：`serve 的端口只听 127.0.0.1` |
| a5 | 宽限改小（第 1 轮 40s，第 2 轮 50s） | 红：`serve 的 stop_grace_period 是 60 秒（compose 展开成 1m0s）` |
| x | a5 的改动、找 a4 的文字（对照） | 照过（如期：判的是那一条） |
| a3b、a3c | `-h 127.0.0.10`；命令前加 `true \|\| `（修复第 2 轮） | 红，报健康检查那一条 |
| a3d | 参数换次序 `-U … -d … -h 127.0.0.1` | 照过（如期，A2-N2） |
| a5b | `stop_grace_period: 1m` | 照过（如期：同一个时长） |

第一次跑修复第 1 轮的变异时没加 `-s`，make 回显的命令里本来就有五条的文字、判不出是哪一条；加了 `-s` 重跑的才作数。没有为 `.dockerignore` 做变异：只在它里少一项不会让 image-smoke 红（构建里的 `.gitignore` 照样忽略，`vcs.modified` 不看被忽略的文件），只把那个文件带进构建上下文。

## 演练

都经 `heavy.sh`，每遍一个脚本，在 `~/.cache/m12-p5-s3/` 下自己的目录里照 `deploy/README.md` 的命令原样执行；脚本、日志与命令的全文在设计工作目录 `m12/p5s3/drills/`。秘密：两个数据库密码只在 `.env` 里（日志只打键名与权限），账户的密码与访问令牌只在变量里（日志写变量名、令牌只记长度）；日志里 32 位以上的十六进制只有提交号与镜像 id（核过）。与文档的出入：没有 sudo，`chown`、`tar`、删属主 65532 的目录用文档写的借镜像当 root 的写法；`compose.yaml` 与 `compose-initdb.sh` 从提交复制（文档 `curl` 取 `v0.1.0` 的，tag 还没有）；用本机构建的镜像，不拉、不校验（rc 才走得到）；账户的密码生成，不打出来。

**镜像**：普通的克隆里 `make image VERSION=0.1.0-drill.N IMAGE=ghcr.io/open-nerve/nervewiki:0.1.0-drill.N`：新的是 S3 的第 4 个提交 `a18350cb`（`drill.1`），旧的是 `5e8af5b5`（`drill.0`；`git log --diff-filter=A 5e8af5b5..HEAD -- server/migrations/sql/` 为空，M12 没有新的迁移，P5-17）；第 4 遍从 `3fef47f3` 建 `drill.6`。用完都按名删了。

**第 1 遍：从零装**（项目 `nervewiki-drill-1`，7 s）：

```bash
cp …/deploy/compose.yaml …/deploy/compose-initdb.sh . && chmod 755 compose-initdb.sh
touch .env && chmod 600 .env && cat > .env <<EOF …（四个键与 NERVEWIKI_PORT；密码 openssl rand -hex 24）
mkdir -m 700 secrets && openssl genpkey -algorithm ed25519 -out secrets/jwt.pem && chmod 644 secrets/jwt.pem
mkdir data && docker run --rm -v "$PWD":/w --entrypoint sh postgres:18.7-trixie -c 'chown 65532:65532 /w/data'
docker compose up -d --wait db && docker compose run --rm migrate && docker compose up -d serve
printf '%s\n' "$PASSWORD" | docker compose run --rm -T serve users create --email admin@example.com
curl -fsS http://127.0.0.1:18081/readyz; curl -fsS http://127.0.0.1:18081/api/v0/instance
```

`.env` 600、`secrets` 700、`jwt.pem` 644、`data` 65532:65532；迁移 00001–00059 以非超级用户执行；`nervewiki` 的 `rolsuper|rolcreatedb|rolcreaterole` 是 `f|f|f`，`public` 每张表与 `pg_trgm` 的所有者是 `nervewiki`，库是 `b|C.UTF-8|UTF8|nervewiki`；版本 `0.1.0-drill.1`、提交 `a18350cb`。经 `127.0.0.1:18081` 带 `X-Forwarded-For: 203.0.113.9` 登录，服务的日志里 `not a trusted proxy` 0 行，`auth_sessions` 的 IP 只有 `203.0.113.9`；写一页、一个 300,000 字节的附件，读回的 sha256 相同。**对照**：同样装好之后去掉 `NWIKI_SERVER__TRUSTED_PROXIES` 一行，WARN 1 行，会话的 IP 是网关 `10.213.47.1`。

**第 2 遍：升一次级**（`nervewiki-drill-2`，10–11 s）：装 `drill.0`（提交 `5e8af5b5`）、写入；`docker compose stop serve`，备份（`mkdir -p -m 700 backups`、`pg_dump -U postgres -Fc`、借镜像 `tar` 出 `blobs/`），`sed -i` 改版本号，`docker compose run --rm migrate` 答 `no pending migrations`，`up -d serve`；版本与提交是新的，库角色与所有者照旧，正文与附件的字节相同。

**第 3 遍：备份再恢复**（`nervewiki-drill-3`，15–16 s）：备份；恢复：`stop serve`、`mv data data.old`（不要 root）、`down -v`（卷剩 0）、`up -d --wait db`、`pg_restore -U nervewiki -d nervewiki --exit-on-error`（没有错）、借镜像建 `data`、解开 `blobs`、`chown -R`（`data` 755、`blobs` 750，都是 65532）、`psql -U postgres -d nervewiki -c ANALYZE`、`up -d serve`；所有者照旧，33 张表有 `last_analyze`，正文与附件的字节（以恢复之前签出的地址读）相同。

**第一次跑三遍时文档做不下去或做错的**，`dd0d6804` 改了，改后三遍照最后的文字再各跑一遍（7、11、16 s）：升级里"照'备份与恢复'备份"会连那一块最后的 `up -d serve` 一起做；`pg_dump` 的输出是 664；`.env` 先写后 `chmod`，写的那一刻是 664；恢复不等就连。探针（新卷上 `up -d db` 之后立即以 `nervewiki` 连）：第 1 次没有套接字，第 2 次 `FATAL: role "nervewiki" does not exist`（只听套接字的临时服务器，初始化脚本还没跑），第 3 次才连上。脚本自己的两次失败（克隆停在旧提交；写入之前没登录）不是文档的问题，都清理干净。

**第 4 遍：同一台机器上的演练**（修复第 1 轮，审查 A 的 I1；94 s，其中浏览器约 70 s）：

- 正式的一套代替 `/srv/nervewiki`，在 `~/.cache/m12-p5-s3/srv/nervewiki`（`install -d …` 那一条去掉 sudo 照样执行），项目名不设、就是 `nervewiki`，端口 8080、子网 `10.213.47.0/24`；照"安装"装好、建账户；"为什么设 `trusted_proxies`"新写的核对命令答 `203.0.113.9`；写一页、一个附件；照"compose 的备份"那一块（从 README 读出来执行；`backups/` 先以 775 建好，块之后 700）；备份之后再写一页、一个附件。
- 演练：README"演练"里同一台机器那一段的三块代码由脚本从文件读出，在**一个新的 shell** 里逐行执行；只替换 `/srv/nervewiki`、`~/nervewiki-drill`、`cd &&`（不写真的家目录）、`d=<备份的日期>`，两行 sudo 换成 README 的借镜像的写法：

  ```bash
  p=$HOME/.cache/m12-p5-s3/srv/nervewiki; d=20261121-231716
  mkdir …/home/nervewiki-drill && cd …/home/nervewiki-drill
  export COMPOSE_PROJECT_NAME=nervewiki-drill
  cp $p/compose.yaml $p/compose-initdb.sh .; install -m 600 $p/.env .env; echo NERVEWIKI_PORT=18080 >> .env
  sed -i 's#10\.213\.47\.0/24#10.213.48.0/24#g' compose.yaml
  mkdir -m 700 secrets backups; install -m 644 $p/secrets/jwt.pem secrets/
  cp $p/backups/nervewiki-$d.dump $p/backups/blobs-$d.tar.gz backups/
  docker compose up -d --wait db
  docker compose exec -T db pg_restore -U nervewiki -d nervewiki --exit-on-error < backups/nervewiki-$d.dump
  docker run --rm -v "$PWD":/w --entrypoint sh postgres:18.7-trixie -c "mkdir /w/data && tar -C /w/data -xzf /w/backups/blobs-$d.tar.gz && chown -R 65532:65532 /w/data"
  docker compose exec -T db psql -U postgres -d nervewiki -c ANALYZE
  docker compose up -d serve
  curl -fsS http://127.0.0.1:18080/readyz; curl -fsS http://127.0.0.1:18080/api/v0/instance
  docker compose down -v
  docker run --rm -v "$PWD":/w --entrypoint sh postgres:18.7-trixie -c 'rm -rf /w/data'
  cd …/home && rm -rf …/home/nervewiki-drill
  unset COMPOSE_PROJECT_NAME
  ```

- 结果：`up -d --wait db` 等到健康；`readyz` 第 1 次 `Connection reset by peer`、1 秒后 200；18080 上以备份时的密码登录，版本是 `.env` 里的，第一页的正文与第一个附件的字节相同，**备份之后写的那一页与附件是 404**，同时正式的 8080 上那一页 200；浏览器（Playwright 的 Chromium）打开 `http://127.0.0.1:18080`、登录，页面树只有备份之前的那一页，附件的链接取回的字节相同。之后那个 shell 里 `COMPOSE_PROJECT_NAME` 已 unset；`nervewiki-drill` 的容器、卷、网络 0，目录没了；正式的两个容器的 id 与启动时间、`data/` 的 inode 都不变，没有 `data.old`，备份之后写的那一页与附件仍在。
- 同一遍在正式的一套上走了改密码（`\password` 的同一条命令加 `-T`、两次从标准输入给）：`sed -i` 之后 `.env` 仍 600，`docker compose up -d` 重建了 `db` 与 `serve`，起来的只有这两个（`migrate` 留在 profile 里），以新密码连上、读回的正文相同。
- 找到的两处（`d1802a0b`）：没有 sudo 时演练指"同上文"，上文删的是 `data.old`，照做要自己改，演练要的两行写在第 2 步之后；`readyz` 紧接着不是 200，那一行加注释。脚本前两次停在它自己从 README 取代码块的断言上（折行的借镜像写法；替换进去的注释里有"sudo"），没走到演练，清理干净。

## 数从哪里来（设计 3.18 的十三行）

出处都是 [P4 设计](../04-P4-performance.md) 10.3"给部署文档"的同名一行，另注的除外；修复第 1 轮改过的标出。

| 3.18 的一行 | 取的数 | 出处 | deploy/README |
|---|---|---|---|
| 默认配置的内存 | 服务空闲 40–54 MiB；20–50 个编辑者 117–136 MiB（"约 120–140"）；B2 堆最坏实测 2,592 MiB，写"约 2.6 GiB（2,662 MiB）"；`VmHWM` 至多 2,718 MiB；按约 3 GiB 留给服务、容器约 3.5 GiB（修复第 1 轮，B-M1）；PostgreSQL 空闲几十 MiB、至多约 0.3 GiB（B-N2）、`memory.peak` 4–8 GB 多半是页缓存 | 10.3 第 1 行；负责人第 13 问第 4 条与其注 3（决定 643、651） | 1"硬件"与"最坏的内存" |
| `GOMEMLIMIT` | 容器上限减约 0.5 GiB、不低于 3 GiB；B2 加 B3 堆 3.3–3.5、进程 3.5–3.8、容器 4–4.5 GiB；再加 8 个标签页 5.5 GiB 起 | 10.3 第 2 行（推算） | 1 |
| 小机器 | 预算 5 MiB、`GOMEMLIMIT=1536MiB`：堆 1,492 MiB、`VmHWM` 1,676 MiB、留 1.7 GiB、容器 2 GiB；PostgreSQL 3.25 GB | 10.3 第 3 行 | 1"硬件" |
| 最低 CPU 的余量 | 13.79、14.54、4.82 秒，余 0.46（"约 0.5"），慢 25% 16.47、16.88；量的正是刚删了马上恢复的状态（修复第 1 轮，B-I2）；B4 在 S7 之后 9.96、6.82、5.96（× 1.25 12.49、8.52、7.45）；B7 6.31、7.33；`batch` 8.2–8.7；比 2.046、2.257 | 10.3 第 4 行；S7 设计 10.6；负责人第 13 问第 3 条 | 1"最低配置的余量"、8 |
| 磁盘：每 1,000 页、`page_search` | 34.7–35.6 MB、每 1,000 个节点约 26 MB；P4 量的那一份 vault（约 6 万个节点，今天要分两次导）导入之后 1.57–1.61 GB、删除再恢复 2.37 GB；3.32–4.57 倍，按 5 倍留 | 10.3 第 5 行；M8 的数 | 7 |
| 附件与导出 | 附件 4.10 GB、zip 0.47 GB（合成的附件每个只有十二分之一是随机的字节，B-N8） | 10.3 第 6 行；`server/tools/perf/vault/vault.go` | 7 |
| WAL | 那一份 vault 一次导入 55.0–55.6 GiB，上限处推算约 41 GiB（B-I1）；30 万页从空建 166.6–230.5 GiB；`max_wal_size` 16 GB 只少约 3%（M8，B-N4） | 10.3 第 7 行 | 7 |
| `reindex` | 分词的版本变了、全部的行重新排队时 26 分钟、约 290 GB、之后约 1.7 倍（B-M3） | M8 的数（旧 README `:534`） | 5、7 |
| 版本的增长 | 约 12.5 KB 一个编辑会话；0.9、6.9、37 GB/年；导入一页约 5.4 KB | 10.3 第 9 行（推算） | 7 |
| 时长 | 那一份导入 224–241 秒（7763 约 9 分钟），上限处推算约 3 分钟、7 分钟；队列清空 212–224 秒；导出 4.0 秒；从空建 21–29 分钟 | 10.3 第 10 行、B1 | 7 |
| 10 万页的树 | 约 27 MiB；1、4、8 个标签页 0.3、1.0、2.1 核、堆 0.2、0.8、1.4 GiB、出口 27、106、212 MiB/s；16 个约 2.9 GiB；gzip 1 级约十分之一 | 10.3 第 11 行 | 3、7 |
| 迁移的停机 | 不量（没有 v0.1.0 之前的实例） | 10.3 第 12 行；负责人 2026-11-17 第 8 问 | 5 |
| 已知限制 | 下一节 | 10.3 的清单 | 8；`CHANGELOG.md` |

## 已知限制的去处

"性能"指 v0.2 的 `M12-performance.md`；最后一列是收尾时写的那一份移交（00 号文档 4.13）。

P4 10.3 的 17 行：

| # | 项 | 写在哪里 | 移交 |
|---|---|---|---|
| 1 | 搜索的尾巴 | `CHANGELOG.md`"大的笔记本" | 性能 |
| 2 | 搜索的体积与建索引的 WAL | deploy/README 7 | 性能 |
| 3 | 按页的日志 | `CHANGELOG.md`"大的笔记本" | 性能 |
| 4 | B2 的内存；同时到 | deploy/README 1；`CHANGELOG.md`"内存"；根 README `:141`、`config.yaml:95` | 性能 |
| 5 | 最低配置离 15 秒近 | deploy/README 1、8；`CHANGELOG.md` | 性能 |
| 6 | 导入的节点上限 45,000 | deploy/README 7；`CHANGELOG.md` | —（S6 已定） |
| 7 | 6 万个节点的最上层过期限 | S7 修了；数进 deploy/README 1 | — |
| 8 | 病态的树写 | deploy/README 8；`CHANGELOG.md`"很大的操作" | 性能 |
| 9 | 一页 100 万行历史 | deploy/README 8；`CHANGELOG.md` | 性能 |
| 10 | 30 万页的升级 | deploy/README 8；`CHANGELOG.md` | 性能 |
| 11 | `batch` 没有单独的期限 | deploy/README 1、8；`CHANGELOG.md` | 性能 |
| 12 | 10 万页被很多标签页重读 | deploy/README 7、8；`CHANGELOG.md` | 性能（分层加载） |
| 13 | 阅读视图的大图 | `CHANGELOG.md`"大图" | 性能 |
| 14、15 | 没量到的；设施的问题 | 不写给用户 | 性能 |
| 16 | 不量的 | deploy/README 3、6 | 性能 |
| 17 | P4/S7 记下的 | 下表 | 性能 |

S7 设计第 9 节：删除随子树的历史涨（deploy/README 8 第 1 条、`CHANGELOG.md`"很大的操作"）；移动、改名改写页约 13 / 16 秒（deploy/README 8 第 2 条、`CHANGELOG.md`"最低配置上离 15 秒的期限近"）；病态形状（8 第 3 条、`CHANGELOG.md`）；`/dev/shm`（8 第 4 条，决定 691）；`LinksReachedExcept`、宽读的花费、通用计划下平方、语义上收窄、F 的两小项、缩小规模的守卫不写给用户。移交都是"性能"。

00 号文档 4.13 的来源对 `CHANGELOG.md`（第 3 节第 10 条）：

| 来源 | `CHANGELOG.md` | 移交 |
|---|---|---|
| P2 推后的 C 类 | "网页的打磨" | `M12-polish.md` |
| P2 4.4.8（C-M2） | "阅读视图的就地更新" | `M12-polish.md` |
| H1 | "撤销的次序比较" | `M12-history.md` |
| 24.1 | "网页的任务列表" | `M12-transfer.md` |
| Shift-JIS、Big5、EUC-KR、CP437、决定 376 的两条；GB18030（C-M1） | "导入的名字的编码" | `M12-transfer.md` |
| U+2800；事件流的 4 条、64 条（C-M1、C2-M1） | "两条边界" | U+2800 `M12-polish.md`，事件流 `M12-release.md`（决定 696） |
| 决定 514 的浏览器一侧（C-M3） | "大的笔记本" | `M12-performance.md` |
| H4 的变通与"可见"（C-M5） | "不能立即永久删除" | `M12-history.md` |
| 切罗基文的标题键与标题 id | "从 v0.1.0 之前的试用库升上来" | `M12-release.md` |
| `batch` 的期限；P4 只量的与改判的 | 各条 | `M12-performance.md` |
| 单实例、配额、任务行 | "运维" | `M12-release.md`、`M12-transfer.md` |
| 续期途中刷新被登出；第三方许可证里没有文本的包 | 各条 | `M12-release.md` |
| 人工验收在发布之后 | 那一节 | v0.1.1 的 `M12-after-release.md` |

没有去处的：没有（修复第 1 轮之前漏的 C-M1–M5 与核对第 1 轮的 C2-M1 都补了）。

## 部署项逐条（移交的落实）

| 移交 | 条 | deploy/README |
|---|---|---|
| M0-P6-image-release | 3 的校验说明 | 2"拉镜像并校验"；`SECURITY.md` |
| M4-performance | 2、3 内存；10；14；17 WAL；18；补记 `:490`、`:492` | 1；4 先 `ANALYZE`、1 与 6 连接池代理；没有定为部署要求，余量写在 1、8；7 与 5；3 推荐 nginx；1 |
| M5-performance | 1、2；5 | 1"单实例"（时钟一条不适用）；5 大版本 |
| M7-transfer | 1；3；4；5、6 的部署项 | 8；6 与 3；6 校时；3、1、2、4、6 |
| M8-history | 2；4 | 7"版本的增长"；8 与变通 |

## 门禁与 CI

每个提交在自己检出的状态上，只跑相关的（负责人 2026-11-19 第 12 问），都经 `heavy.sh`：改 `Makefile`、`compose.yaml`、`config.yaml`、守卫的跑 `make lint`、`make test-go-short`（53–55 秒）、`make deploy-check`；第 1 个另在普通的克隆里跑 `make image-smoke`（`.dockerignore` 改了；克隆里先放了 `deploy/.env` 等四样，`modified=false`）；只改文档的跑 `check_links.py`、`git diff --check`、根下 Markdown 的 `oxfmt --check` 与一个查相对链接的小脚本（`check_links.py` 只看 `docs/`），`CHANGELOG.md` 改了的另跑 `release-notes.sh v0.1.0-rc.1`（79 → 86 行，0.1.0 一节完整）。都绿。

分支 CI，每次推送第一次 attempt：

| 头 | `ci.yml` | `release.yml` 的两个演练 |
|---|---|---|
| `dd0d6804` | run 44913031024，19 项 success；image 1:11（Deploy check 4 秒、Image smoke 57 秒）；costs-1 的 Lint workflows 读了改过的 `ci.yml`，3 秒；e2e 7:48 | run 44913031016：amd64 1:40、arm64 1:28 |
| `d1802a0b` | run 44918606904，19 项 success；image 1:15（Deploy check 3 秒）；e2e 7:45 | run 44918606889：1:33、1:25 |
| `22d6a822` | run 44921699001，19 项 success；image 1:17（Deploy check 5 秒）；e2e 7:53 | run 44921699047：1:36、1:43 |
| 本提交 | `（待填：本提交的分支 CI）` | |

tag 的七个作业在分支上都跳过；S3 不改 Go 的产品代码，组合根的分份不受影响。

## 实施中定的

报告里写作"实施中定"与"与设计不同之处"的，主持者在文档提交时各给一号，进上面"主持者的决定"的 697–706；`deploy-check` 用 `jq` 写在 `Makefile` 并进 693，"一起调长"的例子并进 694。不编号的（自己加的，不改设计）：根 README 另两处（`:173`、`:383`）改指 deploy/README；`config.yaml:95` 的注释；d3–d8、p1b、p2b、a3b–a5b 与对照的变异；第 1 遍的可信代理的对照与 `--wait` 的探针；根下 Markdown 的链接检查与以真的 `CHANGELOG.md` 跑 `release-notes.sh`；image-smoke 的克隆里先放 `deploy/.env` 等四样。

## 秘密

日志、记录与提交里没有密码、私钥与令牌：数据库密码只在 `.env`，账户的密码与令牌只在变量里，日志只写键名、变量名与令牌的长度（第 4 遍另核过：账户的密码、`.env` 的两个值、私钥都不在日志里）。浏览器那一步的密码是演练里生成的、只在那个演练的库里，演练结束连库一起删了。

## 更正

- 实施报告 13.1 的"守卫的状态下更慢"把 P4 的意思读反了（P5S3B-I2）；报告第 14 节把同机的一遍叫"第 6 遍"（脚本的编号），设计 10.3 与本记录叫第 4 遍。
- 实施报告第 9 节第 5 条把 zip 小写成推断；B-N8 之后不是推断。
