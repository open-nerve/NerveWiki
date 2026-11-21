# Nerve Wiki 的部署与运维

本文写怎样在一台机器上装、升级、备份与恢复 Nerve Wiki，以及要给它多少机器。镜像是 `ghcr.io/open-nerve/nervewiki:<版本>`，`linux/amd64` 与 `linux/arm64` 两种架构，只有确切的版本号（例如 `0.1.0`），没有 `latest`。配置项的含义与开发环境见仓库根的 [README](../README.md)；每个版本改了什么见 仓库根的 `CHANGELOG.md`。

下面的命令假定 compose 的文件放在 `/srv/nervewiki`，对外的地址是 `https://wiki.example.com`，照你的改。

## 1. 要求

### 硬件

v0.1 的数在我们的测试机上量（P4 的实测；本机比最低配置快约 2.0–2.3 倍），推到最低配置的写"推断"。

|                   | 默认配置                                                                                             | 小机器                                                                                             |
| ----------------- | ---------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------- |
| 设置              | 不改                                                                                                 | `page.parse_budget_bytes` 5 MiB（`NWIKI_PAGE__PARSE_BUDGET_BYTES=5242880`）加 `GOMEMLIMIT=1536MiB` |
| 服务的内存        | 空闲约 40–54 MiB，20–50 个人同时编辑约 120–140 MiB；最坏约 2.6 GiB 堆（见下），**按约 3 GiB 留内存** | 最坏约 1.5 GiB 堆、进程至多约 1.65 GiB，按约 1.7 GiB 留内存，容器约 2 GiB                          |
| PostgreSQL 的内存 | 自己的进程 0.1–0.3 GiB；页缓存越多越好，量到的容器 4–8 GB 里多半是页缓存                             | 量到的容器约 3.3 GB，多半是页缓存                                                                  |
| CPU               | 最低 4 核，EPYC 7763 一类（见下面"最低配置的余量"）                                                  | 同左                                                                                               |
| 磁盘              | 见"7. 容量"                                                                                          | 同左                                                                                               |

- **最坏的内存**：页面正文的解析预算（`page.parse_budget_bytes`，默认 8 MiB）满着、全是病态正文时，服务的堆至多约 2.6 GiB（2,662 MiB；最坏实测 2,592 MiB），进程至多约 2.65 GiB（实测 2,718 MiB）。"约 3 GiB"只覆盖这一种。几样同时到时：
  - 再加两个上限处的大请求体（`/mcp` 或正文的大的写，整个服务同时至多两个），堆约 3.3–3.5 GiB、进程约 3.5–3.8 GiB（推算）：容器约 4–4.5 GiB；
  - 再加很多标签页重读 10 万页的树（8 个约 1.4 GiB），容器约 5.5 GiB 起（推算）。
- **`GOMEMLIMIT`**：设成容器上限减约 0.5 GiB，不低于 3 GiB（`3072MiB`；小机器的 `1536MiB` 是量过的那一档）。它让 GC 在接近上限时先收；存活的内存超过它时服务不会因它失败，只是 GC 更忙，内存照样涨到存活的大小，所以同时到的情形靠容器的大小。
- **最低配置的余量**：最低配置是 EPYC 7763 的 4 核一类（GitHub 的 runner 用的那种）。请求的期限是 15 秒（`server.request_timeout`），过了就答错误、什么也不写。在这种机器上：
  - 回收站与撤销在它们的上限处（恢复 1 万个节点、40 万行历史；撤销一次这样的删除）的实测最大值加 2 秒的等待是 13.79、14.54 秒：**一般的 7763 只余约 0.5 秒**，按慢 25% 的实例推断 16.5–16.9 秒，**慢的实例可能超时**；刚删了马上恢复、自动 vacuum 还没跑的库上，这两种操作还要慢一些。撤销 1,500 个最上层的项 4.82 秒。
  - 约 6 万个节点的最上层（导入上限的整个 vault）的删除、移动、改名推断 10.0、6.8、6.0 秒（慢 25%：12.5、8.5、7.5 秒）。
  - 10 万页的笔记本升级为 LLM Wiki：发起推断 6.3 秒、批准 7.3 秒（慢 25%：7.9、9.2 秒）。
  - 一次 `batch` 在链接的上限处，单元 8.2–8.7 秒；`batch` 没有单独的期限，最大合法的在慢的机器上可能超时、整个回滚（"8. 已知限制"）。
  - 比 7763 慢的 CPU 不推荐；要用时，大的删除、恢复与撤销分小一些做。

### PostgreSQL

- **18.7 或更新的 18**：18.7 修了 GIN 索引的一个问题，正对着搜索的 `page_search`，还有一批安全修复。`deploy/compose.yaml` 用 `postgres:18.7-trixie`。
- **以 builtin provider 的 `C.UTF-8` 建库**，否则服务拒绝启动（[总体设计](../docs/v0.1/v0.1-design.md) 7.1）：`CREATE DATABASE nervewiki TEMPLATE template0 ENCODING 'UTF8' LOCALE_PROVIDER builtin LOCALE 'C.UTF-8'`；或者整个集群照 compose 样例以 `--locale-provider=builtin --locale=C.UTF-8` initdb。
- **带 LZ4 构建**：页面正文用 LZ4 压缩，迁移 00054 在不带 LZ4 的数据库上失败。官方镜像带。
- 托管的 PostgreSQL 要能照上面三条建库，并允许库的所有者建 `pg_trgm`（PostgreSQL 13 起它是 trusted 的扩展，库的所有者能建）。
- **连接数**：每个实例 `database.max_conns`（默认 10）加 2：后台任务与事件流各借走一个连接专门监听通知。
- **连接池代理**（PgBouncer 等）只能用会话模式：事件流的 `LISTEN`、连接建立时的 `SET jit = off` 都是会话级的，有几条语句的描述与执行分两次往返。

