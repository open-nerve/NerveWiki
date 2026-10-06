# Nerve Wiki 开发命令入口。运行 `make` 或 `make help` 查看所有命令。
# 需兼容 macOS 自带的 GNU Make 3.81。
# 命令按工具链分区：*-go 只需要 Go，*-web 需要 Node（先执行 pnpm install）；
# 不带后缀的 gen、gen-check、lint、fmt 依次执行两个分区。推送前在本地跑 check 与 gen-check，两者合起来是
# 持续集成 server、web 任务的门禁；gen-check 要求生成物已提交，所以不并进 check，在提交之后运行。
# 端到端测试 e2e 另跑（需要 Docker 与 Chromium），对应持续集成的 e2e 任务。

SHELL := /bin/bash
.DEFAULT_GOAL := help

DEV_COMPOSE := docker compose -f deploy/compose.dev.yaml
# 写进 bin/nervewiki 的版本号。默认值与 server/internal/platform/buildinfo 中的相同；发布时指定，例如 make build VERSION=0.1.0
VERSION ?= 0.1.0-dev
GO_LDFLAGS := -X github.com/open-nerve/NerveWiki/server/internal/platform/buildinfo.version=$(VERSION)
GOLANGCI_LINT_VERSION := 2.14.0
BIN_DIR := $(CURDIR)/bin
GOLANGCI_LINT := $(BIN_DIR)/golangci-lint

