# Nerve Wiki 的镜像（docs/v0.1/M0-foundation/06-P6-e2e-delivery.md 3.5）。构建上下文是仓库根目录：
#   docker build -f deploy/Dockerfile --build-arg VERSION=0.1.0 .   （make image）
# 基础镜像按摘要固定，注释中是对应的标签；升级时同时改两者。

# 前端。先只复制包清单与锁文件安装依赖，源码变了这一层照样复用。
# 只安装 @nervewiki/web 与它依赖的包；e2e 的清单也要复制，锁文件里有它
FROM node:24-trixie-slim@sha256:8ec5d7557396cfe32d21c3f9c13072355ceab22b584578ca4bb28af31120cffe AS web
ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=0
RUN corepack enable
WORKDIR /src
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY web/apps/web/package.json web/apps/web/
COPY web/packages/api-client/package.json web/packages/api-client/
COPY e2e/package.json e2e/
RUN pnpm install --frozen-lockfile --filter @nervewiki/web...
COPY web/ web/
RUN pnpm --filter @nervewiki/web build

# 服务端。前端复制进 webui/dist 后内嵌进二进制。
# 提交信息取自 Go 工具链嵌入的 VCS 信息，所以带上 .git（.dockerignore 不排除它），与本地、持续集成的来源相同
FROM golang:1.27.1-trixie@sha256:3b77fc618ec235a1ab412de7737f120dd507c57e8d87de4cbb7994fb94275ed5 AS server
WORKDIR /src/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY .git/ /src/.git/
COPY server/ ./
COPY --from=web /src/web/apps/web/dist/ internal/platform/webui/dist/
ARG VERSION=0.1.0-dev
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-X github.com/open-nerve/NerveWiki/server/internal/platform/buildinfo.version=${VERSION}" \
    -o /out/nervewiki ./cmd/nervewiki

# 运行时：只有二进制（配置、迁移、前端都内嵌在其中）与 CA 证书，没有 shell。
# 用不带 nonroot 的变体，由这里的 USER 声明非 root 用户：去掉它，image-smoke.sh 就会失败。
# 不写 HEALTHCHECK：镜像里没有能发请求的程序，编排系统用 /healthz、/readyz 做探针
FROM gcr.io/distroless/static-debian13@sha256:58133991db06659feaabe0f4e97a35cebf15ef4ea08f8a4c6d2ee5f75e4aa6a0
ARG VERSION=0.1.0-dev
LABEL org.opencontainers.image.title="Nerve Wiki" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.source="https://github.com/open-nerve/NerveWiki" \
      org.opencontainers.image.licenses="AGPL-3.0-only"
COPY --from=server /out/nervewiki /nervewiki
USER nonroot:nonroot
# prod 配置不自动迁移：部署时先 migrate up，再 serve。数据库地址由 NWIKI_DATABASE__URL 提供
ENV NWIKI_ENV=prod
EXPOSE 8080
ENTRYPOINT ["/nervewiki"]
CMD ["serve"]