### 单实例

v0.1 只支持一个 `serve` 进程：

- 附件在本地的目录里（`/data`），一个目录只给一个进程用，启动时它删掉上次中途退出留下的半截文件；
- 限流、大请求体的名额、正在上传的导入只在进程里计数；
- 启动时的收拾（把上一个进程留下的运行中的任务记成失败）假定没有别的实例。

所以升级时先停旧的再起新的（Kubernetes 写 `strategy: Recreate`）。多实例之间的时钟要不要同步的问题因此也不适用。

## 2. 安装

### 拉镜像并校验

```bash
docker pull ghcr.io/open-nerve/nervewiki:0.1.0
```

每个版本的 Release 的说明里有校验的命令（来源证明与签名），照它执行：`gh` 要先 `gh auth login`，cosign 用 v3。0.1.0 的写法：

```bash
gh attestation verify oci://ghcr.io/open-nerve/nervewiki:0.1.0 -R open-nerve/NerveWiki \
  --cert-identity https://github.com/open-nerve/NerveWiki/.github/workflows/release.yml@refs/tags/v0.1.0
cosign verify ghcr.io/open-nerve/nervewiki:0.1.0 \
  --certificate-identity=https://github.com/open-nerve/NerveWiki/.github/workflows/release.yml@refs/tags/v0.1.0 \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
```

第一条加 `--predicate-type https://spdx.dev/Document/v2.3` 是两份 SBOM（镜像的与前端的）的证明。Release 的附件另有两份 SBOM 与镜像里的第三方许可证（`/licenses`）。

### 用 `deploy/compose.yaml`

[`compose.yaml`](compose.yaml) 起 PostgreSQL 与服务，迁移另以 `docker compose run --rm migrate` 执行。服务只听 `127.0.0.1:8080`，前面要有反向代理（"3. 反向代理与 HTTPS"）。

1. 放文件。取与镜像同一个版本的两个文件：

   ```bash
   mkdir -p /srv/nervewiki && cd /srv/nervewiki
   curl -fsSLO https://raw.githubusercontent.com/open-nerve/NerveWiki/v0.1.0/deploy/compose.yaml
   curl -fsSLO https://raw.githubusercontent.com/open-nerve/NerveWiki/v0.1.0/deploy/compose-initdb.sh
   chmod 755 compose-initdb.sh
   ```

2. `.env`：版本号、两个数据库密码与对外的地址。密码只用十六进制（它放进连接地址）：

   ```bash
   cat > .env <<EOF
   NERVEWIKI_VERSION=0.1.0
   NWIKI_DB_ADMIN_PASSWORD=$(openssl rand -hex 24)
   NWIKI_DB_PASSWORD=$(openssl rand -hex 24)
   NWIKI_PUBLIC_URL=https://wiki.example.com
   EOF
   chmod 600 .env
   ```

   `NWIKI_DB_ADMIN_PASSWORD` 是超级用户 `postgres` 的，只在数据库的容器里用；`NWIKI_DB_PASSWORD` 是服务与迁移登录的 `nervewiki` 的。两个密码只在第一次建库时写进库：之后要改，在库里 `ALTER ROLE`，再改 `.env`。另可写 `NERVEWIKI_PORT`（默认 8080）。

3. 签名私钥。服务以 uid 65532 读它，所以文件是 644，所在的目录只给部署的用户：

   ```bash
   mkdir -m 700 secrets
   openssl genpkey -algorithm ed25519 -out secrets/jwt.pem
   chmod 644 secrets/jwt.pem
   ```

   私钥丢了，所有人要重新登录、开着的下载地址失效；与 `.env` 一起另外备份。

4. 附件目录：

   ```bash
   mkdir data && sudo chown 65532:65532 data
   ```

   没有 sudo 时，借已有的镜像当 root（下文凡是 `sudo` 的都可以这样做）：

   ```bash
   docker run --rm -v "$PWD":/w --entrypoint sh postgres:18.7-trixie -c 'chown 65532:65532 /w/data'
   ```

5. 子网。compose 的网络钉在 `10.213.47.0/24`（下文"为什么设 `trusted_proxies`"）。第一次 `up` 之前看一眼 `ip route`：有与它重叠的网段（局域网、VPN、别的 Docker 网络）时，把 `compose.yaml` 里 `subnet` 与 `NWIKI_SERVER__TRUSTED_PROXIES` 两处一起改成别的。与别的 Docker 网络撞了时 `up` 报 `Pool overlaps`。

6. 起数据库、迁移、起服务：

   ```bash
   docker compose up -d --wait db
   docker compose run --rm migrate
   docker compose up -d serve
   ```

   第一次起数据库时 `compose-initdb.sh` 建角色 `nervewiki`（不是超级用户）与它所有的库；`migrate` 以它建表，表与 `pg_trgm` 都归它。`migrate` 不随 `up` 启动，见"5. 升级与回退"。

7. 第一个管理员。密码从标准输入读（见根 README 的"管理员命令"）：

   ```bash
   read -rs PASSWORD
   printf '%s\n' "$PASSWORD" | docker compose run --rm -T serve users create --email admin@example.com
   ```

