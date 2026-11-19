# Nerve Wiki 的镜像（docs/v0.1/M0-foundation/06-P6-e2e-delivery.md 3.5）。构建上下文是仓库根目录：
#   docker build -f deploy/Dockerfile --build-arg VERSION=0.1.0 \
#     --build-arg REVISION="$(git rev-parse HEAD)" --build-arg CREATED="$(git log -1 --format=%cI)" .   （make image）
# 基础镜像按摘要固定，注释中是对应的标签；升级时同时改两者。

# 前端。先只复制包清单与锁文件安装依赖，源码变了这一层照样复用。
# 只安装 @nervewiki/web 与它依赖的包；e2e 的清单也要复制，锁文件里有它；依赖的补丁（patches/：KaTeX 与 @lezer/markdown 的）在安装时打上
FROM node:24-trixie-slim@sha256:af02d57b57719b9657a45199a79264a9437940cd55b3c7b3fc99615654f124a0 AS web
ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=0
RUN corepack enable
WORKDIR /src
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY patches/ patches/
COPY web/apps/web/package.json web/apps/web/
COPY web/packages/api-client/package.json web/packages/api-client/
COPY e2e/package.json e2e/
RUN pnpm install --frozen-lockfile --filter @nervewiki/web...
# 第三方许可证的 npm 一侧（M12/P5/S2，P5 设计 3.11）：@nervewiki/web 与它用的工作区包的每个生产依赖，加上代码随产物
# 发出的开发依赖（tailwindcss）的许可证文件。在复制源码之前：只随锁文件与脚本变，源码变了这一层照样复用
COPY tools/licenses/npm.mjs tools/licenses/
RUN node tools/licenses/npm.mjs /out/licenses/npm
COPY web/ web/
RUN pnpm --filter @nervewiki/web build

# 服务端。前端复制进 webui/dist 后内嵌进二进制。
# 提交信息取自 Go 工具链嵌入的 VCS 信息，与本地、持续集成的来源相同。复制整个上下文（含 .git）而不只是 server/：
# 否则缺了的文件在 git 看来都是删除，二进制总是报告 modified
FROM golang:1.27.4-trixie@sha256:b54b2639de8c1bfe538ec5ffc45f29b1932235ac5f6d63c059853646677112d9 AS server
# golang 镜像设了 GOTOOLCHAIN=local：生效的是摘要，它若是别的 Go，就悄悄用它构建。这一行在最前面断言版本；
# 它与 tag、两份 go.mod 的 toolchain、README 一致，由 server/internal/platform/buildinfo/toolchain_test.go 核对
RUN test "$(go env GOVERSION)" = go1.27.4 || { echo "golang 镜像是 $(go env GOVERSION)，go.mod 要 go1.27.4：换摘要" >&2; exit 1; }
WORKDIR /src/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY . /src/
COPY --from=web /src/web/apps/web/dist/ internal/platform/webui/dist/
ARG VERSION=0.1.0-dev
# 构建参数与 Makefile 的 build 相同，改一处要同步另一处
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-X github.com/open-nerve/NerveWiki/server/internal/platform/buildinfo.version=${VERSION}" \
    -o /out/nervewiki ./cmd/nervewiki
# 第三方许可证的 Go 一侧（M12/P5/S2，P5 设计 3.11）：链接进二进制的每个模块与 Go 本身；没有许可证文件的模块、
# 与二进制记的模块对不上的清单让构建失败
RUN /src/tools/licenses/go.sh /out/nervewiki /out/licenses/go
# 附件与导入导出的目录（M7/P1 设计 3.5）：distroless 没有 mkdir 与 chown，在这里建好，复制时改属主
RUN mkdir /out/data

# 运行时：只有二进制（配置、迁移、前端都内嵌在其中）、CA 证书与第三方许可证，没有 shell。
# 用不带 nonroot 的变体，由这里的 USER 声明非 root 用户：去掉它，image-smoke.sh 就会失败。
# 写成数字（distroless 的 nonroot，uid 65532）：Kubernetes 的 runAsNonRoot 无法核对用户名。
# 不写 HEALTHCHECK：镜像里没有能发请求的程序，编排系统用 /healthz、/readyz 做探针
FROM gcr.io/distroless/static-debian13:latest@sha256:1a64f46364620465ef9e9a8f5413757dfdf22695e050bbfd7303eeaf17a59383 AS runtime
ARG VERSION=0.1.0-dev
# 提交与它的时间（git log -1 --format=%cI），不是构建的时间：同一个提交重建得到同样的 label。
# make image 与 release.yml 传它们；直接 docker build 不传时是空的，image-smoke.sh 会失败
ARG REVISION
ARG CREATED
LABEL org.opencontainers.image.title="Nerve Wiki" \
      org.opencontainers.image.description="自托管的团队在线笔记：工作区、笔记本与层级的 Markdown 页面，内置 MCP" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.created="${CREATED}" \
      org.opencontainers.image.source="https://github.com/open-nerve/NerveWiki" \
      org.opencontainers.image.licenses="AGPL-3.0-only"
COPY --from=server /out/nervewiki /nervewiki
# 第三方许可证（M12/P5/S2，P5 设计 3.11）：Nerve Wiki 自己的、说明、Go 一侧与 npm 一侧。Release 的附件从推上去的
# 镜像里取同一份；image-smoke.sh 核对
COPY LICENSE /licenses/LICENSE
COPY tools/licenses/README.txt /licenses/README.txt
COPY --from=server /out/licenses/go /licenses/go
COPY --from=web /out/licenses/npm /licenses/npm
# 进程要能写的目录，属主是下面的用户。部署时挂载卷或宿主机目录（属主 65532），否则 Docker 建匿名卷，删容器就丢
COPY --from=server --chown=65532:65532 /out/data /data
ENV NWIKI_STORAGE__DIR=/data
VOLUME /data
USER 65532:65532
# prod 配置不自动迁移：部署时先 migrate up，再 serve。数据库地址由 NWIKI_DATABASE__URL 提供
ENV NWIKI_ENV=prod
EXPOSE 8080
ENTRYPOINT ["/nervewiki"]
CMD ["serve"]