# 代码生成（docs/v0.1/M0-foundation/04-P4-api-contract.md 3.3）。生成器都在 server/tools 模块里
OAPI_CODEGEN := go tool -modfile=tools/go.mod oapi-codegen
# sqlc 一律不用 cgo 运行：它改用编译成 wasm 的 libpg_query，不需要 C 编译器
SQLC := CGO_ENABLED=0 go tool -modfile=tools/go.mod sqlc
# go -C 切到 server/tools，所以参数都用绝对路径
BODYSHAPEGEN := go -C server/tools run ./bodyshapegen
# 每个模块一个描述文件 api/modules/<模块>.yaml，生成到该模块的 adapter/http/gen
# events 不生成代码：它唯一的操作（事件流）一直开着答复，处理器手写（M5/P2 文档 3.9）
API_MODULES := $(filter-out events,$(basename $(notdir $(wildcard api/modules/*.yaml))))
# 在读 Makefile 时展开：每个模块的 http/gen 目录里先有 oapi-codegen.yaml，所以新模块的目录也在其中。
# postgres/gen 没有这样的文件：模块第一次有 sqlc 查询时，这个目录要到 gen-go 运行后才出现，不在检查之列；
# 那时没提交的生成代码会让持续集成的编译失败，由编译兜底。git 的 pathspec 不展开 *，这里必须是展开后的路径
GEN_GO_OUT := server/internal/platform/httpserver/apigen $(wildcard server/internal/modules/*/adapter/http/gen) \
	$(wildcard server/internal/modules/*/adapter/postgres/gen)
GEN_WEB_OUT := api/dist web/packages/api-client/src/schema.gen.ts
# 生成物必须已提交且没有差异（未跟踪的新文件也算）；$(1) 是生成物的路径
check-committed = test -z "$$(git status --porcelain -- $(1))" || { git status --short -- $(1); git --no-pager diff -- $(1); echo "生成物与接口描述不一致：执行 make gen，并提交生成的文件"; exit 1; }

.PHONY: help
help: ## 列出所有命令
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## / {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: dev-db
dev-db: ## 启动开发数据库（PostgreSQL 18），等待就绪
	$(DEV_COMPOSE) up -d --wait db

.PHONY: dev-db-down
dev-db-down: ## 停止开发数据库，保留数据
	$(DEV_COMPOSE) down

.PHONY: dev-db-reset
dev-db-reset: ## 停止开发数据库并删除数据卷
	$(DEV_COMPOSE) down -v

.PHONY: run
run: ## 以 dev 配置启动服务（先执行 make dev-db）；Ctrl-C 优雅停止
	cd server && NWIKI_ENV=dev go run ./cmd/nervewiki serve

.PHONY: web-dev
web-dev: ## 前端开发服务器 127.0.0.1:5173（热更新），接口代理到 make run 的后端
	pnpm --filter @nervewiki/web dev

# 两个进程都在前台，Ctrl-C 同时发给它们
.PHONY: dev
dev: dev-db ## 一条命令起开发环境：开发数据库、后端（make run）、前端热更新（make web-dev）
	$(MAKE) -j2 run web-dev

.PHONY: build-web
build-web: ## 构建前端，产物在 web/apps/web/dist（需要 Node）
	pnpm --filter @nervewiki/web build

# webui/dist 里只提交 .gitkeep：先清掉上一次复制进去的前端，再复制这一次的。
# 构建参数与 deploy/Dockerfile 的相同（静态链接、-trimpath），改一处要同步另一处：e2e 测的就是要发布的那种构建
.PHONY: build
build: build-web ## 构建 bin/nervewiki，前端内嵌在其中，版本号取 VERSION（需要 Go 与 Node）
	find server/internal/platform/webui/dist -mindepth 1 ! -name .gitkeep -delete
	cp -R web/apps/web/dist/. server/internal/platform/webui/dist/
	cd server && CGO_ENABLED=0 go build -trimpath -ldflags "$(GO_LDFLAGS)" -o ../bin/nervewiki ./cmd/nervewiki

# 故事读 NWIKI_E2E_VERSION，核对 make build 注入的版本号（docs/v0.1/M0-foundation/06-P6-e2e-delivery.md 3.4）
.PHONY: e2e
e2e: build ## 构建 bin/nervewiki，运行端到端故事（需要 Docker 与 Playwright 的 Chromium，见 README）
	cd e2e && NWIKI_E2E_VERSION=$(VERSION) pnpm exec playwright test

# 镜像的标签跟着 VERSION；构建上下文是仓库根目录，见 deploy/Dockerfile 与 .dockerignore。
# 提交信息取自带进构建上下文的 .git：git worktree 的 .git 是指向别处的文件，在那里构建会失败，所以先检查
IMAGE ?= nervewiki:$(VERSION)

.PHONY: image
image: ## 构建镜像 $(IMAGE)，版本号取 VERSION（只需要 Docker）
	@test -d .git || { echo "make image 要在普通的克隆中运行：这里的 .git 不是目录（git worktree），镜像构建取不到提交信息"; exit 1; }
	docker build -f deploy/Dockerfile --build-arg VERSION=$(VERSION) -t $(IMAGE) .

.PHONY: image-smoke
image-smoke: image ## 在镜像上跑 S1、S3：迁移、探针、前端、实例与提交信息、注册关闭、管理员建账户、非 root、优雅停机（需要 Docker、curl、jq、openssl）
	deploy/image-smoke.sh $(IMAGE) $(VERSION)

.PHONY: tools
tools: ## 安装锁定版本的 golangci-lint 到 ./bin
	@set -o pipefail; \
	if $(GOLANGCI_LINT) --version 2>/dev/null | grep -q "version $(GOLANGCI_LINT_VERSION) "; then \
		echo "golangci-lint $(GOLANGCI_LINT_VERSION) 已安装"; \
	else \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/v$(GOLANGCI_LINT_VERSION)/install.sh | sh -s -- -b $(BIN_DIR) v$(GOLANGCI_LINT_VERSION); \
	fi

.PHONY: gen
gen: gen-go gen-web ## 重新生成全部代码：Go 接口层、api/dist、TS 客户端

.PHONY: gen-go
gen-go: ## 由 api/ 生成 Go 接口层和请求体结构表，由 server/sqlc.yaml 生成查询代码（只需要 Go）
	rm -f server/internal/platform/httpserver/apigen/*.gen.go server/internal/modules/*/adapter/http/gen/*.gen.go
	rm -rf server/internal/modules/*/adapter/postgres/gen
	cd server && $(SQLC) generate
	cd server && $(OAPI_CODEGEN) -config internal/platform/httpserver/apigen/oapi-codegen.yaml ../api/common.yaml
	@set -e; for m in $(API_MODULES); do \
		gen=$(CURDIR)/server/internal/modules/$$m/adapter/http/gen; \
		echo "cd server && $(OAPI_CODEGEN) -config internal/modules/$$m/adapter/http/gen/oapi-codegen.yaml ../api/modules/$$m.yaml"; \
		(cd server && $(OAPI_CODEGEN) -config internal/modules/$$m/adapter/http/gen/oapi-codegen.yaml ../api/modules/$$m.yaml); \
		echo "$(BODYSHAPEGEN) -config $$gen/oapi-codegen.yaml -out $$gen/bodyshape.gen.go $(CURDIR)/api/modules/$$m.yaml"; \
		$(BODYSHAPEGEN) -config $$gen/oapi-codegen.yaml -out $$gen/bodyshape.gen.go $(CURDIR)/api/modules/$$m.yaml; \
	done

.PHONY: gen-web
gen-web: ## 打包 api/dist/openapi.yaml，生成 TS 客户端的类型（需要 Node）
	REDOCLY_SUPPRESS_UPDATE_NOTICE=true pnpm run bundle:api
	pnpm --filter @nervewiki/api-client gen

.PHONY: gen-check
gen-check: gen-check-go gen-check-web ## 重新生成全部代码，检查生成物已提交且没有差异

.PHONY: gen-check-go
gen-check-go: gen-go ## 重新生成 Go 接口层并检查（持续集成 server 任务）
	@$(call check-committed,$(GEN_GO_OUT))

.PHONY: gen-check-web
gen-check-web: gen-web ## 重新生成 api/dist 和 TS 类型并检查（持续集成 web 任务）
	@$(call check-committed,$(GEN_WEB_OUT))

.PHONY: check
check: lint knip test build-web ## 静态检查、未使用代码检查、测试、前端构建（生成物一致性另跑 gen-check）

.PHONY: lint
lint: lint-go lint-web ## 全部静态检查

.PHONY: fmt
fmt: tools ## 修正全部格式：Go（gofmt、goimports）与其余文件（oxfmt，需要 Node）
	cd server && $(GOLANGCI_LINT) fmt ./...
	pnpm run fix:format

# config verify 按 schema 校验 .golangci.yml：run 会静默忽略拼错的键。go mod tidy -diff 在 go.mod 或 go.sum 不整洁时失败。
# server/tools 是嵌套的独立模块，server 的 ./... 不进入它，另跑一次
.PHONY: lint-go
lint-go: tools ## 校验 golangci-lint 配置并运行（含格式检查，server 与 server/tools）；检查 go.mod 整洁
	cd server && $(GOLANGCI_LINT) config verify && $(GOLANGCI_LINT) run ./...
	cd server/tools && $(GOLANGCI_LINT) run --config ../.golangci.yml ./...
	cd server && go mod tidy -diff
	cd server/tools && go mod tidy -diff

.PHONY: lint-web
lint-web: ## Markdown 样例集自检；oxlint（零警告）；格式检查；各包的类型检查（需要 Node）
	node tools/md-fixtures/check.mjs
	pnpm run check:lint
	pnpm run check:format
	pnpm -r run check:types

# 门禁：有未使用的文件、导出、依赖，或配置本身过时（例如不再需要的忽略项），都会失败
.PHONY: knip
knip: ## 检查未使用的文件、导出和依赖（需要 Node）
	pnpm exec knip --treat-config-hints-as-errors

# -race 需要 cgo：macOS 需要 Xcode 命令行工具，Linux 需要 gcc。
# 集成测试用 testcontainers 启动 PostgreSQL，需要 Docker；只跑单元测试用 cd server && go test -short ./...
# 不用测试缓存：它不跟踪 server/ 之外的文件，契约测试读取的 api/dist/openapi.yaml 改了也会重放旧结果。
# server/tools 是嵌套的独立模块，另跑一次
.PHONY: test
test: test-go test-web ## 全部测试

.PHONY: test-go
test-go: ## 运行 Go 测试，含集成测试与 server/tools（开启竞态检测，不用测试缓存；需要 Docker）
	cd server && go test -race -count=1 ./...
	@# 竞态检测让同一段代码的分配多出数倍、耗时慢数倍：分配与耗时的预算在不带它的构建中另测一次
	cd server && go test -count=1 -run '^TestCheckCostsAboutTheBody$$' ./internal/platform/httpserver/bodyshape
	cd server && go test -count=1 -run '^TestTheCostsAreAboutTheSize$$' ./internal/platform/markdown
	cd server && go test -count=1 -run '^TestTheAppsMarkdownCostsAboutItsSize$$' ./internal/bootstrap
	cd server && go test -count=1 -run '^(TestALinkWrittenManyTimesResolvesOnce|TestDistinctPathsToPagesOfOneTitleResolveInTimeAsLongAsThey)$$' ./internal/modules/linking/app
	cd server && go test -count=1 -run '^TestTheLinktextsOfManyPagesOfOneTitleAreCheap$$' ./internal/modules/linking/domain
	go -C server/tools test -race -count=1 ./...

.PHONY: test-web
test-web: ## 运行前端各包的测试（vitest；需要 Node）
	pnpm -r run test