8. 看它起来了，版本号是装的那个：

   ```bash
   curl -fsS http://127.0.0.1:8080/readyz
   curl -fsS http://127.0.0.1:8080/api/v0/instance
   ```

   之后配好反向代理，在浏览器里以管理员登录。注册默认关闭（`auth.signup_enabled`），账户由管理员建。

### 为什么设 `trusted_proxies` 与钉子网

宿主机上的反向代理连 `127.0.0.1:8080`，经 Docker 的端口转发进容器，服务看到的对端是 compose 网络的网关，不是代理自己。不信任它时，服务不采信 `X-Forwarded-For`，所有客户端都算作这一个 IP：按 IP 的限流（登录、注册、认证失败、匿名的读、附件的内容）全实例共用一个桶，一个人连着输错密码，整个团队都登录不了；会话记下的 IP 也都是网关。所以样例钉住子网，`NWIKI_SERVER__TRUSTED_PROXIES` 信任整个子网（子网里只有本项目的容器）。代理也放进 compose 时，可信的是代理容器的地址。

配置不对时服务记 WARN：`ignored X-Forwarded-For from a peer that is not a trusted proxy`。

### 库角色

样例用一个不是超级用户的角色 `nervewiki`，它是库与表的所有者，迁移与服务都用它。要让迁移与服务分用两个角色（服务的角色不能改表结构），照下面"不用 compose"的写法。

### 不用 compose

1. 照"1. 要求"建库。
2. 角色：表的所有者执行迁移，服务以另一个角色登录，`database.auto_migrate` 关闭（prod 默认关闭）。服务需要的权限全部写在 [`runtime-grants.sql`](runtime-grants.sql)，授予组角色 `nervewiki_runtime`：

   ```sql
   CREATE ROLE nervewiki_runtime NOLOGIN;
   CREATE ROLE nervewiki_app LOGIN PASSWORD '…' IN ROLE nervewiki_runtime;
   ```

   - 文件里是业务表逐表的读写（不给 `TRUNCATE`、`REFERENCES`、`TRIGGER` 与 DDL），搜索的两个序列的 `USAGE`，River 的表与序列，`river_job` 的 `MAINTAIN`（River 每天 `REINDEX INDEX CONCURRENTLY` 它的索引，要 PostgreSQL 17 起），导入与后台的恢复、撤销写入的 11 张表与 `page_search` 的 `MAINTAIN`（任务写一批之后 `ANALYZE` 它们；少了它只记一条 WARNING，统计等 autovacuum），`goose_db_version` 的读（`/readyz` 靠它判断迁移是否执行完）。函数与类型靠 PostgreSQL 默认给 PUBLIC 的权限。
   - **每次 `migrate up` 之后、启动服务之前**，以表的所有者执行一次：`psql -v ON_ERROR_STOP=1 -f runtime-grants.sql`。新的迁移可能加了表，文件逐个列出；重复执行没有影响。
   - 少了业务表的权限时接口答 500；少了 `goose_db_version` 的读时 `/readyz` 是 503，后台任务一直等着不启动；少了 `river_queue` 的权限时 River 启动失败，服务随之退出（`start the jobs: … permission denied`）；少了 River 其他表的，`/readyz` 仍是 200，会话清理、软删除清理与索引重建因 `permission denied` 失败，只记在日志里。
   - 索引属于表的所有者：停机打断 River 的索引重建时留下的 `*_ccnew` 索引，服务的角色删不掉，River 此后每天记 WARN `Found reindex artifact`，由表的所有者 `DROP INDEX CONCURRENTLY` 删除。

3. 迁移与服务。每个命令都校验整份配置，所以迁移也带上同样的变量：

   ```bash
   key=(-v "$PWD/jwt.pem:/run/secrets/jwt.pem:ro" -e NWIKI_AUTH__JWT__PRIVATE_KEY_FILE=/run/secrets/jwt.pem)
   docker run --rm -e NWIKI_DATABASE__URL=postgres://owner:…@db:5432/nervewiki "${key[@]}" \
     ghcr.io/open-nerve/nervewiki:0.1.0 migrate up
   psql -v ON_ERROR_STOP=1 -f runtime-grants.sql   # 以表的所有者
   docker run -d --name nervewiki -p 127.0.0.1:8080:8080 -v /srv/nervewiki/data:/data \
     -e NWIKI_DATABASE__URL=postgres://nervewiki_app:…@db:5432/nervewiki \
     -e NWIKI_SERVER__PUBLIC_URL=https://wiki.example.com -e NWIKI_SERVER__TRUSTED_PROXIES=… \
     "${key[@]}" ghcr.io/open-nerve/nervewiki:0.1.0
   ```

### 配置

