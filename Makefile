# Nerve Wiki 开发命令入口。运行 `make` 或 `make help` 查看所有命令。
# 需兼容 macOS 自带的 GNU Make 3.81。
# 命令按工具链分区：*-go 只需要 Go，*-web 需要 Node（先执行 pnpm install）；
# 不带后缀的 lint、fmt 依次执行两个分区；check 执行持续集成的全部门禁，推送前在本地跑它。

SHELL := /bin/bash
.DEFAULT_GOAL := help

DEV_COMPOSE := docker compose -f deploy/compose.dev.yaml
GOLANGCI_LINT_VERSION := 2.14.0
BIN_DIR := $(CURDIR)/bin
GOLANGCI_LINT := $(BIN_DIR)/golangci-lint

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

.PHONY: tools
tools: ## 安装锁定版本的 golangci-lint 到 ./bin
	@set -o pipefail; \
	if $(GOLANGCI_LINT) --version 2>/dev/null | grep -q "version $(GOLANGCI_LINT_VERSION) "; then \
		echo "golangci-lint $(GOLANGCI_LINT_VERSION) 已安装"; \
	else \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/v$(GOLANGCI_LINT_VERSION)/install.sh | sh -s -- -b $(BIN_DIR) v$(GOLANGCI_LINT_VERSION); \
	fi

.PHONY: check
check: lint knip test ## 持续集成的全部门禁：静态检查、未使用代码检查、测试

.PHONY: lint
lint: lint-go lint-web ## 全部静态检查

.PHONY: fmt
fmt: tools ## 修正全部格式：Go（gofmt、goimports）与其余文件（oxfmt，需要 Node）
	cd server && $(GOLANGCI_LINT) fmt ./...
	pnpm run fix:format

# config verify 按 schema 校验 .golangci.yml：run 会静默忽略拼错的键。go mod tidy -diff 在 go.mod 或 go.sum 不整洁时失败
.PHONY: lint-go
lint-go: tools ## 校验 golangci-lint 配置并运行（含格式检查）；检查 go.mod 整洁
	cd server && $(GOLANGCI_LINT) config verify && $(GOLANGCI_LINT) run ./...
	cd server && go mod tidy -diff

.PHONY: lint-web
lint-web: ## Markdown 样例集自检；tools/ 下脚本的 oxlint（零警告）；格式检查（需要 Node）
	node tools/md-fixtures/check.mjs
	pnpm run check:lint
	pnpm run check:format

# 门禁：有未使用的文件、导出、依赖，或配置本身过时（例如不再需要的忽略项），都会失败
.PHONY: knip
knip: ## 检查未使用的文件、导出和依赖（需要 Node）
	pnpm exec knip --treat-config-hints-as-errors

# -race 需要 cgo：macOS 需要 Xcode 命令行工具，Linux 需要 gcc。
# 集成测试用 testcontainers 启动 PostgreSQL，需要 Docker；只跑单元测试用 cd server && go test -short ./...
.PHONY: test
test: ## 运行 Go 测试，含集成测试（开启竞态检测，不用测试缓存；需要 Docker）
	cd server && go test -race -count=1 ./...