- 镜像默认 `NWIKI_ENV=prod`：不自动迁移，注册关闭，签名私钥必须给。配置用环境变量（`NWIKI_<节>__<键>`，例如 `NWIKI_DATABASE__MAX_CONNS=20`），也可以挂载一个目录并设 `NWIKI_CONFIG_DIR`；全部配置项见 [`server/configs/config.yaml`](../server/configs/config.yaml)，合并规则见根 README 的"配置"。
- 至少要有：数据库地址 `NWIKI_DATABASE__URL`、签名私钥 `NWIKI_AUTH__JWT__PRIVATE_KEY_FILE`（uid 65532 要能读）、对外的地址 `NWIKI_SERVER__PUBLIC_URL`（只到源；MCP 的 `read` 给附件的下载地址时以它起头）、反向代理的地址 `NWIKI_SERVER__TRUSTED_PROXIES`（逗号分隔的 CIDR）。
- **附件目录**：附件与导入导出的文件在 `/data`（`storage.dir`，镜像以 `NWIKI_STORAGE__DIR=/data` 设定），它是镜像声明的卷。
  - 要挂载宿主机目录（compose 样例的 `./data`，或 `-v /srv/nervewiki/data:/data`）或具名卷。不挂载时 Docker 建一个匿名卷，删掉容器之后就找不回来了。
  - 进程以 uid 65532 运行：宿主机目录的属主要是它（Kubernetes 里设 `securityContext.fsGroup: 65532`）。目录或其中的区（`blobs/` 等）不可写时 `serve` 拒绝启动，错误写明目录与 uid（`storage: cannot write in /data as uid 65532, gid 65532: …`）。
  - 环境变量优先于配置文件：挂载了配置目录也要改目录时，改 `NWIKI_STORAGE__DIR`。

## 3. 反向代理与 HTTPS

服务不终止 TLS，前面要有反向代理。下面两份是整个站点的样例；要点：

- **`X-Forwarded-For`**：设 `server.trusted_proxies`，否则每个客户端都被当成代理（"2. 安装"）。代理写入的必须是不带端口的 IP。
- **事件流**（`GET /api/v0/events`）不能缓冲，读超时要长于心跳间隔（`events.heartbeat_interval`，默认 20 秒）。服务在它的答复上设了 `X-Accel-Buffering: no`。样例里事件流经过的那一部分（nginx 的 `location /` 与 `server` 里共用的几行、Caddy 的最后一个 `handle`）是验证过的写法：帧在写入答复的同一毫秒到达。Caddy 的 `encode` 不压缩 `text/event-stream`，不必排除。nginx 用 HTTP/1.1 连上游；`proxy_read_timeout` 默认 60 秒，长于默认的心跳，调大心跳时一起调大。
- **JSON 开压缩**：大的笔记本的页面树是一次几 MiB 到几十 MiB 的 JSON（10 万页约 27 MiB），服务自己不压缩；gzip 1 级约压到十分之一。样例里 nginx 的 `gzip` 几行与 Caddy 的 `encode` 做这件事。
- **请求体按路由处理**：`/mcp` 与页面正文的两个写（`PUT /api/v0/pages/{page_id}/content`、`POST /api/v0/notebooks/{notebook_id}/pages`）**缓冲**请求体；附件的上传与导入的 zip **不缓冲**；别的照代理的默认。
- 搜索的词在请求行里（`GET /api/v0/workspaces/{slug}/search?q=…`），附件与导出的下载地址带签名，都会进代理的访问日志（服务自己的访问日志只记路由）：访问日志要照敏感数据保管。

nginx：共用的几行写在 `server` 里，每个 `location` 都继承它们。在某个 `location` 里加 `proxy_set_header` 时，它不再继承 `server` 里的三行 `proxy_set_header`，要在那里一起写上（少了 `X-Forwarded-For`，每个客户端都成了代理的 IP）：

```nginx
server {
    server_name wiki.example.com;
    # listen 443 ssl; ssl_certificate …; ssl_certificate_key …; 照你的部署

    proxy_http_version 1.1;
    proxy_set_header Connection "";
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_read_timeout 60s;

    # JSON 的答复压缩（事件流不在 gzip_types 里，不压缩）
    gzip on;
    gzip_proxied any;
    gzip_comp_level 1;
    gzip_types application/json application/problem+json;

    # 页面、接口与事件流；1m 等于 server.max_body_bytes（默认 1 MiB），改了它时一起改
    location / {
        proxy_pass http://127.0.0.1:8080;
        client_max_body_size 1m;
    }

    # MCP：缓冲请求体，上限是 /mcp 的 37,814,272 字节
    location = /mcp {
        proxy_pass http://127.0.0.1:8080;
        proxy_request_buffering on;  # 显式写：server 或 http 一层的 off 会被继承
        client_max_body_size 37m;
    }

    # 页面正文的两个写：缓冲请求体，上限是它们的 31,522,816 字节
    location ~ ^/api/v0/(pages/[^/]+/content|notebooks/[^/]+/pages)$ {
        proxy_pass http://127.0.0.1:8080;
        proxy_request_buffering on;  # 同上
        client_max_body_size 31m;
    }

    # 附件的上传与导入的 zip：不缓冲；上限是导入包的上限（默认 512 MiB）加 64 KiB 的表单
    location ~ ^/api/v0/notebooks/[^/]+/(assets|imports)$ {
        proxy_pass http://127.0.0.1:8080;
        proxy_request_buffering off;
        client_max_body_size 513m;
    }
}
```

Caddy（默认不缓冲请求体，`request_buffers` 只给要缓冲的三个路由；自动的 HTTPS 照 Caddy 的默认）：

```caddyfile
wiki.example.com {
    # JSON 的答复压缩；Caddy 不压缩 text/event-stream
    encode zstd gzip

    # /mcp 与页面正文的两个写：缓冲请求体（37MiB 不小于 /mcp 的 37,814,272 字节）
    @buffered path_regexp ^/(mcp|api/v0/pages/[^/]+/content|api/v0/notebooks/[^/]+/pages)$
    handle @buffered {
        reverse_proxy 127.0.0.1:8080 {
            request_buffers 37MiB
        }
    }

    # 别的路由（页面、接口、事件流、上传与导入）：不缓冲，Caddy 的默认
    handle {
        reverse_proxy 127.0.0.1:8080
    }
}
```

**推荐 nginx**，在大的 MCP 请求体与正文写较多时尤其如此：经 Caddy 时，服务在读请求体之前就答的拒绝（503 `server_busy`、401、429）对几 MiB 以上的请求体常常到客户端成了 502、没有 `Retry-After`（我们在 Caddy 2.10.2 上用在读请求体之前就答它们的替身量过这三种；nginx 照原样转发了 503 与它的 `Retry-After`）：网页的大的保存看到的是一次失败、不会等一等再重发；agent 看不到 `Retry-After`。服务端的改法在 v0.2。

**上传与导入**：附件的上传（`POST /api/v0/notebooks/{notebook_id}/assets`）与导入的 zip（`POST /api/v0/notebooks/{notebook_id}/imports`）走流式的路由，服务按字节放宽它们的读写期限（不低于每秒 64 KiB）。

- 代理不要缓冲这两个路由的请求体，否则整个文件传到代理之后服务才开始读，提前的拒绝（无权、重名、超过上限）也要等传完。别的路由不要关掉缓冲（下面 MCP 的一条）。
- 它们的请求体上限不小于导入包的上限（`transfer.import_max_bytes`，默认 512 MiB）加 64 KiB（请求体是 zip 加表单的几个部分）；附件的上限（`asset.max_bytes`，默认 50 MiB）在它之内。改了 `transfer.import_max_bytes` 时这一行照它改。
- nginx 的 `client_body_timeout`、`proxy_send_timeout`、`proxy_read_timeout` 是两次读写之间的间隔，默认的 60 秒就够；按整个请求计时的代理或负载均衡，要容得下最慢的传输：50 MiB 的附件在每秒 64 KiB 下约 13 分钟，512 MiB 的导入包约 2 小时 17 分钟。
- **几个 GB 的导出的下载**（`exports/` 里的 zip，例如导入上限的 vault 带 5,000 个附件约 0.5 GB）照附件的下载：不要给下载设整个请求的时限，nginx 的 `proxy_max_temp_file_size` 默认 1 GB，更大的答复它边收边传，不必改；按整个请求计时的负载均衡要容得下最慢的客户端。

**MCP**：`/mcp` 是普通的请求与答复，每个 POST 一个 JSON 答复，不是 SSE，也不持住连接。

- 请求体的上限不小于 `/mcp` 的 37,814,272 字节（约 36 MiB）：一次调用至多 5 MiB 的正文加 1 MiB 的别的参数，每个字节按 JSON 最长的转义算 6 个。
- 读超时不短于 45 秒：一次调用的工作至多 15 秒（`server.request_timeout`），之前读请求体至多 30 秒（`server.read_timeout`）；改了这两个的，不短于它们的和。nginx 默认的 60 秒够；Caddy 默认不限。
- `Host` 照常转发：服务关掉了 MCP SDK 对不是回环地址的 `Host` 的拒绝，由 Bearer 令牌把关。
- **缓冲的理由**：大的请求体（MCP 超过 64 KiB 的调用、超过 `server.max_body_bytes` 的正文写）整个服务同时只有两个名额、每个凭证一个，名额从读请求体之前占到答完；代理收完请求体再一次转给服务，慢的发送者就占不住名额。缓冲的请求体在服务认证之前就收在代理那里：nginx 把过了 `client_body_buffer_size` 的写到 `client_body_temp_path` 的磁盘上，Caddy 放在内存里（Caddy 2.10.2 上收着一个 30 MiB 的请求体时代理的内存涨约 66 MB，接近 `/mcp` 上限的约 116 MB），几个慢的发送者不必有有效的令牌就能让代理各占至多约 37 MiB 的磁盘（nginx）或一百多 MB 的内存（Caddy）：给代理留出这些地方。
- 不缓冲时（代理关掉了缓冲，或直接暴露服务）：两个慢的发送者各占一个名额至多约 45 秒，其间别人的大的写等 2 秒后答 503 `server_busy`。这样被拒的每个请求，服务记一行 info 的日志 `large request bodies are saturated`，`holders` 是持着名额的凭证（`pat:<令牌 id>` 或 `session:<会话 id>`）与路由：由 `api_tokens` 或 `auth_sessions` 那一行的 `user_id` 找到账户，请他撤销那个令牌，或以 `nervewiki users reset-password` 撤销这个账户的全部会话与 PAT。

## 4. 备份与恢复

**要备份的**：数据库；附件目录里的 `blobs/`（附件的文件；`exports/`、`imports/` 等是过程中的文件，不必备份）；`.env` 与签名私钥（另存，不与备份放在同一台机器上）。

**次序**：先停 `serve`，**先备份库，再备份 `blobs/`**。附件写入后不变，物理清除要等 60 天，按这个次序得到的备份里，数据库引用的附件都在；恢复时用同一次备份的两份，目录里多出的文件（数据库里没有的）由每天的清扫删掉。

### compose 的备份

```bash
cd /srv/nervewiki
mkdir -p backups
d=$(date +%Y%m%d-%H%M%S)
docker compose stop serve
docker compose exec -T db pg_dump -U postgres -Fc nervewiki > backups/nervewiki-$d.dump
sudo tar -C data -czf backups/blobs-$d.tar.gz blobs
docker compose up -d serve
```

`-T` 不分配终端，二进制的输出才完整。`.env` 与 `secrets/jwt.pem` 另存一份。

### 不用 compose 的备份

- `pg_dump -Fc` 以表的所有者或超级用户导出这个库；分用两个角色时另 `pg_dumpall --roles-only` 存下角色（含 `nervewiki_runtime` 与服务的角色）。
- 停着服务，把附件目录的 `blobs/` 打包。
- 私钥与配置另存。

### compose 的恢复

恢复到一个新的空库：初始化脚本重建角色与空库，以库的所有者 `nervewiki` 恢复，`pg_trgm` 与各表的所有者同新装一样都是它。

```bash
cd /srv/nervewiki
d=<备份的日期>
docker compose stop serve
mv data data.old               # 挪开旧的附件目录（核对过恢复之后再删）
docker compose down -v         # 只删本项目的容器、网络与 db 卷；data/ 是绑定的目录，不受影响
docker compose up -d --wait db
docker compose exec -T db pg_restore -U nervewiki -d nervewiki --exit-on-error < backups/nervewiki-$d.dump
mkdir data && sudo tar -C data -xzf backups/blobs-$d.tar.gz && sudo chown -R 65532:65532 data
docker compose exec -T db psql -U postgres -d nervewiki -c ANALYZE
docker compose up -d serve
curl -fsS http://127.0.0.1:8080/readyz
```

- **先 `ANALYZE` 再开放服务**：恢复的库没有统计，PostgreSQL 给树的读、链接的读选的计划可能每一步扫整本笔记本（一万个子节点的文件夹，读子树 21–40 秒），请求过期限。
- 以 `postgres` 恢复时 `pg_trgm` 归 `postgres`，与新装不同；照上面以 `nervewiki` 恢复。
- 没有 sudo 时，借已有的镜像当 root 放回 `blobs/`，核对过之后删掉挪开的旧目录（属主 65532，`blobs/` 是 750）：

  ```bash
  docker run --rm -v "$PWD":/w --entrypoint sh postgres:18.7-trixie -c \
    "mkdir /w/data && tar -C /w/data -xzf /w/backups/blobs-$d.tar.gz && chown -R 65532:65532 /w/data"
  docker run --rm -v "$PWD":/w --entrypoint sh postgres:18.7-trixie -c 'rm -rf /w/data.old'
  ```

  备份时的 `tar` 同样：`docker run --rm -v "$PWD":/w --entrypoint sh postgres:18.7-trixie -c "tar -C /w/data -czf /w/backups/blobs-$d.tar.gz blobs"`。

### 不用 compose 的恢复

照"1. 要求"建一个 builtin `C.UTF-8` 的空库；两个角色时先恢复角色（`pg_dumpall --roles-only` 的输出）；以表的所有者 `pg_restore --exit-on-error`；两个角色时执行一次 `runtime-grants.sql`；放回 `blobs/`、属主 65532；`ANALYZE`；启动，看 `/readyz`。

### 演练

备份没恢复过就不算有。上线之前、之后每隔一段时间，在另一台机器（或同一台机器上另一个目录与项目名）上照上面恢复一次：

1. 在要演练的目录里放 `compose.yaml`、`compose-initdb.sh`、原来的 `.env` 与 `secrets/jwt.pem`，`NERVEWIKI_PORT` 换一个空闲的端口；同一台机器上另用一个项目名（`docker compose -p nervewiki-drill …`），并把子网改成别的（钉住的子网同一时刻只能有一个项目用）。
2. 照"compose 的恢复"恢复最近的一次备份。
3. 登录，打开几页、几个附件，与原来的比；看 `/api/v0/instance` 的版本号。
4. 删掉：`docker compose -p nervewiki-drill down -v`，删 `data/`（属主 65532，要 sudo 或借镜像）。

## 5. 升级与回退

### 升级

每个版本的说明（仓库根的 `CHANGELOG.md` 的一节）写明它的迁移会不会重写大表、要不要 `nervewiki reindex`、要不要改代理的配置。通用的步骤：

```bash
cd /srv/nervewiki
docker pull ghcr.io/open-nerve/nervewiki:<新版本>      # 照"2. 安装"校验
docker compose stop serve                               # 先停
# 照"4. 备份与恢复"备份库与 blobs/
sed -i 's/^NERVEWIKI_VERSION=.*/NERVEWIKI_VERSION=<新版本>/' .env
docker compose run --rm migrate
# 说明里要 reindex 时：docker compose run --rm serve reindex
docker compose up -d serve
curl -fsS http://127.0.0.1:8080/readyz
curl -fsS http://127.0.0.1:8080/api/v0/instance       # 版本号是新的
```

- **先停旧的服务再迁移**：迁移可能以 `ACCESS EXCLUSIVE` 重写整张表，旧的服务还在跑时每次读写都等它、大的实例上会过请求的期限；也不要让旧、新两个版本同时写。不用 compose 时 `docker stop -t 40`。
- 取下一个版本的 `compose.yaml` 与 `compose-initdb.sh` 时，比一下与你改过的有什么不同。
- 两个角色时，`migrate up` 之后、启动服务之前以表的所有者执行一次 `runtime-grants.sql`。
- **`nervewiki reindex`** 重建链接索引与搜索的索引，只在说明要它时执行（`migrate` 之后、`serve` 之前）。它重建一个已有的索引比从空建贵：30 万页约 26 分钟、约 290 GB 的 WAL（每页约 1 MB），之后 `page_search` 在磁盘上约是原来的 1.7 倍，空出的地方不还给系统。执行之前留出至少 `page_search` 现在那么大的空闲磁盘；有 WAL 归档或复制槽时，归档与 `pg_wal` 要容得下这些 WAL。
- **迁移的停机没有量过**：v0.1.0 的迁移从空库执行，没有在大的库上量过从别的版本升上来要停多久。之后的版本会在说明里写它的迁移的代价。

### 回退

回退靠恢复备份：`NERVEWIKI_VERSION` 改回旧的，照"compose 的恢复"恢复升级之前的那一份。`nervewiki migrate down` 不保证数据（有的迁移回退之后，新版本写下的数据旧版本读不了或丢掉标记），不要用它回退。

### PostgreSQL 的升级

- **小版本**（18.7 → 18.8 等）：换镜像的 tag，`docker compose up -d db`。照 PostgreSQL 的发布说明做：18.7 修了 GIN 索引的一个问题，从更早的 18 升上来、怀疑 `page_search` 的 GIN 索引已经损坏时 `REINDEX` 它；18.7 的说明里另有 TOAST 表的索引要 `REINDEX` 的情形，照说明核对。
- **大版本**（18 → 19）：`pg_upgrade`，或 `pg_dump` 再恢复到新版本上照"1. 要求"建的 builtin `C.UTF-8` 库。官方镜像的数据目录按大版本分（`/var/lib/postgresql/18/docker`），换大版本的镜像不会自动升级数据。之后复核"一个事务里几条 `NOTIFY` 只取一次全局锁"仍成立：一次写的事件靠它（论证见 [M5 总设计](../docs/v0.1/M5-collab-editing/00-M5-design.md) 4.10，依据是 PostgreSQL 13–18 的 `async.c` 的 `PreCommit_Notify`）。

## 6. 运行与监控

- **探针**：存活 `GET /healthz`（不访问任何依赖），就绪 `GET /readyz`（数据库可用、迁移已执行完）。镜像里没有 shell 与 curl，没有写 `HEALTHCHECK`，由编排系统从外面探测。
- **日志**：JSON 一行一条（`log.format`），写到标准错误，`docker compose logs serve` 读得到；访问日志也在其中，只记路由。看 `level` 为 `WARN`、`ERROR` 的行。启动之后不久就停止时，River 常记一条 ERROR `maintenance.PeriodicJobEnqueuer: Error starting transaction`（`context canceled`），退出码仍是 0，不是问题。
- **磁盘与 507**：剩余空间低于 `storage.min_free_bytes`（默认 1 GiB）时不再接受写入，答 507；磁盘或配额已满时同样照常启动，写入答 507，启动时记 WARN `storage is full`。**没有按工作区或笔记本的总量配额**：由管理员监控附件目录与数据库所在的磁盘（数据库的空间另见"7. 容量"）。
- **导入导出的文件**：`/data/exports/`、`/data/imports/` 是过程中的文件：导入的 zip 做完就删；成功的导出保留 `transfer.export_ttl`（默认 24 小时），每人每本笔记本只留最新的一份，到期由后台清掉。
- **后台任务**（River，随 `serve` 运行）：每小时的过期会话清理与软删除清理，每 10 分钟的过期编辑会话清理，每天的附件目录清扫，导入导出的收拾与到期，每 5 秒的搜索的建索引。导出、导入与建索引的任务各占一个连接，合起来至多 `database.max_conns` 的一半（默认的配置下 `max_conns` 至少 6）。
- **停机**：发 SIGTERM，服务停止接收新连接，关闭事件流，等正在处理的请求（最多 `server.shutdown_timeout`，默认 20 秒），关闭接收通知的连接（最多 2 秒），等正在执行的后台任务（最多 `jobs.shutdown_timeout` 10 秒，之后取消、再宽限 1 秒），关闭连接池（最多 5 秒），一共至多 38 秒。宽限期要比它长：`docker stop` 默认只等 10 秒，用 `docker stop -t 40`；compose 样例写了 `stop_grace_period: 40s`。
  - `server.shutdown_timeout` 20 秒短于一次 `/mcp` 调用（读请求体至多 30 秒，加工作至多 15 秒，至多约 45 秒）：停机开始时还在传大请求体的调用到 20 秒时被切断、整个回滚，什么也不留一半。要它们做完，把 `server.shutdown_timeout` 与宽限期一起调长（例如 50 秒与 70 秒）。
  - 停机打断的导入、导出、后台的恢复与撤销记成失败 `interrupted`，已写的部分留着，报告写明，重新开始即可；被不干净地停下的建索引任务在下一次启动时丢掉（WARN `jobs left running by a stopped server discarded`），之后重新排队。
- **`GOMEMLIMIT`** 与容器的内存见"1. 要求"。
- **校时**：服务与客户端的时钟都要对。下载地址的到期按客户端的时钟比较，客户端的时钟偏慢时，链接可能指向刚到期的地址、下载失败；服务的时钟决定会话与令牌的到期。用 NTP 或 systemd-timesyncd。
- **连接池代理**（PgBouncer）只能用会话模式（"1. 要求"）。

## 7. 容量

数在我们的测试机上量（P4 的实测）；"推算"写明依据。

| 项                             | 数                                                                                                                                                                                                                                                 |
| ------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 数据库，每 1,000 页            | 约 35 MB（2 KiB 正文、30 条链接的页，含文件夹、附件的行与搜索的索引；按节点算每 1,000 个约 26 MB）。45,000 页的 vault 导入之后库约 1.6 GB；删除再恢复留下死行，之后约 2.4 GB，等 autovacuum 收回                                                   |
| 搜索的索引                     | `page_search` 是正文的 3.3–4.6 倍（笔记本少时更小），按正文的约 5 倍留磁盘。正文的大小：`SELECT pg_size_pretty(sum(byte_size)) FROM page_contents`                                                                                                 |
| 附件与导出                     | 附件的文件本身在 `blobs/`；导出的 zip 在 `exports/` 里另占地方，成功的保留 24 小时。45,000 页、5,000 个 800 KiB 附件的那本，附件 4.1 GB、导出的 zip 0.47 GB：那些附件是合成的、压得很小，真实的图片与 PDF 压不下多少，按正文加附件的大小留（推断） |
| 版本的增长（推算）             | 每次编辑会话（停顿保存的一段编辑，一页一个版本）约 12.5 KB（10 KiB 正文、20 条链接）。每年约：10 人 × 每天 20 次 0.9 GB；50 人 × 30 次 6.9 GB；200 人 × 40 次 37 GB。导入一页一个版本，约 5.4 KB。v0.1 保留每一个版本                              |
| WAL                            | 一次导入上限的导入（45,000 页、5,000 个附件）约 55 GiB；30 万页从空建搜索的索引 167–231 GiB（`max_wal_size=1GB`）；`nervewiki reindex` 30 万页约 290 GB。有 WAL 归档或复制槽时，归档与 `pg_wal` 要容得下这些，导入与重建时看着磁盘                 |
| 时长                           | 导入上限的导入约 4 分钟（推断 7763 上约 9 分钟），之后搜索的建索引约 3.5–4 分钟才找得全；导出这样一本 4 秒；30 万页从空建搜索 21–29 分钟；`reindex` 30 万页约 26 分钟                                                                              |
| 很多标签页开着 10 万页的笔记本 | 树写之后每个标签页重读一次树（约 27 MiB 的 JSON，未压缩）：1、4、8 个标签页持续重读时服务约 0.3、1.0、2.1 核、堆增量至多 0.2、0.8、1.4 GiB、出口 27、106、212 MiB/s（未压缩；代理开了压缩时线上约十分之一）；16 个推算约 2.9 GiB 堆                |
| 内存                           | 见"1. 要求"                                                                                                                                                                                                                                        |

- **导入的上限**：一次导入建的节点（条目加路径隐含的文件夹）至多 45,000 个，超过的整包失败、什么都不写：更大的 vault 分几次导入（例如按顶层的文件夹拆成几个 zip）。zip 另有字节、条目与解压后大小的上限（`transfer.import_max_*`）。
- 搜索的索引与 `reindex` 留磁盘见"5. 升级与回退"。

## 8. 运维相关的已知限制

- **单实例**（"1. 要求"）。
- **没有总量配额**：只有单个文件与导入包的上限、同时的任务数与磁盘余量（507），由管理员监控磁盘。
- **结束的任务行不清理**：导入、导出、恢复、撤销的任务结束之后，任务的行一直留着（笔记本删除之后才随它清理），列表按游标分页，不影响正确性。
- **没有永久删除**：回收站的一项、单个版本都不能立即永久删除，回收站也不能清空；删除的项过了保留期（`jobs.purge_retention`，默认 60 天）由清理删掉，连同它的版本。变通：粘贴进页面的秘密，先把要留的正文写进一页新的，再删除原来的页（它的全部版本随它进回收站），然后照需要把 `jobs.purge_retention` 调短（至少 1 天）、等下一次清理；其间回收站与版本对能读这本笔记本的人可见。
- **`batch` 没有单独的期限**：它与别的请求一样是 15 秒，最大合法的 `batch` 在最低配置上单元 8.2–8.7 秒，在慢的机器上可能超时、整个回滚（答错误，什么也不写），agent 分小一些再发。
- **最低配置上离 15 秒的期限近**：回收站与撤销在上限处一般的 7763 只余约 0.5 秒，慢的实例可能超时（"1. 要求"）。
- **很大的树写**（推断，最低配置上）：
  - 同一个约 6 万个节点的最上层反复删除、恢复，每次删除多约 0.7–0.9 秒（每次给每个节点添历史），第 5–6 次起过 15 秒；
  - 移动、改名的余量只在不改写任何页时成立；大的子树里有页以库内的绝对路径链进来、要改写这些页时，推断约 13 秒，慢 25% 时约 16 秒；
  - 20 万个节点的最上层的删除约 25 秒，约 11.4 万个节点起过期限；400 页 × 2,000 条链接的一次移动约 40 秒（约 150 页起过期限）；被约 1,300 页以上链着的页的改名过期限。过期限的答错误、什么也不写。
  - 约 20 万个节点以上的最上层，读到几十万个元素时，PostgreSQL 的并行计划可能在 Docker 默认 64 MB 的 `/dev/shm` 上报错（这样的形状本来就过期限）；要调就给数据库的容器设 `shm_size`（compose 样例没有设）。
- **很长的历史**：一页 100 万行历史（版本与条目）的删除推断最低配置上约 19–20 秒，过期限（约 75–80 万行起）；后台恢复这样一页约 27–29 秒、其间挡着这本笔记本的写。
- **很大的笔记本升级为 LLM Wiki**：30 万页的发起推断约 23–25 秒、批准约 19–26 秒，过期限；约 14–16 万页起过它的预算，约 18–20 万页起过 15 秒。
- **很大的笔记本的页面树**：5 万、10 万页的树一次整个读，很多标签页同时开着时 CPU、内存与出口随标签页数线性涨（"7. 容量"）；10 万页在浏览器里打开会很慢。分层加载在 v0.2。

v0.1.0 的全部已知限制见 仓库根的 `CHANGELOG.md`。
